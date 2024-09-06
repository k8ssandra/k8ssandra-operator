package k8ssandra

import (
	"context"
	"errors"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/nodeconfig"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var perNodeConfigNotOwnedByK8ssandraOperator = errors.New("per-node configuration already exists and is not owned by k8ssandra-operator")

func (r *K8ssandraClusterReconciler) reconcilePerNodeConfiguration(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcConfig *cassandra.DatacenterConfig,
	remoteClient client.Client,
	dcLogger logr.Logger,
) result.ReconcileResult {
	if dcConfig.PerNodeConfigMapRef.Name == "" {
		return r.reconcileDefaultPerNodeConfiguration(ctx, kc, dcConfig, remoteClient, dcLogger)
	} else {
		return r.reconcileUserProvidedPerNodeConfiguration(ctx, kc, dcConfig, remoteClient, dcLogger)
	}
}

func (r *K8ssandraClusterReconciler) reconcileDefaultPerNodeConfiguration(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcConfig *cassandra.DatacenterConfig,
	remoteClient client.Client,
	dcLogger logr.Logger,
) result.ReconcileResult {

	kcKey := utils.GetKey(kc)

	perNodeConfigKey := nodeconfig.NewDefaultPerNodeConfigMapKey(kc, dcConfig)
	dcLogger = dcLogger.WithValues("PerNodeConfigMap", perNodeConfigKey)

	desiredPerNodeConfig := nodeconfig.NewDefaultPerNodeConfigMap(kcKey, kc, dcConfig)
	if desiredPerNodeConfig != nil {
		annotations.AddHashAnnotation(desiredPerNodeConfig)
	}

	actualPerNodeConfig := &corev1.ConfigMap{}
	err := remoteClient.Get(ctx, perNodeConfigKey, actualPerNodeConfig)

	if err != nil {

		if !apierrors.IsNotFound(err) {
			dcLogger.Error(err, "Failed to get per-node configuration")
			return result.Error(err)
		}

		// Create
		if desiredPerNodeConfig != nil {
			// Note: cannot set controller reference on remote objects
			if err = remoteClient.Create(ctx, desiredPerNodeConfig); err != nil {
				if apierrors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created
					// already; simply requeue until the cache is up-to-date
					dcLogger.Info("Per-node configuration already exists, requeueing")
					return result.RequeueSoon(r.DefaultDelay)
				} else {
					dcLogger.Error(err, "Failed to create per-node configuration")
					return result.Error(err)
				}
			}
			dcLogger.Info("Created per-node configuration")
		}

	} else if desiredPerNodeConfig != nil {

		// Update
		if !k8ssandralabels.IsOwnedByK8ssandraController(actualPerNodeConfig) {
			dcLogger.Error(perNodeConfigNotOwnedByK8ssandraOperator, "Failed to update per-node configuration")
			return result.Error(err)

		} else if !annotations.CompareHashAnnotations(actualPerNodeConfig, desiredPerNodeConfig) {
			resourceVersion := actualPerNodeConfig.GetResourceVersion()
			desiredPerNodeConfig.DeepCopyInto(actualPerNodeConfig)
			actualPerNodeConfig.SetResourceVersion(resourceVersion)
			if err = remoteClient.Update(ctx, actualPerNodeConfig); err != nil {
				dcLogger.Error(err, "Failed to update per-node configuration")
				return result.Error(err)
			}
			dcLogger.Info("Updated per-node configuration")
		}

	} else {

		// Delete
		if !k8ssandralabels.IsOwnedByK8ssandraController(actualPerNodeConfig) {
			dcLogger.Error(perNodeConfigNotOwnedByK8ssandraOperator, "Failed to delete per-node configuration")
			return result.Error(err)

		} else if err = remoteClient.Delete(ctx, actualPerNodeConfig); err != nil {
			dcLogger.Error(err, "Failed to delete per-node configuration")
			return result.Error(err)
		}
		dcLogger.Info("Deleted per-node configuration")
	}

	if desiredPerNodeConfig != nil {
		dcConfig.PerNodeConfigMapRef.Name = desiredPerNodeConfig.Name
		perNodeConfigMapHash := annotations.GetAnnotation(desiredPerNodeConfig, k8ssandraapi.ResourceHashAnnotation)
		annotations.AddAnnotation(&dcConfig.PodTemplateSpec, k8ssandraapi.PerNodeConfigHashAnnotation, perNodeConfigMapHash)
		nodeconfig.MountPerNodeConfig(dcConfig)
		dcLogger.Info("Mounted per-node configuration")
	}

	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileUserProvidedPerNodeConfiguration(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcConfig *cassandra.DatacenterConfig,
	remoteClient client.Client,
	dcLogger logr.Logger,
) result.ReconcileResult {

	kcKey := utils.GetKey(kc)

	perNodeConfigKey := types.NamespacedName{
		Name:      dcConfig.PerNodeConfigMapRef.Name,
		Namespace: utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kc.Namespace),
	}
	dcLogger = dcLogger.WithValues("PerNodeConfigMap", perNodeConfigKey)

	actualPerNodeConfig := &corev1.ConfigMap{}
	err := remoteClient.Get(ctx, perNodeConfigKey, actualPerNodeConfig)

	if err == nil {

		if !k8ssandralabels.IsWatchedByK8ssandraCluster(actualPerNodeConfig, kcKey) {

			// We set the configmap as managed by the operator so that we are notified of changes to
			// its contents. Note that we do NOT set the configmap as owned by the operator, nor do
			// we set our controller reference on it.
			patch := client.MergeFromWithOptions(actualPerNodeConfig.DeepCopy())
			k8ssandralabels.SetWatchedByK8ssandraCluster(actualPerNodeConfig, kcKey)
			if err = remoteClient.Patch(ctx, actualPerNodeConfig, patch); err != nil {
				dcLogger.Error(err, "Failed to set user-provided per-node config managed by k8ssandra-operator")
				return result.Error(err)
			}
		}

		// Note: when the per-node config is user-provided, we don't check its contents, we just
		// mount it as is. Also, we don't manage its full lifecycle.

		perNodeConfigMapHash := utils.DeepHashString(actualPerNodeConfig)
		annotations.AddAnnotation(&dcConfig.PodTemplateSpec, k8ssandraapi.PerNodeConfigHashAnnotation, perNodeConfigMapHash)
		nodeconfig.MountPerNodeConfig(dcConfig)
		dcLogger.Info("Mounted per-node configuration")

	} else {

		dcLogger.Error(err, "Failed to retrieve user-provided per-node config")
		return result.Error(err)
	}

	return result.Continue()
}

// setupMedusaCleanup adds an owner reference to ensure that the remote ConfigMap created by
// reconcileDefaultPerNodeConfiguration is correctly cleaned up when the CassandraDatacenter is deleted. We do that in a
// second pass because the CassandraDatacenter did not exist yet at the time the ConfigMap was created.
func (r *K8ssandraClusterReconciler) setupPerNodeConfigurationCleanup(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger,
) result.ReconcileResult {
	configMapKey := client.ObjectKey{
		Namespace: dc.Namespace,
		Name:      kc.SanitizedName() + "-" + dc.SanitizedName() + "-metrics-agent-config",
	}
	return setDcOwnership(ctx, dc, configMapKey, &corev1.ConfigMap{}, remoteClient, r.Scheme, logger)
}
