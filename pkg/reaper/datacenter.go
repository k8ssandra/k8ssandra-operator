package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	corev1 "k8s.io/api/core/v1"
)

func AddReaperSettingsToDcConfig(reaperTemplate *reaperapi.ReaperClusterTemplate, dcConfig *cassandra.DatacenterConfig) {
	addUser(reaperTemplate, dcConfig)
	if dcConfig.PodTemplateSpec == nil {
		dcConfig.PodTemplateSpec = &corev1.PodTemplateSpec{}
	}
	addInitContainer(reaperTemplate, dcConfig)
	modifyMainContainer(dcConfig)
}

func addUser(reaperTemplate *reaperapi.ReaperClusterTemplate, dcConfig *cassandra.DatacenterConfig) {
	cassandraUserSecretRef := reaperTemplate.CassandraUserSecretRef
	if cassandraUserSecretRef == "" {
		cassandraUserSecretRef = DefaultUserSecretName(dcConfig.Cluster)
	}
	dcConfig.Users = append(dcConfig.Users, cassdcapi.CassandraUser{
		SecretName: cassandraUserSecretRef,
		Superuser:  true,
	})
}

func addInitContainer(reaperTemplate *reaperapi.ReaperClusterTemplate, dcConfig *cassandra.DatacenterConfig) {
	jmxUserSecretRef := reaperTemplate.JmxUserSecretRef
	if jmxUserSecretRef == "" {
		jmxUserSecretRef = DefaultJmxUserSecretName(dcConfig.Cluster)
	}
	dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, corev1.Container{
		Name:            "jmx-credentials",
		Image:           "docker.io/busybox:1.33.1",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name: "REAPER_JMX_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: jmxUserSecretRef},
						Key:                  "username",
					},
				},
			},
			{
				Name: "REAPER_JMX_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: jmxUserSecretRef},
						Key:                  "password",
					},
				},
			},
		},
		Args: []string{
			"/bin/sh",
			"-c",
			"echo \"$REAPER_JMX_USERNAME $REAPER_JMX_PASSWORD\" > /config/jmxremote.password",
		},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "server-config",
			MountPath: "/config",
		}},
	})
}

func modifyMainContainer(dcConfig *cassandra.DatacenterConfig) {
	for i, container := range dcConfig.PodTemplateSpec.Spec.Containers {
		if container.Name == reconciliation.CassandraContainerName {
			container.Env = append(container.Env, corev1.EnvVar{Name: "LOCAL_JMX", Value: "no"})
			dcConfig.PodTemplateSpec.Spec.Containers[i] = container
			return
		}
	}
	dcConfig.PodTemplateSpec.Spec.Containers = append(dcConfig.PodTemplateSpec.Spec.Containers, corev1.Container{
		Name: reconciliation.CassandraContainerName,
		Env:  []corev1.EnvVar{{Name: "LOCAL_JMX", Value: "no"}},
	})
}
