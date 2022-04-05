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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

		// Check that certificates are provided if client encryption is enabled
		if cassandra.ClientEncryptionEnabled(dcConfig) {
			if medusaSpec.CertificatesSecretRef.Name == "" {
				return result.Error(fmt.Errorf("medusa encryption certificates were not provided despite client encryption being enabled"))
			}
		}
		if medusaSpec.StorageProperties.StorageProvider != "local" && medusaSpec.StorageProperties.StorageSecretRef.Name == "" {
			return result.Error(fmt.Errorf("medusa storage secret is not defined for storage provider %s", medusaSpec.StorageProperties.StorageProvider))
		}
		if res := r.reconcileMedusaConfigMap(ctx, remoteClient, kc, logger, namespace); res.Completed() {
			return res
		}

		// Generate the Medusa main container
		medusaMainContainer := medusa.GenerateMedusaMainContainer(dcConfig, medusaSpec, logger)
		// Add the containers definitions in the podTemplateSpec
		medusa.UpdateMedusaInitContainer(dcConfig, medusaSpec, logger)
		medusa.UpdateMedusaMainContainer(dcConfig, medusaMainContainer, logger)
		// Create required volumes for Medusa
		additionalVolumes := medusa.UpdateMedusaVolumes(dcConfig.PodTemplateSpec, medusaSpec, dcConfig.Cluster, logger)
		for _, volume := range additionalVolumes {
			cassandra.AddOrUpdateAdditionalVolume(dcConfig, volume.Volume, volume.VolumeIndex, volume.Exists)
		}
		cassandra.AddCqlUser(medusaSpec.CassandraUserSecretRef, dcConfig, medusa.CassandraUserSecretName(medusaSpec, kc.Name))
		// Create the Medusa standalone pod
		medusaStandalone := medusa.StandaloneMedusaDeployment(medusaMainContainer, kc.Name, namespace, logger)
		medusa.UpdateMedusaVolumes(&medusaStandalone.Spec.Template, medusaSpec, dcConfig.Cluster, logger)
		medusaService := medusa.StandaloneMedusaService(dcConfig, medusaSpec, kc.Name, namespace, logger)
		r.reconcileMedusaStandalone(ctx, kc, remoteClient, medusaStandalone, medusaService, logger)
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
			Name:      fmt.Sprintf("%s-medusa", kc.Name),
		}

		logger := logger.WithValues("MedusaConfigMap", configMapKey)
		desiredConfigMap := medusa.CreateMedusaConfigMap(namespace, kc.Name, medusaIni)
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

func (r *K8ssandraClusterReconciler) reconcileMedusaStandalone(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	remoteClient client.Client,
	desiredDeployment *appsv1.Deployment,
	desiredService *corev1.Service,
	logger logr.Logger,
) result.ReconcileResult {

	logger.Info("Reconciling Medusa standalone deployment on namespace : " + desiredDeployment.ObjectMeta.Namespace)
	if kc.Spec.Medusa != nil {
		deploymentKey := utils.GetKey(desiredDeployment)

		logger := logger.WithValues("MedusaDeployment", deploymentKey)

		// Compute a hash which will allow to compare desired and actual deployments
		annotations.AddHashAnnotation(desiredDeployment)

		actualDeployment := &appsv1.Deployment{}
		if err := remoteClient.Get(ctx, deploymentKey, actualDeployment); err != nil {
			if errors.IsNotFound(err) {
				if err := remoteClient.Create(ctx, desiredDeployment); err != nil {
					logger.Error(err, "Failed to create Medusa deployment")
					return result.Error(err)
				}
			}
		}

		actualDeployment = actualDeployment.DeepCopy()

		if actualDeployment.OwnerReferences == nil {
			if err := controllerutil.SetControllerReference(kc, actualDeployment, r.Scheme); err != nil {
				logger.Error(err, "failed to set controller reference", "CassandraDatacenter", utils.GetKey(kc))
				return result.Error(err)
			}
			if err := r.Update(ctx, actualDeployment); err != nil {
				return result.Error(err)
			} else {
				logger.Info("updated task with owner reference", "CassandraDatacenter", utils.GetKey(kc))
				return result.RequeueSoon(r.DefaultDelay)
			}
		}

		if !annotations.CompareHashAnnotations(actualDeployment, desiredDeployment) {
			logger.Info("Updating Medusa deployment on namespace " + actualDeployment.ObjectMeta.Namespace)
			resourceVersion := actualDeployment.GetResourceVersion()
			desiredDeployment.DeepCopyInto(actualDeployment)
			actualDeployment.SetResourceVersion(resourceVersion)
			if err := remoteClient.Update(ctx, actualDeployment); err != nil {
				logger.Error(err, "Failed to update Medusa deployment resource")
				return result.Error(err)
			}
			return result.RequeueSoon(r.DefaultDelay)
		}

		// Create a service for the medusa standalone deployment
		logger.Info("Reconciling Medusa standalone service on namespace : " + desiredService.ObjectMeta.Namespace)
		serviceKey := utils.GetKey(desiredService)
		if err := controllerutil.SetControllerReference(kc, desiredService, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "CassandraDatacenter", utils.GetKey(kc))
			return result.Error(err)
		}
		logger = logger.WithValues("MedusaService", serviceKey)
		if err := remoteClient.Get(ctx, serviceKey, desiredService); err != nil {
			if errors.IsNotFound(err) {
				if err := remoteClient.Create(ctx, desiredService); err != nil {
					return result.Error(err)
				}
			} else {
				return result.Error(err)
			}
		}
	}

	logger.Info("Medusa standalone deployment successfully reconciled")
	return result.Continue()
}
