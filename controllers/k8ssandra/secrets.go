package k8ssandra

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileSuperuserSecret(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	// Note that we always create a superuser secret, even when authentication is globally disabled on the cluster.
	// This is because cass-operator would create a DC-specific superuser if we don't create a cluster-wide one here,
	// and also because if we don't create the secret when the K8ssandraCluster is created, then it would be impossible
	// to turn on authentication later on, since kc.Spec.Cassandra.SuperuserSecretRef is immutable.
	// Finally, creating the superuser secret when auth is disabled does not do any harm: no credentials will be
	// required to connect to Cassandra nodes by CQL nor JMX.
	if kc.Spec.Cassandra.SuperuserSecretRef.Name == "" {
		patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
		kc.Spec.Cassandra.SuperuserSecretRef.Name = secret.DefaultSuperuserSecretName(kc.SanitizedName())
		if err := r.Patch(ctx, kc, patch); err != nil {
			if errors.IsConflict(err) {
				return result.RequeueSoon(1 * time.Second)
			}
			return result.Error(fmt.Errorf("failed to set default superuser secret name: %v", err))
		}

		kc.Spec.Cassandra.SuperuserSecretRef.Name = secret.DefaultSuperuserSecretName(kc.SanitizedName())
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
			var cassandraUserSecretRef corev1.LocalObjectReference
			var jmxUserSecretRef corev1.LocalObjectReference
			var uiUserSecretRef corev1.LocalObjectReference
			if kc.Spec.Reaper != nil {
				cassandraUserSecretRef = kc.Spec.Reaper.CassandraUserSecretRef
				jmxUserSecretRef = kc.Spec.Reaper.JmxUserSecretRef
				uiUserSecretRef = kc.Spec.Reaper.UiUserSecretRef
			}
			if cassandraUserSecretRef.Name == "" {
				cassandraUserSecretRef.Name = reaper.DefaultUserSecretName(kc.SanitizedName())
			}
			if jmxUserSecretRef.Name == "" {
				jmxUserSecretRef.Name = reaper.DefaultJmxUserSecretName(kc.SanitizedName())
			}
			if uiUserSecretRef.Name == "" {
				uiUserSecretRef.Name = reaper.DefaultUiSecretName(kc.SanitizedName())
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
			if err := secret.ReconcileSecret(ctx, r.Client, uiUserSecretRef.Name, kcKey); err != nil {
				logger.Error(err, "Failed to reconcile Reaper UI secret", "ReaperUiUserSecretRef", uiUserSecretRef)
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
