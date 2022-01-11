package k8ssandra

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"time"
)

// checkSystemReplication checks for the SystemReplicationAnnotation on kc. If found, the
// JSON value is unmarshalled and returned. If not found, the SystemReplication is computed
// and is stored in the SystemReplicationAnnotation on kc. The value is JSON-encoded.
// Lastly, kc is patched so that the changes are persisted,
func (r *K8ssandraClusterReconciler) checkSystemReplication(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) (*cassandra.SystemReplication, error) {
	if val := annotations.GetAnnotation(kc, api.SystemReplicationAnnotation); val != "" {
		replication := &cassandra.SystemReplication{}
		if err := json.Unmarshal([]byte(val), replication); err == nil {
			return replication, nil
		} else {
			return nil, err
		}
	}

	replication := cassandra.ComputeSystemReplication(kc)
	bytes, err := json.Marshal(replication)

	if err != nil {
		logger.Error(err, "Failed to marshal SystemReplication", "SystemReplication", replication)
		return nil, err
	}

	patch := client.MergeFromWithOptions(kc.DeepCopy())
	if kc.Annotations == nil {
		kc.Annotations = make(map[string]string)
	}
	kc.Annotations[api.SystemReplicationAnnotation] = string(bytes)
	if err = r.Patch(ctx, kc, patch); err != nil {
		logger.Error(err, "Failed to apply "+api.SystemReplicationAnnotation+" patch")
		return nil, err
	}

	return &replication, nil
}

// updateReplicationOfSystemKeyspaces ensures that the replication for the system_auth,
// system_traces, and system_distributed keyspaces is up to date. It ensures that there are
// replicas for each DC and that there is a max of 3 replicas per DC.
func (r *K8ssandraClusterReconciler) updateReplicationOfSystemKeyspaces(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger) result.ReconcileResult {

	if recResult := r.versionCheck(ctx, kc); recResult.Completed() {
		return recResult
	}

	managementApiFacade, err := r.ManagementApi.NewManagementApiFacade(ctx, dc, remoteClient, logger)
	if err != nil {
		logger.Error(err, "Failed to create ManagementApiFacade")
		return result.Error(err)
	}

	keyspaces := getReplicatedInternalKeyspaces(kc)
	datacenters := cassandra.GetDatacentersForReplication(kc)
	replication := cassandra.ComputeReplication(3, datacenters...)

	logger.Info("Preparing to update replication for system keyspaces", "replication", replication)

	for _, ks := range keyspaces {
		if err := managementApiFacade.EnsureKeyspaceReplication(ks, replication); err != nil {
			logger.Error(err, "Failed to update replication", "keyspace", ks)
			return result.Error(err)
		}
	}

	return result.Continue()
}

// updateUserKeyspacesReplication updates the replication factor of user-defined keyspaces.
// The K8ssandraCluster must specify the k8ssandra.io/dc-replication in order for any
// updates to be applied. The annotation can specify multiple DCs but only the DC just
// added will be considered for replication changes. For example, if dc2 is added to a
// cluster that has dc1 and if the annotation specifies changes for both dc1 and dc2, only
// changes for dc2 will be applied. Replication for all user-defined keyspaces must be
// specified; otherwise an error is returned. This is required to avoid surprises for the
// user.
func (r *K8ssandraClusterReconciler) updateUserKeyspacesReplication(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger) result.ReconcileResult {

	jsonReplication := annotations.GetAnnotation(kc, api.DcReplicationAnnotation)
	if jsonReplication == "" {
		logger.Info(api.DcReplicationAnnotation + " not set. Replication for user keyspaces will not be updated")
		return result.Continue()
	}

	logger.Info("Updating replication for user keyspaces")

	mgmtApi, err := r.ManagementApi.NewManagementApiFacade(ctx, dc, remoteClient, logger)
	if err != nil {
		logger.Error(err, "Failed to create ManagementApiFacade")
		return result.Error(err)
	}

	userKeyspaces, err := getUserKeyspaces(mgmtApi, kc)
	if err != nil {
		logger.Error(err, "Failed to get user keyspaces")
		return result.Error(err)
	}

	replication, err := cassandra.ParseReplication([]byte(jsonReplication))
	if err != nil {
		logger.Error(err, "Failed to parse replication")
		return result.Error(err)
	}

	// The replication object can specify multiple DCs, new ones to be added to the cluster
	// as well as existing DCs. We need to be careful about a couple of things. First, we do
	// not want to modify the replication for existing DCs since this is not intended as a
	// general purpose mechanism for managing keyspace replication. Secondly, we need to
	// make sure we only update the replication for the DC just added to the C* cluster and
	// not others which have been added to the K8ssandraCluster but not yet rolled out. This
	// is because Cassandra 4 does not allow you to update a keyspace's replication with a
	// non-existent DC.

	// This is validation check to make sure the user specifies all user keyspaces for each
	// DC listed in the annotation. We want to force the user to be explicit to avoid any
	// surprises.
	if !replication.EachDcContainsKeyspaces(userKeyspaces...) {
		err = fmt.Errorf("the %s annotation must include all user keyspaces for each specified DC", api.DcReplicationAnnotation)
		logger.Error(err, "Invalid "+api.DcReplicationAnnotation+" annotation")
		return result.Error(err)
	}

	replication = getReplicationForDeployedDcs(kc, replication)

	logger.Info("computed replication")

	for _, ks := range userKeyspaces {
		replicationFactor := replication.ReplicationFactor(dc.Name, ks)
		logger.Info("computed replication factor", "keyspace", ks, "replication_factor", replicationFactor)
		if replicationFactor == 0 {
			continue
		}
		if err = ensureKeyspaceReplication(mgmtApi, ks, dc.Name, replicationFactor); err != nil {
			logger.Error(err, "Keyspace replication check failed", "Keyspace", ks)
			return result.Error(err)
		}
	}

	return result.Continue()
}

func getUserKeyspaces(mgmtApi cassandra.ManagementApiFacade, kc *api.K8ssandraCluster) ([]string, error) {
	keyspaces, err := mgmtApi.ListKeyspaces("")
	if err != nil {
		return nil, err
	}

	internalKeyspaces := getInternalKeyspaces(kc)
	userKeyspaces := make([]string, 0)

	for _, ks := range keyspaces {
		if !utils.SliceContains(internalKeyspaces, ks) {
			userKeyspaces = append(userKeyspaces, ks)
		}
	}

	return userKeyspaces, nil
}

// getReplicatedInternalKeyspaces returns Cassandra, internal keyspaces that should be
// replicated, namely system_auth. The slice also includes the Stargate auth keyspace if
// Stargate is enabled and the Reaper keyspace if Reaper is enabled.
func getReplicatedInternalKeyspaces(kc *api.K8ssandraCluster) []string {
	keyspaces := []string{"system_traces", "system_distributed", "system_auth"}

	if kc.HasStargates() {
		keyspaces = append(keyspaces, stargate.AuthKeyspace)
	}

	if kc.HasReapers() {
		if kc.Spec.Reaper != nil && kc.Spec.Reaper.Keyspace != "" {
			keyspaces = append(keyspaces, kc.Spec.Reaper.Keyspace)
		} else {
			keyspaces = append(keyspaces, reaperapi.DefaultKeyspace)
		}
	}

	return keyspaces
}

// getInternalKeyspaces returns all internal Cassandra keyspaces as well as the Stargate
// auth and Reaper keyspaces if Stargate and Reaper are enabled.
func getInternalKeyspaces(kc *api.K8ssandraCluster) []string {
	keyspaces := getReplicatedInternalKeyspaces(kc)
	keyspaces = append(keyspaces, "system", "system_schema")
	return keyspaces
}

// getReplicationForDeployedDcs gets the replication for only those DCs that have already
// been deployed. The replication argument may include DCs that have not yet been deployed.
func getReplicationForDeployedDcs(kc *api.K8ssandraCluster, replication *cassandra.Replication) *cassandra.Replication {
	dcNames := make([]string, 0)
	for _, dc := range kc.Spec.Cassandra.Datacenters {
		if status, found := kc.Status.Datacenters[dc.Meta.Name]; found && status.Cassandra.GetConditionStatus(cassdcapi.DatacenterInitialized) == corev1.ConditionTrue {
			dcNames = append(dcNames, dc.Meta.Name)
		}
	}

	return replication.ForDcs(dcNames...)
}

func ensureKeyspaceReplication(mgmtApi cassandra.ManagementApiFacade, ks, dcName string, replicationFactor int) error {
	replication, err := getKeyspaceReplication(mgmtApi, ks)
	if err != nil {
		return err
	}

	replication[dcName] = replicationFactor

	return mgmtApi.EnsureKeyspaceReplication(ks, replication)
}

// getKeyspaceReplication returns a map of DCs to their replica counts for ks.
func getKeyspaceReplication(mgmtApi cassandra.ManagementApiFacade, ks string) (map[string]int, error) {
	settings, err := mgmtApi.GetKeyspaceReplication(ks)
	if err != nil {
		return nil, err
	}

	replication := make(map[string]int)
	for k, v := range settings {
		if k == "class" {
			continue
		}
		count, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		replication[k] = count
	}

	return replication, nil
}

func (r *K8ssandraClusterReconciler) versionCheck(ctx context.Context, kc *api.K8ssandraCluster) result.ReconcileResult {
	kcCopy := kc.DeepCopy()
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	if err := r.ClientCache.GetLocalClient().Patch(ctx, kc, patch); err != nil {
		if errors.IsConflict(err) {
			return result.RequeueSoon(1 * time.Second)
		}
		return result.Error(fmt.Errorf("k8ssandracluster version check failed: %v", err))
	}
	// Need to copy the status here as in-memory status updates can be lost by results
	// returned from the api server.
	kc.Status = kcCopy.Status

	return result.Continue()
}
