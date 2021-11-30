package k8ssandra

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cassandra "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	corev1 "k8s.io/api/core/v1"
)

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

// Get the cassandra user secret name from the medusa spec or generate one based on the cluster name
func cassandraUserSecretName(medusaSpec *medusaapi.MedusaClusterTemplate, clusterName string) string {
	if medusaSpec.CassandraUserSecretRef == "" {
		return fmt.Sprintf("%s-medusa", clusterName)
	}
	return medusaSpec.CassandraUserSecretRef
}

// Create all things Medusa related in the cassdc podTemplateSpec
func AddMedusa(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) error {
	// Medusa isn't enabled (anymore) in this cluster
	if medusaSpec != nil {
		logger.Info("Medusa is enabled")
		updateMedusaInitContainer(dcConfig, medusaSpec, logger)
		updateMedusaMainContainer(dcConfig, medusaSpec, logger)
		updateMedusaVolumes(dcConfig, medusaSpec, logger)
	} else {
		logger.Info("Medusa is not enabled")
	}

	return nil
}

func updateMedusaInitContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
	restoreContainerIndex, found := findMedusaContainer(dcConfig.PodTemplateSpec, "medusa-restore", true)
	restoreContainer := &corev1.Container{Name: "medusa-restore"}
	if found {
		logger.Info("Found medusa-restore init container")
		// medusa-restore init container already exists, we may need to update it
		restoreContainer = dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex].DeepCopy()
	}
	restoreContainer.Image = fmt.Sprintf("%s/%s:%s", *medusaSpec.Image.Registry, medusaSpec.Image.Repository, *medusaSpec.Image.Tag)
	restoreContainer.ImagePullPolicy = *medusaSpec.Image.PullPolicy
	restoreContainer.SecurityContext = medusaSpec.SecurityContext
	restoreContainer.Env = []corev1.EnvVar{
		{
			Name:  "MEDUSA_MODE",
			Value: "RESTORE",
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
	restoreContainer.VolumeMounts = []corev1.VolumeMount{
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
		restoreContainer.VolumeMounts = append(restoreContainer.VolumeMounts, corev1.VolumeMount{
			// Medusa local backup storage volume
			Name:      "medusa-backups",
			MountPath: "/mnt/backups",
		})
	} else {
		// We're not using local storage for backups, which requires a secret with backend credentials
		restoreContainer.VolumeMounts = append(restoreContainer.VolumeMounts, corev1.VolumeMount{
			// Medusa storage secret volume
			Name:      medusaSpec.StorageProperties.StorageSecretRef,
			MountPath: "/etc/medusa-secrets",
		})
	}

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
}

// Create or update volumes for medusa
func updateMedusaVolumes(dcConfig *cassandra.DatacenterConfig, medusaSpec *medusaapi.MedusaClusterTemplate, logger logr.Logger) {
	// Medusa config volume, containing medusa.ini
	configVolumeIndex, found := findMedusaVolume(dcConfig.PodTemplateSpec, fmt.Sprintf("%s-medusa", dcConfig.Cluster))
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
		secretVolumeIndex, found := findMedusaVolume(dcConfig.PodTemplateSpec, medusaSpec.StorageProperties.StorageSecretRef)
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
	podInfoVolumeIndex, found := findMedusaVolume(dcConfig.PodTemplateSpec, "podinfo")
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

func findMedusaContainer(dcPodTemplateSpec *corev1.PodTemplateSpec, containerName string, initContainer bool) (int, bool) {
	if dcPodTemplateSpec.Spec.InitContainers == nil {
		dcPodTemplateSpec.Spec.InitContainers = []corev1.Container{}
	}
	if initContainer {
		for i, container := range dcPodTemplateSpec.Spec.InitContainers {
			if container.Name == containerName {
				return i, true
			}
		}
	} else {
		for i, container := range dcPodTemplateSpec.Spec.Containers {
			if container.Name == containerName {
				return i, true
			}
		}
	}

	return -1, false
}

func findMedusaVolume(dcPodTemplateSpec *corev1.PodTemplateSpec, volumeName string) (int, bool) {
	for i, volume := range dcPodTemplateSpec.Spec.Volumes {
		if volume.Name == volumeName {
			return i, true
		}
	}

	return -1, false
}
