package reaper

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultImageRepository = "thelastpickle"
	DefaultImageName       = "cassandra-reaper"
	DefaultVersion         = "3.1.1"
	// When changing the default version above, please also change the kubebuilder markers in
	// apis/reaper/v1alpha1/reaper_types.go accordingly.
)

var defaultImage = images.Image{
	Registry:   images.DefaultRegistry,
	Repository: DefaultImageRepository,
	Name:       DefaultImageName,
	Tag:        DefaultVersion,
}

func NewDeployment(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter, keystorePassword *string, truststorePassword *string, authVars ...*corev1.EnvVar) *appsv1.Deployment {
	labels := createServiceAndDeploymentLabels(reaper)

	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
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
			Value: "cassandra",
		},
		{
			Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
			Value: "false",
		},
		{
			Name:  "REAPER_CASS_CONTACT_POINTS",
			Value: fmt.Sprintf("[%s]", dc.GetDatacenterServiceName()),
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: reaper.Spec.DatacenterAvailability,
		},
		{
			Name:  "REAPER_CASS_LOCAL_DC",
			Value: dc.Name,
		},
		{
			Name:  "REAPER_CASS_KEYSPACE",
			Value: reaper.Spec.Keyspace,
		},
	}

	if reaper.Spec.AutoScheduling.Enabled {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_ENABLED",
			Value: "true",
		})
		adaptive, incremental := getAdaptiveIncremental(reaper, dc)
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

	volumeMounts := []corev1.VolumeMount{}
	volumes := []corev1.Volume{}
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

	initImage := reaper.Spec.InitContainerImage.Merge(defaultImage)
	mainImage := reaper.Spec.ContainerImage.Merge(defaultImage)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name:      reaper.Name,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity:       reaper.Spec.Affinity,
					InitContainers: computeInitContainers(reaper, initImage, envVars, volumeMounts),
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
						},
					},
					ServiceAccountName: reaper.Spec.ServiceAccountName,
					Tolerations:        reaper.Spec.Tolerations,
					SecurityContext:    reaper.Spec.PodSecurityContext,
					ImagePullSecrets:   computeImagePullSecrets(reaper, mainImage, initImage),
					Volumes:            volumes,
				},
			},
		},
	}
	addAuthEnvVars(deployment, authVars)
	annotations.AddHashAnnotation(deployment)
	return deployment
}

func computeInitContainers(reaper *api.Reaper, initImage *images.Image, envVars []corev1.EnvVar, volumeMounts []corev1.VolumeMount) []corev1.Container {
	var initContainers []corev1.Container
	if !reaper.Spec.SkipSchemaMigration {
		initContainers = append(initContainers,
			corev1.Container{
				Name:            "reaper-schema-init",
				Image:           initImage.String(),
				ImagePullPolicy: initImage.PullPolicy,
				SecurityContext: reaper.Spec.InitContainerSecurityContext,
				Env:             envVars,
				Args:            []string{"schema-migration"},
				VolumeMounts:    volumeMounts,
			})
	}
	return initContainers
}

func computeImagePullSecrets(reaper *api.Reaper, mainImage, initImage *images.Image) []corev1.LocalObjectReference {
	if reaper.Spec.SkipSchemaMigration {
		return images.CollectPullSecrets(mainImage)
	} else {
		return images.CollectPullSecrets(mainImage, initImage)
	}
}

func computeProbe(probeTemplate *corev1.Probe) *corev1.Probe {
	var probe *corev1.Probe
	if probeTemplate != nil {
		probe = probeTemplate.DeepCopy()
	} else {
		probe = &corev1.Probe{
			InitialDelaySeconds: 45,
			PeriodSeconds:       15,
		}
	}
	// The handler cannot be user-specified, so force it now
	probe.ProbeHandler = corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/healthcheck",
			Port: intstr.FromInt(8081),
		},
	}
	return probe
}

func addAuthEnvVars(deployment *appsv1.Deployment, vars []*corev1.EnvVar) {
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	for _, v := range vars {
		envVars = append(envVars, *v)
	}
	deployment.Spec.Template.Spec.Containers[0].Env = envVars
	if len(deployment.Spec.Template.Spec.InitContainers) > 0 {
		initEnvVars := deployment.Spec.Template.Spec.InitContainers[0].Env
		for _, v := range vars {
			initEnvVars = append(initEnvVars, *v)
		}
		deployment.Spec.Template.Spec.InitContainers[0].Env = initEnvVars
	}
}

func getAdaptiveIncremental(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter) (adaptive bool, incremental bool) {
	switch reaper.Spec.AutoScheduling.RepairType {
	case "ADAPTIVE":
		adaptive = true
	case "INCREMENTAL":
		incremental = true
	case "AUTO":
		if semver.MustParse(dc.Spec.ServerVersion).Major() == 3 {
			adaptive = true
		} else {
			incremental = true
		}
	}
	return
}
