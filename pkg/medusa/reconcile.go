package medusa

import (
	"fmt"

	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cassandra "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultMedusaImageRepository = "k8ssandra"
	DefaultMedusaImageName       = "medusa"
	DefaultMedusaVersion         = "0.11.3"
)

var (
	defaultMedusaImageId = images.NewImageId(
		images.DefaultRegistry,
		DefaultMedusaImageRepository,
		DefaultMedusaImageName,
		DefaultMedusaVersion,
	)
	latestMedusaImageId = images.NewImageId(
		images.DefaultRegistry,
		DefaultMedusaImageRepository,
		DefaultMedusaImageName,
		"latest",
	)
)

var (
	defaultMedusaImage = images.NewImage(defaultMedusaImageId, corev1.PullIfNotPresent, nil)
	latestMedusaImage  = images.NewImage(latestMedusaImageId, corev1.PullAlways, nil)
)

func CreateMedusaIni(kc *k8ss.K8ssandraCluster) string {
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
    max_backup_count = %d`, kc.Spec.Medusa.StorageProperties.StorageProvider,
		kc.Spec.Medusa.StorageProperties.BucketName,
		prefix,
		kc.Spec.Medusa.StorageProperties.MaxBackupAge,
		kc.Spec.Medusa.StorageProperties.MaxBackupCount)

	if kc.Spec.Medusa.StorageProperties.Host != "" {
		medusaIni += fmt.Sprintf("\n    host = %s", kc.Spec.Medusa.StorageProperties.Host)
	}

	if kc.Spec.Medusa.StorageProperties.Port != 0 {
		medusaIni += fmt.Sprintf("\n    port = %d", kc.Spec.Medusa.StorageProperties.Port)
	}

	if kc.Spec.Medusa.StorageProperties.Secure {
		medusaIni += "\n    secure = True"
	} else {
		medusaIni += "\n    secure = False"
	}

	if kc.Spec.Medusa.StorageProperties.BackupGracePeriodInDays != 0 {
		medusaIni += fmt.Sprintf("\n    backup_grace_period_in_days = %d", kc.Spec.Medusa.StorageProperties.BackupGracePeriodInDays)
	}

	medusaIni += `

    [grpc]
    enabled = 1

    [kubernetes]
    cassandra_url = http://127.0.0.1:8080/api/v0/ops/node/snapshots
    use_mgmt_api = 1
    enabled = 1

    [logging]
    level = DEBUG`

	return medusaIni
}

func CreateMedusaConfigMap(medusaSpec *medusaapi.MedusaClusterTemplate, namespace, clusterName, medusaIni string) *corev1.ConfigMap {
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
func CassandraUserSecretName(medusaSpec *medusaapi.MedusaClusterTemplate, clusterName string) string {
	if medusaSpec.CassandraUserSecretRef == "" {
		return fmt.Sprintf("%s-medusa", clusterName)
	}
	return medusaSpec.CassandraUserSecretRef
}

func UpdateMedusaInitContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
	restoreContainerIndex, found := cassandra.FindInitContainer(dcConfig.PodTemplateSpec, "medusa-restore")
	restoreContainer := &corev1.Container{Name: "medusa-restore"}
	if found {
		logger.Info("Found medusa-restore init container")
		// medusa-restore init container already exists, we may need to update it
		restoreContainer = dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex].DeepCopy()
	}
	setImage(medusaSpec.ContainerImage, restoreContainer)
	restoreContainer.SecurityContext = medusaSpec.SecurityContext
	restoreContainer.Env = medusaEnvVars(medusaSpec, dcConfig, logger, "RESTORE")
	restoreContainer.VolumeMounts = medusaVolumeMounts(medusaSpec, dcConfig, logger)

	if !found {
		logger.Info("Couldn't find medusa-restore init container")
		// medusa-restore init container doesn't exist, we need to add it
		// We'll add the server-config-init init container in first position so it initializes cassandra.yaml for Medusa to use.
		// The definition of that container will be completed later by cass-operator.
		serverConfigContainer := &corev1.Container{Name: "server-config-init"}
		dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, *serverConfigContainer)
		dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, *restoreContainer)
	} else {
		// Overwrite existing medusa-restore init container
		dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex] = *restoreContainer
	}
}

func UpdateMedusaMainContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
	medusaContainerIndex, found := cassandra.FindContainer(dcConfig.PodTemplateSpec, "medusa")
	medusaContainer := &corev1.Container{Name: "medusa"}
	if found {
		logger.Info("Found medusa container")
		// medusa container already exists, we may need to update it
		medusaContainer = dcConfig.PodTemplateSpec.Spec.Containers[medusaContainerIndex].DeepCopy()
	}
	setImage(medusaSpec.ContainerImage, medusaContainer)
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

// Build the image name and pull policy and add it to a medusa container definition
func setImage(containerImage *medusaapi.ContainerImage, container *corev1.Container) {
	image := computeImage(containerImage)
	container.Image = images.ImageString(image)
	container.ImagePullPolicy = image.GetPullPolicy()
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
						Name: CassandraUserSecretName(medusaSpec, dcConfig.Cluster),
					},
					Key: "username",
				},
			},
		},
		{Name: "CQL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: CassandraUserSecretName(medusaSpec, dcConfig.Cluster),
					},
					Key: "password",
				},
			},
		},
	}
}

// Create or update volumes for medusa
func UpdateMedusaVolumes(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
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

func computeImage(containerImage *api.ContainerImage) images.Image {
	if containerImage == nil {
		return defaultMedusaImage
	} else {
		return images.Coalesce(containerImage, latestMedusaImage)
	}
}
