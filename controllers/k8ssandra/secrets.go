package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

func (r *K8ssandraClusterReconciler) reconcileSuperuserSecret(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if kc.Spec.Cassandra.SuperuserSecretName == "" {
		// Note that we do not persist this change because doing so would prevent us from
		// differentiating between a default secret by the operator vs one provided by the
		// client that happens to have the same name as the default name.
		kc.Spec.Cassandra.SuperuserSecretName = secret.DefaultSuperuserSecretName(kc.Spec.Cassandra.Cluster)
		logger.Info("Setting default superuser secret", "SuperuserSecretName", kc.Spec.Cassandra.SuperuserSecretName)
	}

	if err := secret.ReconcileSecret(ctx, r.Client, kc.Spec.Cassandra.SuperuserSecretName, utils.GetKey(kc)); err != nil {
		logger.Error(err, "Failed to verify existence of superuserSecret")
		return result.Error(err)
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
