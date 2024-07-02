package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

func (r *K8ssandraClusterReconciler) reconcileSuperuserSecret(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	// If secrets provider is set to external, the CQL user will be created by a kubernetes job and does not need to be set
	// in the cassdc settings
	if kc.Spec.UseExternalSecrets() {
		return result.Continue()
	}

	// Note that we always create a superuser secret, even when authentication is globally disabled on the cluster.
	// This is because cass-operator would create a DC-specific superuser if we don't create a cluster-wide one here,
	// and also because if we don't create the secret when the K8ssandraCluster is created, then it would be impossible
	// to turn on authentication later on, since kc.Spec.Cassandra.SuperuserSecretRef is immutable.
	// Finally, creating the superuser secret when auth is disabled does not do any harm: no credentials will be
	// required to connect to Cassandra nodes by CQL nor JMX.
	if err := secret.ReconcileSecret(ctx, r.Client, SuperuserSecretName(kc), utils.GetKey(kc)); err != nil {
		logger.Error(err, "Failed to verify existence of superuserSecret")
		return result.Error(err)
	}

	return result.Continue()
}

func SuperuserSecretName(kc *api.K8ssandraCluster) string {
	if kc.Spec.Cassandra.SuperuserSecretRef.Name == "" {
		return secret.DefaultSuperuserSecretName(kc.SanitizedName())
	}
	return kc.Spec.Cassandra.SuperuserSecretRef.Name
}

func (r *K8ssandraClusterReconciler) reconcileReaperSecrets(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if kc.Spec.Reaper != nil && kc.Spec.Reaper.ReaperRef.Name != "" {
		logger.Info("ReaperRef points to an existing Reaper, we don't need a new Reaper CQL secret")
		return result.Continue()
	}
	if kc.Spec.Reaper != nil {
		// Reaper secrets are only required when authentication is enabled on the cluster
		if kc.Spec.IsAuthEnabled() && !kc.Spec.UseExternalSecrets() {
			logger.Info("Reconciling Reaper user secrets")
			var cassandraUserSecretRef corev1.LocalObjectReference
			var uiUserSecretRef corev1.LocalObjectReference
			if kc.Spec.Reaper != nil {
				cassandraUserSecretRef = kc.Spec.Reaper.CassandraUserSecretRef
				if kc.Spec.Reaper.UiUserSecretRef != nil {
					uiUserSecretRef = *kc.Spec.Reaper.UiUserSecretRef
				}
			}
			if cassandraUserSecretRef.Name == "" {
				cassandraUserSecretRef.Name = reaper.DefaultUserSecretName(kc.SanitizedName())
			}
			if kc.Spec.Reaper.UiUserSecretRef == nil {
				uiUserSecretRef.Name = reaper.DefaultUiSecretName(kc.SanitizedName())
			}
			kcKey := utils.GetKey(kc)
			if kc.Spec.Reaper != nil && kc.Spec.Reaper.ReaperRef.Name == "" {
				// We reconcile the CQL user secret only if are reconciling k8ssandra-cluster-specific Reaper
				if err := secret.ReconcileSecret(ctx, r.Client, cassandraUserSecretRef.Name, kcKey); err != nil {
					logger.Error(err, "Failed to reconcile Reaper CQL user secret", "ReaperCassandraUserSecretRef", cassandraUserSecretRef)
					return result.Error(err)
				}
			}
			if kc.Spec.Reaper.UiUserSecretRef == nil || kc.Spec.Reaper.UiUserSecretRef.Name != "" {
				if err := secret.ReconcileSecret(ctx, r.Client, uiUserSecretRef.Name, kcKey); err != nil {
					logger.Error(err, "Failed to reconcile Reaper UI secret", "ReaperUiUserSecretRef", uiUserSecretRef)
					return result.Error(err)
				}
			}
			logger.Info("Reaper user secrets successfully reconciled")

		} else if kc.Spec.IsAuthEnabled() && kc.Spec.UseExternalSecrets() {
			// Auth is enabled in the cluster, but the SecretsProvider is set to external, so no secret need to
			// be reconciled. Secrets will be injected into the Reaper pod by the mutating webhook configured by
			// the user to retrieve secrets from the external secret store of their choice.
			logger.Info("Auth is enabled and SecretsProvider is 'external': skipping Reaper user secret reconciliation")
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
	if kc.Spec.UseExternalSecrets() {
		return result.Continue()
	}

	if err := secret.ReconcileReplicatedSecret(ctx, r.Client, r.Scheme, kc, logger); err != nil {
		logger.Error(err, "Failed to reconcile ReplicatedSecret")
		return result.Error(err)
	}
	return result.Continue()
}
