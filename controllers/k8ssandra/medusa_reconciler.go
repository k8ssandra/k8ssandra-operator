package k8ssandra

import (
	"context"
	"fmt"
	"github.com/adutra/goalesce"
	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cassandra "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	medusa "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Create all things Medusa related in the cassdc podTemplateSpec
func (r *K8ssandraClusterReconciler) reconcileMedusa(
	ctx context.Context,
	desiredKc *api.K8ssandraCluster,
	dcConfig *cassandra.DatacenterConfig,
	remoteClient client.Client,
	logger logr.Logger,
) result.ReconcileResult {
	kc := desiredKc.DeepCopy()
	namespace := utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kc.Namespace)
	logger.Info("Medusa reconcile for " + dcConfig.CassDcName() + " on namespace " + namespace)
	if kc.Spec.Medusa != nil {
		logger.Info("Medusa is enabled")

		mergeResult := r.mergeStorageProperties(ctx, r.Client, namespace, kc.Spec.Medusa, logger, kc)
		medusaSpec := kc.Spec.Medusa
		if mergeResult.IsError() {
			return result.Error(mergeResult.GetError())
		}

		// Check that certificates are provided if client encryption is enabled
		if cassandra.ClientEncryptionEnabled(dcConfig) {
			if kc.Spec.UseExternalSecrets() {
				medusaSpec.CertificatesSecretRef.Name = ""
			} else if medusaSpec.CertificatesSecretRef.Name == "" {
				return result.Error(fmt.Errorf("medusa encryption certificates were not provided despite client encryption being enabled"))
			}
		}

		if medusaSpec.StorageProperties.StorageSecretRef.Name == "" {
			medusaSpec.StorageProperties.StorageSecretRef = corev1.LocalObjectReference{Name: ""}
			if medusaSpec.StorageProperties.CredentialsType == "role-based" && medusaSpec.StorageProperties.StorageProvider == "s3" {
				// It's okay for the secret ref to be unset
				medusaSpec.StorageProperties.StorageSecretRef = corev1.LocalObjectReference{Name: ""}
			} else {
				return result.Error(fmt.Errorf("medusa storage secret is not defined for storage provider %s", medusaSpec.StorageProperties.StorageProvider))
			}
		}
		if res := r.reconcileMedusaConfigMap(ctx, remoteClient, kc, logger, namespace); res.Completed() {
			return res
		}

		medusaContainer, err := medusa.CreateMedusaMainContainer(dcConfig, medusaSpec, kc.Spec.UseExternalSecrets(), kc.SanitizedName(), logger)
		if err != nil {
			return result.Error(err)
		}
		medusa.UpdateMedusaInitContainer(dcConfig, medusaSpec, kc.Spec.UseExternalSecrets(), kc.SanitizedName(), logger)
		medusa.UpdateMedusaMainContainer(dcConfig, medusaContainer)

		// Create required volumes for the Medusa containers
		volumes := medusa.GenerateMedusaVolumes(dcConfig, medusaSpec, kc.SanitizedName())
		for _, volume := range volumes {
			cassandra.AddOrUpdateVolume(dcConfig, volume.Volume, volume.VolumeIndex, volume.Exists)
		}

		// Create the Medusa standalone pod
		desiredMedusaStandalone := medusa.StandaloneMedusaDeployment(*medusaContainer, kc.SanitizedName(), dcConfig.SanitizedName(), namespace, logger)

		// Add the volumes previously computed to the Medusa standalone pod
		for _, volume := range volumes {
			cassandra.AddOrUpdateVolumeToSpec(&desiredMedusaStandalone.Spec.Template, volume.Volume, volume.VolumeIndex, volume.Exists)
		}

		if !kc.Spec.UseExternalSecrets() {
			cassandraUserSecretName := medusa.CassandraUserSecretName(medusaSpec, kc.SanitizedName())
			cassandra.AddCqlUser(medusaSpec.CassandraUserSecretRef, dcConfig, cassandraUserSecretName)

			if dcConfig.Meta.Pods.Annotations == nil {
				dcConfig.Meta.Pods.Annotations = map[string]string{}
			}
			if err := secret.AddInjectionAnnotationMedusaContainers(&dcConfig.Meta.Pods, cassandraUserSecretName); err != nil {
				return result.Error(err)
			}
		}

		// Reconcile the Medusa standalone deployment
		kcKey := utils.GetKey(kc)
		desiredMedusaStandalone.SetLabels(labels.CleanedUpByLabels(kcKey))
		recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *desiredMedusaStandalone)
		switch {
		case recRes.IsError():
			return recRes
		case recRes.IsRequeue():
			return recRes
		}

		// Create and reconcile the Medusa service for the standalone deployment
		medusaService := medusa.StandaloneMedusaService(dcConfig, medusaSpec, kc.SanitizedName(), namespace, logger)
		medusaService.SetLabels(labels.CleanedUpByLabels(kcKey))
		recRes = reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *medusaService)
		switch {
		case recRes.IsError():
			return recRes
		case recRes.IsRequeue():
			return recRes
		}

		// Check if the Medusa Standalone deployment is ready, and requeue if not
		ready, err := r.isMedusaStandaloneReady(ctx, remoteClient, desiredMedusaStandalone)
		if err != nil {
			logger.Info("Failed to check if Medusa standalone deployment is ready", "error", err)
			return result.Error(err)
		}
		if !ready {
			logger.Info("Medusa standalone deployment is not ready yet")
			return result.RequeueSoon(r.DefaultDelay)
		}
		// Create a cron job to purge Medusa backups
		purgeCronJob, err := medusa.PurgeCronJob(dcConfig, kc.SanitizedName(), namespace, logger)
		if err != nil {
			logger.Info("Failed to create Medusa purge backups cronjob", "error", err)
			return result.Error(err)
		}
		purgeCronJob.SetLabels(labels.CleanedUpByLabels(kcKey))
		recRes = reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *purgeCronJob)
		switch {
		case recRes.IsError():
			return recRes
		case recRes.IsRequeue():
			return recRes
		}

	} else {
		logger.Info("Medusa is not enabled")
	}

	return result.Continue()
}

// Check if the Medusa standalone deployment is ready
func (r *K8ssandraClusterReconciler) isMedusaStandaloneReady(ctx context.Context, remoteClient client.Client, desiredMedusaStandalone *appsv1.Deployment) (bool, error) {
	// Get the medusa standalone deployment and check the rollout status
	deplKey := utils.GetKey(desiredMedusaStandalone)
	medusaStandalone := &appsv1.Deployment{}
	if err := remoteClient.Get(context.Background(), deplKey, medusaStandalone); err != nil {
		return false, err
	}
	// Check the conditions to see if the deployment has successfully rolled out
	for _, c := range medusaStandalone.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable {
			return c.Status == corev1.ConditionTrue, nil // deployment is available
		}
	}
	return false, nil // deployment condition not found
}

// Generate a secret for Medusa or use the existing one if provided in the spec
func (r *K8ssandraClusterReconciler) reconcileMedusaSecrets(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) result.ReconcileResult {
	logger.Info("Reconciling Medusa user secrets")
	if kc.Spec.Medusa != nil && !kc.Spec.UseExternalSecrets() {
		cassandraUserSecretRef := kc.Spec.Medusa.CassandraUserSecretRef
		if cassandraUserSecretRef.Name == "" {
			cassandraUserSecretRef.Name = medusa.CassandraUserSecretName(kc.Spec.Medusa, kc.SanitizedName())
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

		if err := r.reconcileBucketSecrets(ctx, r.ClientCache.GetLocalClient(), kc, logger); err != nil {
			logger.Error(err, "Failed to reconcile Medusa bucket secrets")
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
		desiredConfigMap := medusa.CreateMedusaConfigMap(namespace, kc.SanitizedName(), medusaIni)
		kcKey := utils.GetKey(kc)
		desiredConfigMap.SetLabels(labels.CleanedUpByLabels(kcKey))
		recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *desiredConfigMap)
		switch {
		case recRes.IsError():
			return recRes
		case recRes.IsRequeue():
			return recRes
		}
	}
	logger.Info("Medusa ConfigMap successfully reconciled")
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) mergeStorageProperties(
	ctx context.Context,
	remoteClient client.Client,
	namespace string,
	medusaSpec *medusaapi.MedusaClusterTemplate,
	logger logr.Logger,
	desiredKc *api.K8ssandraCluster,
) result.ReconcileResult {
	// check if the StorageProperties are defined in the K8ssandraCluster
	if medusaSpec.MedusaConfigurationRef.Name == "" {
		return result.Continue()
	}
	storageProperties := &medusaapi.MedusaConfiguration{}
	configNamespace := utils.FirstNonEmptyString(medusaSpec.MedusaConfigurationRef.Namespace, namespace)
	configKey := types.NamespacedName{Namespace: configNamespace, Name: medusaSpec.MedusaConfigurationRef.Name}
	if err := remoteClient.Get(ctx, configKey, storageProperties); err != nil {
		logger.Error(err, fmt.Sprintf("failed to get MedusaConfiguration %s", configKey))
		return result.Error(err)
	}
	// check if the StorageProperties from the cluster have the prefix field set
	// it is required to be present because that's the single thing that differentiates backups of two different clusters
	if desiredKc.Spec.Medusa.StorageProperties.Prefix == "" {
		return result.Error(fmt.Errorf("StorageProperties.Prefix is not set in K8ssandraCluster %s", utils.GetKey(desiredKc)))
	}
	// try to merge the storage properties. goalesce gives priority to the 2nd argument,
	// so stuff in the cluster overrides stuff in the config object
	mergedProperties, err := goalesce.DeepMerge(storageProperties.Spec.StorageProperties, desiredKc.Spec.Medusa.StorageProperties)
	if err != nil {
		logger.Error(err, "failed to merge MedusaConfiguration StorageProperties")
		return result.Error(err)
	}
	// medusaapi.MedusaConfiguration comes with a storage corev1.Secret containing the credentials to access the storage
	// we make a copy of that secret for each cluster/dc, and then point to it with a corev1.LocalObjectReference
	// when we do the copy, we name the secret as <cluster-name>-<original-secret-name>
	// here we need to update the reference to point to that copied secret
	mergedProperties.StorageSecretRef.Name = fmt.Sprintf("%s-%s", desiredKc.Name, mergedProperties.StorageSecretRef.Name)

	// copy the merged properties back into the cluster
	mergedProperties.DeepCopyInto(&desiredKc.Spec.Medusa.StorageProperties)
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileBucketSecrets(
	ctx context.Context,
	c client.Client,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) error {

	logger.Info("Reconciling Medusa bucket secrets")
	medusaSpec := kc.Spec.Medusa

	// there is nothing to reconcile if we're not using Medusa configuration reference
	if medusaSpec == nil || medusaSpec.MedusaConfigurationRef.Name == "" {
		logger.Info("MedusaConfigurationRef is not set, skipping bucket secret reconciliation")
		return nil
	}

	// fetch the referenced configuration
	medusaConfigName := medusaSpec.MedusaConfigurationRef.Name
	medusaConfigNamespace := utils.FirstNonEmptyString(medusaSpec.MedusaConfigurationRef.Namespace, kc.Namespace)
	medusaConfigKey := types.NamespacedName{Namespace: medusaConfigNamespace, Name: medusaConfigName}
	medusaConfig := &medusaapi.MedusaConfiguration{}
	if err := c.Get(ctx, medusaConfigKey, medusaConfig); err != nil {
		logger.Error(err, fmt.Sprintf("could not get MedusaConfiguration %s/%s", medusaConfigNamespace, medusaConfigName))
		return err
	}

	// fetch the referenced medusa configuration's bucket secret
	bucketSecretName := medusaConfig.Spec.StorageProperties.StorageSecretRef.Name
	bucketSecret := &corev1.Secret{}
	bucketSecretKey := types.NamespacedName{Namespace: medusaConfigNamespace, Name: bucketSecretName}
	if err := c.Get(ctx, bucketSecretKey, bucketSecret); err != nil {
		logger.Error(err, "could not get bucket Secret")
		return err
	}

	// write the secret into the namespace of the K8ssandraCluster
	clusterBucketSecret := bucketSecret.DeepCopy()
	clusterBucketSecret.ResourceVersion = ""
	clusterBucketSecret.Name = fmt.Sprintf("%s-%s", kc.Name, bucketSecret.Name)
	clusterBucketSecret.Namespace = kc.Namespace
	labels.SetReplicatedBy(clusterBucketSecret, utils.GetKey(kc))
	if err := c.Create(ctx, clusterBucketSecret); err != nil {
		if !errors.IsAlreadyExists(err) {
			logger.Error(err, fmt.Sprintf("failed to create cluster bucket secret %s", clusterBucketSecret))
			return err
		}
		// we already have the bucket secret, so continue to updating the cluster (it might have failed before)
	}

	return nil
}
