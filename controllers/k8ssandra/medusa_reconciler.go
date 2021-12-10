package k8ssandra

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cassandra "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Create all things Medusa related in the cassdc podTemplateSpec
func (r *K8ssandraClusterReconciler) ReconcileMedusa(
	ctx context.Context,
	remoteClient client.Client,
	dcConfig *cassandra.DatacenterConfig,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) (ctrl.Result, error) {
	// Medusa isn't enabled (anymore) in this cluster
	logger.Info("Medusa reconcile for " + dcConfig.Meta.Name + " on namespace " + kc.Namespace + "/" + dcConfig.Meta.Namespace)
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
		if result, err := r.reconcileMedusaConfigMap(ctx, remoteClient, kc, logger); !result.IsZero() || err != nil {
			return result, err
		}
		updateMedusaInitContainer(dcConfig, medusaSpec, logger)
		updateMedusaMainContainer(dcConfig, medusaSpec, logger)
		updateMedusaVolumes(dcConfig, medusaSpec, logger)
	} else {
		logger.Info("Medusa is not enabled")
	}

	return ctrl.Result{}, nil
}

// Generate a secret for Medusa or use the existing one if provided in the spec
func (r *K8ssandraClusterReconciler) reconcileMedusaSecrets(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) error {
	logger.Info("Reconciling Medusa user secrets")
	if kc.Spec.Medusa != nil {
		cassandraUserSecretRef := kc.Spec.Medusa.CassandraUserSecretRef
		if cassandraUserSecretRef == "" {
			cassandraUserSecretRef = cassandraUserSecretName(kc.Spec.Medusa, kc.Name)
		}
		logger = logger.WithValues(
			"MedusaCassandraUserSecretRef",
			cassandraUserSecretRef,
		)
		if err := secret.ReconcileSecret(ctx, r.Client, cassandraUserSecretRef, kc.Name, kc.Namespace); err != nil {
			logger.Error(err, "Failed to reconcile Medusa CQL user secret")
			return err
		}
	}
	logger.Info("Medusa user secrets successfully reconciled")
	return nil
}

// Create the Medusa config map if it doesn't exist
func (r *K8ssandraClusterReconciler) reconcileMedusaConfigMap(
	ctx context.Context,
	remoteClient client.Client,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) (ctrl.Result, error) {
	logger.Info("Reconciling Medusa configMap on namespace : " + kc.Namespace)
	if kc.Spec.Medusa != nil {
		medusaIni := createMedusaIni(kc)
		desiredConfigMap := createMedusaConfigMap(kc.Spec.Medusa, kc.Namespace, kc.Spec.Cassandra.Cluster, medusaIni)
		// Compute a hash which will allow to compare desired and actual configMaps
		utils.AddHashAnnotation(desiredConfigMap, api.ResourceHashAnnotation)
		actualConfigMap := &corev1.ConfigMap{}

		configMapKey := client.ObjectKey{
			Namespace: kc.Namespace,
			Name:      fmt.Sprintf("%s-medusa", kc.Spec.Cassandra.Cluster),
		}

		if err := remoteClient.Get(ctx, configMapKey, actualConfigMap); err != nil {
			if errors.IsNotFound(err) {
				if err := remoteClient.Create(ctx, desiredConfigMap); err != nil {
					logger.Error(err, "Failed to create Medusa ConfigMap")
					return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
				}
			}
		}

		actualConfigMap = actualConfigMap.DeepCopy()

		if !utils.CompareAnnotations(actualConfigMap, desiredConfigMap, api.ResourceHashAnnotation) {
			logger.Info("Updating Medusa ConfigMap")
			resourceVersion := actualConfigMap.GetResourceVersion()
			desiredConfigMap.DeepCopyInto(actualConfigMap)
			actualConfigMap.SetResourceVersion(resourceVersion)
			logger.Info("Updating configMap on namespace " + actualConfigMap.ObjectMeta.Namespace)
			logger.Info("Should be on namespace " + actualConfigMap.ObjectMeta.Namespace)
			if err := remoteClient.Update(ctx, actualConfigMap); err != nil {
				logger.Error(err, "Failed to update Medusa ConfigMap resource")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	}
	logger.Info("Medusa ConfigMap successfully reconciled")
	return ctrl.Result{}, nil
}

func createMedusaIni(kc *api.K8ssandraCluster) string {
	prefix := kc.Spec.Medusa.StorageProperties.Prefix
	if prefix == "" {
		prefix = kc.ClusterName
	}

	medusaIni := fmt.Sprintf(`
    [cassandra]
    # The start and stop commands are not applicable in k8s.
    stop_cmd = /etc/init.d/cassandra stop
    start_cmd = /etc/init.d/cassandra start
	
    check_running = nodetool version

    [storage]
    storage_provider = %s
	bucket_name = %s
  	key_file = /etc/medusa-secrets/credentials
    prefix = %s
	max_backup_age = %d
	max_backup_count = %d
	`, kc.Spec.Medusa.StorageProperties.StorageProvider,
		kc.Spec.Medusa.StorageProperties.BucketName,
		prefix,
		kc.Spec.Medusa.StorageProperties.MaxBackupAge,
		kc.Spec.Medusa.StorageProperties.MaxBackupCount)

	if kc.Spec.Medusa.StorageProperties.Host != "" {
		medusaIni += fmt.Sprintf("host = %s\n", kc.Spec.Medusa.StorageProperties.Host)
	}

	if kc.Spec.Medusa.StorageProperties.Port != 0 {
		medusaIni += fmt.Sprintf("port = %d\n", kc.Spec.Medusa.StorageProperties.Port)
	}

	if kc.Spec.Medusa.StorageProperties.Secure {
		medusaIni += "secure = True\n"
	} else {
		medusaIni += "secure = False\n"
	}

	if kc.Spec.Medusa.StorageProperties.BackupGracePeriodInDays != 0 {
		medusaIni += fmt.Sprintf("backup_grace_period_in_days = %d\n", kc.Spec.Medusa.StorageProperties.BackupGracePeriodInDays)
	}

	medusaIni += `[grpc]
    enabled = 1

    [kubernetes]
    cassandra_url = http://127.0.0.1:8080/api/v0/ops/node/snapshots
    use_mgmt_api = 1
    enabled = 1

    [logging]
    level = DEBUG`

	return medusaIni
}

func createMedusaConfigMap(medusaSpec *medusaapi.MedusaClusterTemplate, namespace, clusterName, medusaIni string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-medusa", clusterName),
			Namespace: namespace,
		},
		Data: map[string]string{
			"medusa.ini": medusaIni,
		},
	}
}

// Get the cassandra user secret name from the medusa spec or generate one based on the cluster name
func cassandraUserSecretName(medusaSpec *medusaapi.MedusaClusterTemplate, clusterName string) string {
	if medusaSpec.CassandraUserSecretRef == "" {
		return fmt.Sprintf("%s-medusa", clusterName)
	}
	return medusaSpec.CassandraUserSecretRef
}

func updateMedusaInitContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
	restoreContainerIndex, found := cassandra.FindInitContainer(dcConfig.PodTemplateSpec, "medusa-restore")
	restoreContainer := &corev1.Container{Name: "medusa-restore"}
	if found {
		logger.Info("Found medusa-restore init container")
		// medusa-restore init container already exists, we may need to update it
		restoreContainer = dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex].DeepCopy()
	}
	restoreContainer.Image = fmt.Sprintf("%s/%s:%s", *medusaSpec.Image.Registry, medusaSpec.Image.Repository, *medusaSpec.Image.Tag)
	restoreContainer.ImagePullPolicy = *medusaSpec.Image.PullPolicy
	restoreContainer.SecurityContext = medusaSpec.SecurityContext
	restoreContainer.Env = medusaEnvVars(medusaSpec, dcConfig, logger, "RESTORE")
	restoreContainer.VolumeMounts = medusaVolumeMounts(medusaSpec, dcConfig, logger)

	if !found {
		logger.Info("Couldn't find medusa-restore init container")
		// medusa-restore init container doesn't exist, we need to add it
		dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, *restoreContainer)
	} else {
		// Overwrite existing medusa-restore init container
		dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex] = *restoreContainer
	}
}

func updateMedusaMainContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
	medusaContainerIndex, found := cassandra.FindContainer(dcConfig.PodTemplateSpec, "medusa")
	medusaContainer := &corev1.Container{Name: "medusa"}
	if found {
		logger.Info("Found medusa container")
		// medusa container already exists, we may need to update it
		medusaContainer = dcConfig.PodTemplateSpec.Spec.Containers[medusaContainerIndex].DeepCopy()
	}
	medusaContainer.Image = fmt.Sprintf("%s/%s:%s", *medusaSpec.Image.Registry, medusaSpec.Image.Repository, *medusaSpec.Image.Tag)
	medusaContainer.ImagePullPolicy = *medusaSpec.Image.PullPolicy
	medusaContainer.SecurityContext = medusaSpec.SecurityContext
	medusaContainer.Env = medusaEnvVars(medusaSpec, dcConfig, logger, "GRPC")

	medusaContainer.VolumeMounts = medusaVolumeMounts(medusaSpec, dcConfig, logger)

	if !found {
		logger.Info("Couldn't find medusa container")
		// medusa container doesn't exist, we need to add it
		dcConfig.PodTemplateSpec.Spec.Containers = append(dcConfig.PodTemplateSpec.Spec.Containers, *medusaContainer)
	} else {
		// Overwrite existing medusa container
		dcConfig.PodTemplateSpec.Spec.Containers[medusaContainerIndex] = *medusaContainer
	}
}

func medusaVolumeMounts(medusaSpec *medusaapi.MedusaClusterTemplate, dcConfig *cassandra.DatacenterConfig, logger logr.Logger) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{ // Cassandra config volume
			Name:      "server-config",
			MountPath: "/etc/cassandra",
		},
		{ // Cassandra data volume
			Name:      "server-data",
			MountPath: "/var/lib/cassandra",
		},
		{ // Medusa config volume
			Name:      fmt.Sprintf("%s-medusa", dcConfig.Cluster),
			MountPath: "/etc/medusa",
		},
		{ // Pod info volume
			Name:      "podinfo",
			MountPath: "/etc/podinfo",
		},
	}

	if medusaSpec.StorageProperties.StorageProvider == "local" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			// Medusa local backup storage volume
			Name:      "medusa-backups",
			MountPath: "/mnt/backups",
		})
	} else {
		// We're not using local storage for backups, which requires a secret with backend credentials
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			// Medusa storage secret volume
			Name:      medusaSpec.StorageProperties.StorageSecretRef,
			MountPath: "/etc/medusa-secrets",
		})
	}

	return volumeMounts
}

func medusaEnvVars(medusaSpec *medusaapi.MedusaClusterTemplate, dcConfig *cassandra.DatacenterConfig, logger logr.Logger, mode string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "MEDUSA_MODE",
			Value: mode,
		},
		{Name: "CQL_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cassandraUserSecretName(medusaSpec, dcConfig.Cluster),
					},
					Key: "username",
				},
			},
		},
		{Name: "CQL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cassandraUserSecretName(medusaSpec, dcConfig.Cluster),
					},
					Key: "password",
				},
			},
		},
	}
}

// Create or update volumes for medusa
func updateMedusaVolumes(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
	// Medusa config volume, containing medusa.ini
	configVolumeIndex, found := cassandra.FindVolume(dcConfig.PodTemplateSpec, fmt.Sprintf("%s-medusa", dcConfig.Cluster))
	configVolume := &corev1.Volume{
		Name: fmt.Sprintf("%s-medusa", dcConfig.Cluster),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("%s-medusa", dcConfig.Cluster),
				},
			},
		},
	}

	addOrUpdateVolume(dcConfig, configVolume, configVolumeIndex, found)

	// Medusa credentials volume using the referenced secret
	if medusaSpec.StorageProperties.StorageProvider != "local" {
		// We're not using local storage for backups, which requires a secret with backend credentials
		secretVolumeIndex, found := cassandra.FindVolume(dcConfig.PodTemplateSpec, medusaSpec.StorageProperties.StorageSecretRef)
		secretVolume := &corev1.Volume{
			Name: medusaSpec.StorageProperties.StorageSecretRef,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: medusaSpec.StorageProperties.StorageSecretRef,
				},
			},
		}

		addOrUpdateVolume(dcConfig, secretVolume, secretVolumeIndex, found)
	}

	// Pod info volume
	podInfoVolumeIndex, found := cassandra.FindVolume(dcConfig.PodTemplateSpec, "podinfo")
	podInfoVolume := &corev1.Volume{
		Name: "podinfo",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: "labels",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.labels",
						},
					},
				},
			},
		},
	}

	addOrUpdateVolume(dcConfig, podInfoVolume, podInfoVolumeIndex, found)
}

func addOrUpdateVolume(dcConfig *cassandra.DatacenterConfig, volume *corev1.Volume, volumeIndex int, found bool) {
	if !found {
		// volume doesn't exist, we need to add it
		dcConfig.PodTemplateSpec.Spec.Volumes = append(dcConfig.PodTemplateSpec.Spec.Volumes, *volume)
	} else {
		// Overwrite existing volume
		dcConfig.PodTemplateSpec.Spec.Volumes[volumeIndex] = *volume
	}
}
