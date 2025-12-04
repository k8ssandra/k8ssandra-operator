package k8ssandra

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	kerrors "github.com/k8ssandra/k8ssandra-operator/pkg/errors"
	"github.com/k8ssandra/k8ssandra-operator/pkg/k8ssandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// an annotation is used to store the initial replication factor for system keyspaces.
// its schema evolved in v1.1 but we need to be able to parse the old format for upgrades.
type SystemReplicationOldFormat struct {
	Datacenters       []string `json:"datacenters"`
	ReplicationFactor int      `json:"replicationFactor"`
}

func (r *K8ssandraClusterReconciler) checkSchemas(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger) result.ReconcileResult {

	mgmtApi, err := r.ManagementApi.NewManagementApiFacade(ctx, dc, remoteClient, logger)
	if err != nil {
		return result.Error(err)
	}

	if recResult := r.updateReplicationOfSystemKeyspaces(ctx, kc, mgmtApi, logger); recResult.Completed() {
		return recResult
	}

	if recResult := r.reconcileStargateAuthSchema(ctx, kc, mgmtApi, logger); recResult.Completed() {
		return recResult
	}

	// we only want to reconcile the Reaper schema if we're deploying Reaper together with k8ssandra cluster
	// otherwise we just register the k8ssandra cluster with an external reaper (happens after reconciling the DCs)
	if kc.Spec.Reaper != nil && !kc.Spec.Reaper.HasReaperRef() {
		if recResult := r.reconcileReaperSchema(ctx, kc, mgmtApi, logger); recResult.Completed() {
			return recResult
		}
	}

	decommCassDcName := k8ssandra.GetDatacenterForDecommission(kc)

	logger.Info("Checking if user keyspace replication needs to be updated", "decommissioning_dc", decommCassDcName)
	decommission := false
	status := kc.Status.Datacenters[decommCassDcName]
	if decommCassDcName != "" {
		decommission = status.DecommissionProgress == api.DecommUpdatingReplication
	}

	if decommission {
		kcKey := utils.GetKey(kc)
		logger.Info("Decommissioning DC", "dc", decommCassDcName, "context", status.ContextName)

		var dcRemoteClient client.Client
		if status.ContextName == "" {
			dcRemoteClient = remoteClient
		} else {
			dcRemoteClient, err = r.ClientCache.GetRemoteClient(status.ContextName)
			if err != nil {
				return result.Error(err)
			}
		}

		dc, _, err = r.findDcForDeletion(ctx, kcKey, decommCassDcName, dcRemoteClient)
		if err != nil {
			return result.Error(err)
		}

		decommDcName := decommCassDcName
		if dc.Spec.DatacenterName != "" {
			decommDcName = dc.Spec.DatacenterName
		}

		if recResult := r.checkUserKeyspacesReplicationForDecommission(kc, decommDcName, mgmtApi, logger); recResult.Completed() {
			return recResult
		}
		status.DecommissionProgress = api.DecommDeleting
		kc.Status.Datacenters[decommCassDcName] = status
		return result.RequeueSoon(r.DefaultDelay)
	} else if annotations.HasAnnotationWithValue(kc, api.RebuildDcAnnotation, dc.Name) {
		if recResult := r.updateUserKeyspacesReplication(kc, dc, mgmtApi, logger); recResult.Completed() {
			return recResult
		}
	}

	return result.Continue()
}

// checkInitialSystemReplication checks for the InitialSystemReplicationAnnotation on kc. If found, the
// JSON value is unmarshalled and returned. If not found, the SystemReplication is computed
// and is stored in the InitialSystemReplicationAnnotation on kc. The value is JSON-encoded.
// Lastly, kc is patched so that the changes are persisted,
func (r *K8ssandraClusterReconciler) checkInitialSystemReplication(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	logger logr.Logger) (cassandra.SystemReplication, error) {

	replication := make(map[string]int)
	if val := annotations.GetAnnotation(kc, api.InitialSystemReplicationAnnotation); val != "" {
		if err := json.Unmarshal([]byte(val), &replication); err == nil {
			return replication, nil
		} else {
			// We could have an annotation in the old format. Try to parse it and convert it to the new format by pursuing the execution.
			var replicationOldFormat SystemReplicationOldFormat
			if err2 := json.Unmarshal([]byte(val), &replicationOldFormat); err2 == nil {
				replication = make(map[string]int)
				for _, dc := range replicationOldFormat.Datacenters {
					replication[dc] = replicationOldFormat.ReplicationFactor
				}
			} else {
				logger.Error(err, "could not parse the initial-system-replication annotation of the K8ssandraCluster object")
				return nil, err
			}
		}
	} else {
		// Cassandra 4.1 writes the superuser role with CL=EACH_QUORUM, so we can't reference DCs before they are created.
		// Only reference the first DC in the replication; for subsequent DCs, the replication will be altered.
		firstDc := kc.Spec.Cassandra.Datacenters[0]
		replication = cassandra.ComputeReplicationFromDatacenters(3, kc.Spec.ExternalDatacenters, firstDc)
	}

	bytes, err := json.Marshal(replication)

	if err != nil {
		logger.Error(err, "Failed to marshal SystemReplication", "SystemReplication", replication)
		return nil, err
	}

	patch := client.MergeFromWithOptions(kc.DeepCopy())
	if kc.Annotations == nil {
		kc.Annotations = make(map[string]string)
	}
	kc.Annotations[api.InitialSystemReplicationAnnotation] = string(bytes)
	logger.Info("New initial system replication", "SystemReplication", string(bytes))
	if err = r.Patch(ctx, kc, patch); err != nil {
		logger.Error(err, "Failed to apply "+api.InitialSystemReplicationAnnotation+" patch")
		return nil, err
	}

	return replication, nil
}

// updateReplicationOfSystemKeyspaces ensures that the replication for the system_auth,
// system_traces, and system_distributed keyspaces is up to date.
// DSE has a few specific system keyspaces that need to be updated as well.
// It ensures that there are replicas for each DC and that there is a max of 3 replicas per DC.
func (r *K8ssandraClusterReconciler) updateReplicationOfSystemKeyspaces(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	mgmtApi cassandra.ManagementApiFacade,
	logger logr.Logger) result.ReconcileResult {

	if recResult := r.versionCheck(ctx, kc); recResult.Completed() {
		return recResult
	}

	if kc.Spec.Cassandra.ServerType == api.ServerDistributionCassandra {
		versionString := kc.Spec.Cassandra.ServerVersion
		version, err := semver.NewVersion(versionString)
		if err == nil {
			if version.GreaterThanEqual(semver.MustParse("4.0.0")) && len(kc.Status.Datacenters) > len(kc.Spec.Cassandra.Datacenters) {
				// A DC is being decommissioned and Cassandra 4.1+ will require to keep system_auth replicas until the DC is gone.
				return result.Continue()
			}
		} else {
			logger.Error(err, "Failed to parse version", "version", versionString)
			return result.Error(err)
		}
	}

	replication := cassandra.ComputeReplicationFromDatacenters(3, kc.Spec.ExternalDatacenters, kc.GetInitializedDatacenters()...)

	logger.Info("Preparing to update replication for system keyspaces", "replication", replication)

	systemKeyspaces := append([]string{}, api.SystemKeyspaces...)
	if kc.Spec.Cassandra.ServerType == api.ServerDistributionDse {
		// DSE has a few more system keyspaces
		systemKeyspaces = append(systemKeyspaces, api.DseKeyspaces...)
	}

	for _, ks := range systemKeyspaces {
		if err := mgmtApi.EnsureKeyspaceReplication(ks, replication); err != nil {
			if kerrors.IsSchemaDisagreement(err) {
				return result.RequeueSoon(r.DefaultDelay)
			}
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
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	mgmtApi cassandra.ManagementApiFacade,
	logger logr.Logger) result.ReconcileResult {

	jsonReplication := annotations.GetAnnotation(kc, api.DcReplicationAnnotation)
	if jsonReplication == "" {
		logger.Info(api.DcReplicationAnnotation + " not set. Replication for user keyspaces will not be updated")
		return result.Continue()
	}

	logger.Info("Updating replication for user keyspaces")

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

	for _, ks := range userKeyspaces {
		replicationFactor := replication.ReplicationFactor(dc.DatacenterName(), ks)
		logger.Info("computed replication factor", "keyspace", ks, "replication_factor", replicationFactor)
		if replicationFactor == 0 {
			continue
		}
		if err = ensureKeyspaceReplication(mgmtApi, ks, dc.DatacenterName(), replicationFactor); err != nil {
			if kerrors.IsSchemaDisagreement(err) {
				return result.RequeueSoon(r.DefaultDelay)
			}
			logger.Error(err, "Keyspace replication check failed", "Keyspace", ks)
			return result.Error(err)
		}
	}

	return result.Continue()
}

// checkUserKeyspacesReplicationForDecommission checks if no user keyspace still has replicas
// for a DC going being decommissioned.
func (r *K8ssandraClusterReconciler) checkUserKeyspacesReplicationForDecommission(
	kc *api.K8ssandraCluster,
	decommDc string,
	mgmtApi cassandra.ManagementApiFacade,
	logger logr.Logger) result.ReconcileResult {

	logger.Info("Updating replication for user keyspaces for decommission")

	userKeyspaces, err := getUserKeyspaces(mgmtApi, kc)
	if err != nil {
		return result.Error(fmt.Errorf("failed to get user keyspaces: %v", err))
	}

	for _, ks := range userKeyspaces {
		replication, err := getKeyspaceReplication(mgmtApi, ks)
		if err != nil {
			return result.Error(fmt.Errorf("failed to get replication for keyspace (%s): %v", ks, err))
		}
		logger.Info("checking keyspace replication", "keyspace", ks, "replication", replication, "decommissioning_dc", decommDc)
		if _, hasReplicas := replication[decommDc]; hasReplicas {
			return result.Error(fmt.Errorf("cannot decommission DC %s: keyspace %s still has replicas on it", decommDc, ks))
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

// getInternalKeyspaces returns all internal Cassandra keyspaces as well as the Stargate
// auth and Reaper keyspaces if Stargate and Reaper are enabled.
func getInternalKeyspaces(kc *api.K8ssandraCluster) []string {
	keyspaces := api.SystemKeyspaces

	if kc.HasStargates() {
		keyspaces = append(keyspaces, stargate.AuthKeyspace)
	}

	if kc.Spec.Reaper != nil {
		keyspaces = append(keyspaces, getReaperKeyspace(kc))
	}

	keyspaces = append(keyspaces, "system", "system_schema", "system_views", "system_virtual_schema")
	return keyspaces
}

// getReplicationForDeployedDcs gets the replication for only those DCs that have already
// been deployed. The replication argument may include DCs that have not yet been deployed.
func getReplicationForDeployedDcs(kc *api.K8ssandraCluster, replication *cassandra.Replication) *cassandra.Replication {
	dcNames := make([]string, 0)
	for _, template := range kc.GetInitializedDatacenters() {
		dcNames = append(dcNames, template.CassDcName())
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
