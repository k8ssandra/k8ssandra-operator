package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

func (r *K8ssandraClusterReconciler) reconcileSuperuserSecret(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	// Note that we always create a superuser secret, even when authentication is globally disabled on the cluster.
	// This is because cass-operator would create a DC-specific superuser if we don't create a cluster-wide one here,
	// and also because if we don't create the secret when the K8ssandraCluster is created, then it would be impossible
	// to turn on authentication later on, since kc.Spec.Cassandra.SuperuserSecretRef is immutable.
	// Finally, creating the superuser secret when auth is disabled does not do any harm: no credentials will be
	// required to connect to Cassandra nodes by CQL nor JMX.
	if kc.Spec.Cassandra.SuperuserSecretRef.Name == "" {
		// Note that we do not persist this change because doing so would prevent us from
		// differentiating between a default secret by the operator vs one provided by the
		// client that happens to have the same name as the default name.
		kc.Spec.Cassandra.SuperuserSecretRef.Name = secret.DefaultSuperuserSecretName(kc.Spec.Cassandra.Cluster)
		logger.Info("Setting default superuser secret", "SuperuserSecretName", kc.Spec.Cassandra.SuperuserSecretRef.Name)
	}

	if err := secret.ReconcileSecret(ctx, r.Client, kc.Spec.Cassandra.SuperuserSecretRef.Name, utils.GetKey(kc)); err != nil {
		logger.Error(err, "Failed to verify existence of superuserSecret")
		return result.Error(err)
	}

	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileReaperSecrets(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if kc.Spec.Reaper != nil {
		// Reaper secrets are only required when authentication is enabled on the cluster
		if kc.Spec.IsAuthEnabled() {
			logger.Info("Reconciling Reaper user secrets")
			cassandraUserSecretRef := kc.Spec.Reaper.CassandraUserSecretRef
			if cassandraUserSecretRef.Name == "" {
				cassandraUserSecretRef.Name = reaper.DefaultUserSecretName(kc.Spec.Cassandra.Cluster)
			}
			jmxUserSecretRef := kc.Spec.Reaper.JmxUserSecretRef
			if jmxUserSecretRef.Name == "" {
				jmxUserSecretRef.Name = reaper.DefaultJmxUserSecretName(kc.Spec.Cassandra.Cluster)
			}
			kcKey := utils.GetKey(kc)
			if err := secret.ReconcileSecret(ctx, r.Client, cassandraUserSecretRef.Name, kcKey); err != nil {
				logger.Error(err, "Failed to reconcile Reaper CQL user secret", "ReaperCassandraUserSecretRef", cassandraUserSecretRef)
				return result.Error(err)
			}
			if err := secret.ReconcileSecret(ctx, r.Client, jmxUserSecretRef.Name, kcKey); err != nil {
				logger.Error(err, "Failed to reconcile Reaper JMX user secret", "ReaperJmxUserSecretRef", jmxUserSecretRef)
				return result.Error(err)
			}
			logger.Info("Reaper user secrets successfully reconciled")
		} else {
			// Auth is disabled in the cluster, so no Reaper secrets need to be reconciled. Note that, if auth was
			// previously enabled, Reaper secrets may have been created, but they are now useless; we don't delete them
			// just yet though, because they are replicated to all contexts where K8ssandra is deployed, and are meant
			// to be deleted only when the whole K8ssandraCluster resource is deleted.
			logger.Info("Auth is disabled: skipping Reaper user secret reconciliation")
		}
	}
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileReplicatedSecret(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if err := secret.ReconcileReplicatedSecret(ctx, r.Client, r.Scheme, kc, logger); err != nil {
		logger.Error(err, "Failed to reconcile ReplicatedSecret")
		return result.Error(err)
	}
	return result.Continue()
}
