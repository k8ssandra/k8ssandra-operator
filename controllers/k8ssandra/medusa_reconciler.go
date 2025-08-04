package k8ssandra

import (
	"context"
	"fmt"
	"os"

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
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	operatorNamespaceEnvVar = "OPERATOR_NAMESPACE"
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
	dcNamespace := utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kc.Namespace)
	logger.Info("Medusa reconcile for " + dcConfig.CassDcName() + " on namespace " + dcNamespace)
	if kc.Spec.Medusa != nil {
		logger.Info("Medusa is enabled")

		mergeResult := r.mergeStorageProperties(ctx, r.Client, kc.Spec.Medusa, logger, kc)
		medusaSpec := kc.Spec.Medusa
		if mergeResult.IsError() {
			return result.Error(mergeResult.GetError())
		}

		// Check that certificates are provided if client encryption is enabled
		if cassandra.ClientEncryptionEnabled(dcConfig) {
			// check if we need to worry about client encryption stores ourselves
			if kc.Spec.UseExternalSecrets() {
				medusaSpec.CertificatesSecretRef.Name = ""
			} else if medusaSpec.CertificatesSecretRef.Name == "" && medusaSpec.ClientEncryptionStores == nil {
				return result.Error(fmt.Errorf("medusa encryption certificates were not provided despite client encryption being enabled"))
			}
			// possibly issue warnings about using the deprecated way of setting medusa's client certs
			if medusaSpec.CertificatesSecretRef.Name != "" {
				logger.Info("medusa.Spec.CertificatesSecretRef has been deprecated, please use medusa.Spec.ClientEncryptionStores instead")
			}
			if medusaSpec.CertificatesSecretRef.Name != "" && medusaSpec.ClientEncryptionStores != nil {
				logger.Info("medusa has both certificatesSecretRef and clientEncryptionStores set, will still use the certificatesSecretRef for backwards compatibility")
			}
		}

		if err := r.validateStorageCredentials(medusaSpec); err != nil {
			return result.Error(err)
		}

		if res := r.reconcileMedusaConfigMap(ctx, remoteClient, kc, dcConfig, logger, dcNamespace); res.Completed() {
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

		logger.Info("Checking if Medusa backups should be purged, PurgeBackups: " + fmt.Sprintf("%t", *medusaSpec.PurgeBackups))
		operatorNamespace := r.getOperatorNamespace()
		// Always cleanup the purge cronjob if it exists, we no longer use it.
		err = r.cleanupPurgeCronJob(ctx, dcConfig, kc, operatorNamespace, logger, remoteClient)
		if err != nil {
			return result.Error(err)
		}

		// Create the MedusaBackupSchedule
		// Default to true if not set
		if medusaSpec.PurgeBackups == nil || *medusaSpec.PurgeBackups {
			logger.Info("Creating Medusa purge schedule")
			medusaBackupSchedule := r.createPurgeSchedule(ctx, remoteClient, kc, dcConfig, *r.Scheme, logger)
			if recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *medusaBackupSchedule); recRes.Completed() {
				return recRes
			}
		} else {
			logger.Info("Deleting Medusa purge schedule if it exists")
			err := r.maybeCleanupPurgeSchedule(ctx, dcConfig, kc, operatorNamespace, logger, remoteClient)
			if err != nil {
				return result.Error(err)
			}
		}
	} else {
		logger.Info("Medusa is not enabled")
	}

	return result.Continue()
}

// This function is used to cleanup the purge cronjob if it exists.
// We no longer use CronJobs to schedule purges, but now rely on the MedusaBackupSchedule API.
func (*K8ssandraClusterReconciler) cleanupPurgeCronJob(ctx context.Context, dcConfig *cassandra.DatacenterConfig, kc *api.K8ssandraCluster, operatorNamespace string, logger logr.Logger, remoteClient client.Client) error {
	cronJobName := medusa.MedusaPurgeScheduleName(kc.SanitizedName(), dcConfig.SanitizedName())
	cronJobKey := types.NamespacedName{Namespace: operatorNamespace, Name: cronJobName}
	cronJob := &batchv1.CronJob{}
	if err := remoteClient.Get(ctx, cronJobKey, cronJob); err != nil {
		// If the error is anything else but not found, fail the reconcile
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get Medusa purge backups cronjob")
			return err
		}
	} else {
		// The cron job exists, delete it
		logger.Info("Deleting Medusa purge backups cronjob (may have been created before PurgeBackups was set to false")
		if err := remoteClient.Delete(ctx, cronJob); err != nil {
			logger.Info("Failed to delete Medusa purge backups cronjob", "error", err)
			return err
		}
	}
	return nil
}

func (*K8ssandraClusterReconciler) maybeCleanupPurgeSchedule(ctx context.Context, dcConfig *cassandra.DatacenterConfig, kc *api.K8ssandraCluster, operatorNamespace string, logger logr.Logger, remoteClient client.Client) error {
	purgeScheduleName := medusa.MedusaPurgeScheduleName(kc.SanitizedName(), dcConfig.SanitizedName())
	dcNamespace := dcConfig.Meta.Namespace
	if dcNamespace == "" {
		dcNamespace = kc.Namespace
	}
	purgeScheduleKey := types.NamespacedName{Namespace: dcNamespace, Name: purgeScheduleName}
	purgeSchedule := &medusaapi.MedusaBackupSchedule{}
	if err := remoteClient.Get(ctx, purgeScheduleKey, purgeSchedule); err != nil {
		// If the error is anything else but not found, fail the reconcile
		if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get Medusa purge backup schedule")
			return err
		}
	} else {
		// The purge schedule exists, delete it
		logger.Info("Deleting Medusa purge backup schedule")
		if err := remoteClient.Delete(ctx, purgeSchedule); err != nil {
			logger.Info("Failed to delete Medusa purge backup schedule", "error", err)
			return err
		}
	}
	return nil
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
		if err := secret.ReconcileSecret(ctx, r.Client, cassandraUserSecretRef.Name, kc); err != nil {
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
	dcConfig *cassandra.DatacenterConfig,
	logger logr.Logger,
	namespace string,
) result.ReconcileResult {
	logger.Info("Reconciling Medusa configMap on namespace : " + namespace)
	if kc.Spec.Medusa != nil {
		medusaIni := medusa.CreateMedusaIni(kc, dcConfig)
		desiredConfigMap := medusa.CreateMedusaConfigMap(namespace, kc.SanitizedName(), medusaIni)
		kcKey := utils.GetKey(kc)
		desiredConfigMap.SetLabels(labels.CleanedUpByLabels(kcKey))
		if recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *desiredConfigMap); recRes.Completed() {
			return recRes
		}
	}
	logger.Info("Medusa ConfigMap successfully reconciled")
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) mergeStorageProperties(
	ctx context.Context,
	remoteClient client.Client,
	medusaSpec *medusaapi.MedusaClusterTemplate,
	logger logr.Logger,
	desiredKc *api.K8ssandraCluster,
) result.ReconcileResult {
	// check if the StorageProperties are defined in the K8ssandraCluster
	if medusaSpec.MedusaConfigurationRef.Name == "" {
		return result.Continue()
	}
	storageProperties := &medusaapi.MedusaConfiguration{}
	configKey := types.NamespacedName{Namespace: desiredKc.Namespace, Name: medusaSpec.MedusaConfigurationRef.Name}

	if err := remoteClient.Get(ctx, configKey, storageProperties); err != nil {
		logger.Error(err, "failed to get MedusaConfiguration", "MedusaConfigKey", configKey, "K8ssandraCluster", desiredKc)
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

	// copy the merged properties back into the cluster
	mergedProperties.DeepCopyInto(&desiredKc.Spec.Medusa.StorageProperties)
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) getOperatorNamespace() string {
	operatorNamespace, found := os.LookupEnv(operatorNamespaceEnvVar)
	if !found {
		return "default"
	}
	return operatorNamespace
}

func (r *K8ssandraClusterReconciler) validateStorageCredentials(medusaSpec *medusaapi.MedusaClusterTemplate) error {

	// we must specify either storage secret or role-based credentials
	if medusaSpec.StorageProperties.StorageSecretRef.Name == "" && medusaSpec.StorageProperties.CredentialsType != medusa.CredentialsTypeRoleBased {
		return fmt.Errorf("must specify either a storge secret or use role-based credentials")
	}

	// if a storage secret is set, we error if role-based credentials are set too
	if medusaSpec.StorageProperties.StorageSecretRef.Name != "" {
		if medusaSpec.StorageProperties.CredentialsType == medusa.CredentialsTypeRoleBased {
			return fmt.Errorf("cannot specify both a storage secret and role-based credentials: %s", medusaSpec.StorageProperties.StorageSecretRef.Name)
		}
	}

	return nil
}

func (r *K8ssandraClusterReconciler) createPurgeSchedule(ctx context.Context, remoteClient client.Client, kc *api.K8ssandraCluster, dcConfig *cassandra.DatacenterConfig, scheme runtime.Scheme, logger logr.Logger) *medusaapi.MedusaBackupSchedule {
	logger.Info("Creating Medusa purge schedule", "K8ssandraCluster", kc.Name, "CassandraDatacenter", dcConfig.SanitizedName())
	dcNamespace := dcConfig.Meta.Namespace
	if dcNamespace == "" {
		dcNamespace = kc.Namespace
	}
	purgeSchedule := medusaapi.MedusaBackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      medusa.MedusaPurgeScheduleName(kc.SanitizedName(), dcConfig.SanitizedName()),
			Namespace: dcNamespace,
		},
		Spec: medusaapi.MedusaBackupScheduleSpec{
			BackupSpec: medusaapi.MedusaBackupJobSpec{
				CassandraDatacenter: dcConfig.SanitizedName(),
			},
			CronSchedule:  "0 0 * * *",
			OperationType: string(medusaapi.OperationTypePurge),
		},
	}
	purgeSchedule.SetLabels(labels.CleanedUpByLabels(utils.GetKey(kc)))
	err := controllerutil.SetOwnerReference(kc, &purgeSchedule, &scheme)
	if err != nil {
		logger.Error(err, "failed to set owner reference for MedusaBackupSchedule")
		return nil
	}
	if recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, purgeSchedule); recRes.Completed() {
		return &purgeSchedule
	}
	return &purgeSchedule
}
