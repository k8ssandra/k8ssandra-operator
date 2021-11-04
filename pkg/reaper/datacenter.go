package reaper

import (
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	corev1 "k8s.io/api/core/v1"
)

func AddReaperSettingsToDcPodTemplate(template *corev1.PodTemplateSpec, jmxUserSecretRef string) *corev1.PodTemplateSpec {
	template.Spec.InitContainers = append(template.Spec.InitContainers, corev1.Container{
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
	template.Spec.Containers = append(template.Spec.Containers, corev1.Container{
		Name: reconciliation.CassandraContainerName,
		Env:  []corev1.EnvVar{{Name: "LOCAL_JMX", Value: "no"}},
	})
	return template
}
