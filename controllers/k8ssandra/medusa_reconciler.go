package k8ssandra

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	cassandra "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	medusa "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Create all things Medusa related in the cassdc podTemplateSpec
func (r *K8ssandraClusterReconciler) ReconcileMedusa(
	ctx context.Context,
	dcConfig *cassandra.DatacenterConfig,
	dcTemplate api.CassandraDatacenterTemplate,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) result.ReconcileResult {
	remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
	if err != nil {
		return result.Error(err)
	}
	namespace := dcTemplate.Meta.Namespace
	if namespace == "" {
		namespace = kc.Namespace
	}
	logger.Info("Medusa reconcile for " + dcConfig.Meta.Name + " on namespace " + namespace)
	medusaSpec := kc.Spec.Medusa
	if medusaSpec != nil {
		logger.Info("Medusa is enabled")
		if dcConfig.PodTemplateSpec == nil {
			dcConfig.PodTemplateSpec = &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers:     []corev1.Container{},
					InitContainers: []corev1.Container{},
				},
			}
		}
		if res := r.reconcileMedusaConfigMap(ctx, remoteClient, kc, logger, namespace); res.Completed() {
			return res
		}
		medusa.UpdateMedusaInitContainer(dcConfig, medusaSpec, logger)
		medusa.UpdateMedusaMainContainer(dcConfig, medusaSpec, logger)
		medusa.UpdateMedusaVolumes(dcConfig, medusaSpec, logger)
	} else {
		logger.Info("Medusa is not enabled")
	}

	return result.Continue()
}

// Generate a secret for Medusa or use the existing one if provided in the spec
func (r *K8ssandraClusterReconciler) reconcileMedusaSecrets(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) result.ReconcileResult {
	logger.Info("Reconciling Medusa user secrets")
	if kc.Spec.Medusa != nil {
		cassandraUserSecretRef := kc.Spec.Medusa.CassandraUserSecretRef
		if cassandraUserSecretRef.Name == "" {
			cassandraUserSecretRef.Name = medusa.CassandraUserSecretName(kc.Spec.Medusa, kc.Name)
		}
		logger = logger.WithValues(
			"MedusaCassandraUserSecretRef",
			cassandraUserSecretRef,
		)
		kcKey := utils.GetKey(kc)
		if err := secret.ReconcileSecret(ctx, r.Client, cassandraUserSecretRef.Name, kcKey); err != nil {
			logger.Error(err, "Failed to reconcile Medusa CQL user secret")
			return result.Error(err)
		}
	}
	logger.Info("Medusa user secrets successfully reconciled")
	return result.Continue()
}

// Create the Medusa config map if it doesn't exist
func (r *K8ssandraClusterReconciler) reconcileMedusaConfigMap(
	ctx context.Context,
	remoteClient client.Client,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
	namespace string,
) result.ReconcileResult {
	logger.Info("Reconciling Medusa configMap on namespace : " + namespace)
	if kc.Spec.Medusa != nil {
		medusaIni := medusa.CreateMedusaIni(kc)
		configMapKey := client.ObjectKey{
			Namespace: kc.Namespace,
			Name:      fmt.Sprintf("%s-medusa", kc.Spec.Cassandra.Cluster),
		}

		logger := logger.WithValues("MedusaConfigMap", configMapKey)
		desiredConfigMap := medusa.CreateMedusaConfigMap(namespace, kc.Spec.Cassandra.Cluster, medusaIni)
		// Compute a hash which will allow to compare desired and actual configMaps
		annotations.AddHashAnnotation(desiredConfigMap)
		actualConfigMap := &corev1.ConfigMap{}

		if err := remoteClient.Get(ctx, configMapKey, actualConfigMap); err != nil {
			if errors.IsNotFound(err) {
				if err := remoteClient.Create(ctx, desiredConfigMap); err != nil {
					logger.Error(err, "Failed to create Medusa ConfigMap")
					return result.Error(err)
				}
			}
		}

		actualConfigMap = actualConfigMap.DeepCopy()

		if !annotations.CompareHashAnnotations(actualConfigMap, desiredConfigMap) {
			logger.Info("Updating configMap on namespace " + actualConfigMap.ObjectMeta.Namespace)
			resourceVersion := actualConfigMap.GetResourceVersion()
			desiredConfigMap.DeepCopyInto(actualConfigMap)
			actualConfigMap.SetResourceVersion(resourceVersion)
			if err := remoteClient.Update(ctx, actualConfigMap); err != nil {
				logger.Error(err, "Failed to update Medusa ConfigMap resource")
				return result.Error(err)
			}
			return result.RequeueSoon(r.DefaultDelay)
		}
	}
	logger.Info("Medusa ConfigMap successfully reconciled")
	return result.Continue()
}
