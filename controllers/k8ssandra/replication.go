package k8ssandra

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// checkSystemReplication checks for the SystemReplicationAnnotation on kc. If found, the
// JSON value is unmarshalled and returned. If not found, the SystemReplication is computed
// and is stored in the SystemReplicationAnnotation on kc. The value is JSON-encoded.
// Lastly, kc is patched so that the changes are persisted,
func (r *K8ssandraClusterReconciler) checkSystemReplication(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) (*cassandra.SystemReplication, error) {
	if val := utils.GetAnnotation(kc, api.SystemReplicationAnnotation); val != "" {
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
func (r *K8ssandraClusterReconciler) updateReplicationOfSystemKeyspaces(ctx context.Context, kc *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client, logger logr.Logger) result.ReconcileResult {
	managementApiFacade, err := r.ManagementApi.NewManagementApiFacade(ctx, dc, remoteClient, logger)
	if err != nil {
		logger.Error(err, "Failed to create ManagementApiFacade")
		return result.Error(err)
	}

	keyspaces := []string{"system_traces", "system_distributed", "system_auth"}
	replication := cassandra.ComputeReplication(3, kc.Spec.Cassandra.Datacenters...)

	logger.Info("Preparing to update replication for system keyspaces", "replication", replication)

	for _, ks := range keyspaces {
		if err := managementApiFacade.EnsureKeyspaceReplication(ks, replication); err != nil {
			logger.Error(err, "Failed to update replication", "keyspace", ks)
			return result.Error(err)
		}
	}

	return result.Continue()
}
