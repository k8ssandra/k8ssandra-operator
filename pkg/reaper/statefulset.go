package reaper

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewStatefulSet(reaper *api.Reaper, keystorePassword *string, truststorePassword *string, logger logr.Logger, authVars ...*corev1.EnvVar) *appsv1.StatefulSet {
	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			// Note: managed-by shouldn't be used here, but we're keeping it for backwards compatibility, since changing
			// a deployment's selector is a breaking change.
			{
				Key:      v1alpha1.ManagedByLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{v1alpha1.NameLabelValue},
			},
			{
				Key:      api.ReaperLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{reaper.Name},
			},
		},
	}

	readinessProbe := computeProbe(reaper.Spec.ReadinessProbe)
	livenessProbe := computeProbe(reaper.Spec.LivenessProbe)

	envVars := []corev1.EnvVar{
		{
			Name:  "REAPER_STORAGE_TYPE",
			Value: "memory",
		},
		{
			Name:  "REAPER_MEMORY_STORAGE_DIRECTORY",
			Value: "/var/lib/cassandra-reaper/storage",
		},
		{
			Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
			Value: "false",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: "ALL",
		},
	}

	if reaper.Spec.AutoScheduling.Enabled {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_ENABLED",
			Value: "true",
		})
		adaptive, incremental := getAdaptiveIncremental(reaper, nil)
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_ADAPTIVE",
			Value: fmt.Sprintf("%v", adaptive),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_INCREMENTAL",
			Value: fmt.Sprintf("%v", incremental),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_PERCENT_UNREPAIRED_THRESHOLD",
			Value: fmt.Sprintf("%v", reaper.Spec.AutoScheduling.PercentUnrepairedThreshold),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_INITIAL_DELAY_PERIOD",
			Value: reaper.Spec.AutoScheduling.InitialDelay,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_PERIOD_BETWEEN_POLLS",
			Value: reaper.Spec.AutoScheduling.PeriodBetweenPolls,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_TIME_BEFORE_FIRST_SCHEDULE",
			Value: reaper.Spec.AutoScheduling.TimeBeforeFirstSchedule,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_SCHEDULE_SPREAD_PERIOD",
			Value: reaper.Spec.AutoScheduling.ScheduleSpreadPeriod,
		})
		if reaper.Spec.AutoScheduling.ExcludedClusters != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_CLUSTERS",
				Value: fmt.Sprintf("[%s]", strings.Join(reaper.Spec.AutoScheduling.ExcludedClusters, ", ")),
			})
		}
		if reaper.Spec.AutoScheduling.ExcludedKeyspaces != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_KEYSPACES",
				Value: fmt.Sprintf("[%s]", strings.Join(reaper.Spec.AutoScheduling.ExcludedKeyspaces, ", ")),
			})
		}
	}

	if reaper.Spec.SkipSchemaMigration {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_SKIP_SCHEMA_MIGRATION",
			Value: "true",
		})
	}

	if reaper.Spec.HeapSize != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_HEAP_SIZE",
			Value: fmt.Sprintf("%d", reaper.Spec.HeapSize.Value()),
		})
	}
	if reaper.Spec.HttpManagement.Enabled {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_HTTP_MANAGEMENT_ENABLE",
			Value: "true",
		})

		if reaper.Spec.HttpManagement.Keystores != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_HTTP_MANAGEMENT_KEYSTORE_PATH",
				Value: "/etc/encryption/mgmt/keystore.jks",
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_HTTP_MANAGEMENT_TRUSTSTORE_PATH",
				Value: "/etc/encryption/mgmt/truststore.jks",
			})
		}
	}

	volumeMounts := []corev1.VolumeMount{}
	volumes := []corev1.Volume{}
	// TODO add Reaper's memory storage volume

	// if client encryption is turned on, we need to mount the keystore and truststore volumes
	if reaper.Spec.ClientEncryptionStores != nil && keystorePassword != nil && truststorePassword != nil {
		keystoreVolume, truststoreVolume := cassandra.EncryptionVolumes(encryption.StoreTypeClient, *reaper.Spec.ClientEncryptionStores)
		volumes = append(volumes, *keystoreVolume)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      keystoreVolume.Name,
			MountPath: cassandra.StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameKeystore),
		})
		volumes = append(volumes, *truststoreVolume)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      truststoreVolume.Name,
			MountPath: cassandra.StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameTruststore),
		})

		javaOpts := fmt.Sprintf("-Djavax.net.ssl.keyStore=/mnt/client-keystore/keystore -Djavax.net.ssl.keyStorePassword=%s -Djavax.net.ssl.trustStore=/mnt/client-truststore/truststore -Djavax.net.ssl.trustStorePassword=%s -Dssl.enable=true", *keystorePassword, *truststorePassword)
		envVars = append(envVars, corev1.EnvVar{
			Name:  "JAVA_OPTS",
			Value: javaOpts,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_CASS_NATIVE_PROTOCOL_SSL_ENCRYPTION_ENABLED",
			Value: "true",
		})
	}

	if reaper.Spec.HttpManagement.Enabled && reaper.Spec.HttpManagement.Keystores != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "management-api-keystore",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: reaper.Spec.HttpManagement.Keystores.Name,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "management-api-keystore",
			MountPath: "/etc/encryption/mgmt",
		})
	}

	mainImage := reaper.Spec.ContainerImage.ApplyDefaults(defaultImage)

	initContainerResources := computeInitContainerResources(reaper.Spec.InitContainerResources)
	mainContainerResources := computeMainContainerResources(reaper.Spec.Resources)

	podMeta := getPodMeta(reaper)

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   reaper.Namespace,
			Name:        reaper.Name,
			Labels:      createServiceAndDeploymentLabels(reaper),
			Annotations: map[string]string{},
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podMeta.Labels,
					Annotations: podMeta.Annotations,
				},
				Spec: corev1.PodSpec{
					Affinity:       reaper.Spec.Affinity,
					InitContainers: computeInitContainers(reaper, mainImage, envVars, volumeMounts, initContainerResources),
					Containers: []corev1.Container{
						{
							Name:            "reaper",
							Image:           mainImage.String(),
							ImagePullPolicy: mainImage.PullPolicy,
							SecurityContext: reaper.Spec.SecurityContext,
							Ports: []corev1.ContainerPort{
								{
									Name:          "app",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
								{
									Name:          "admin",
									ContainerPort: 8081,
									Protocol:      "TCP",
								},
							},
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
							Env:            envVars,
							VolumeMounts:   volumeMounts,
							Resources:      *mainContainerResources,
						},
					},
					ServiceAccountName: reaper.Spec.ServiceAccountName,
					Tolerations:        reaper.Spec.Tolerations,
					SecurityContext:    reaper.Spec.PodSecurityContext,
					ImagePullSecrets:   computeImagePullSecrets(reaper, mainImage),
					Volumes:            volumes,
				},
			},
		},
	}
	addAuthEnvVarsToStatefulSet(statefulSet, authVars)
	configVectorForStatefulSet(reaper, statefulSet, logger)
	annotations.AddHashAnnotation(statefulSet)
	return statefulSet
}
