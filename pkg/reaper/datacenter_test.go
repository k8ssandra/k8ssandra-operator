package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestAddReaperSettingsToDcConfig(t *testing.T) {
	tests := []struct {
		name           string
		reaperTemplate *reaperapi.ReaperClusterTemplate
		actual         *cassandra.DatacenterConfig
		expected       *cassandra.DatacenterConfig
	}{
		{
			"defaults",
			&reaperapi.ReaperClusterTemplate{},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
			},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
				Users:   []cassdcapi.CassandraUser{{SecretName: "cluster1-reaper", Superuser: true}},
				PodTemplateSpec: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:            "jmx-credentials",
							Image:           "docker.io/busybox:1.33.1",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name: "REAPER_JMX_USERNAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "cluster1-reaper-jmx"},
											Key:                  "username",
										},
									},
								},
								{
									Name: "REAPER_JMX_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: "cluster1-reaper-jmx"},
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
						}},
						Containers: []corev1.Container{{
							Name: reconciliation.CassandraContainerName,
							Env:  []corev1.EnvVar{{Name: "LOCAL_JMX", Value: "no"}},
						}},
					},
				},
			},
		},
		{
			"existing objects",
			&reaperapi.ReaperClusterTemplate{
				CassandraUserSecretRef: "cass-user",
				JmxUserSecretRef:       "jmx-user",
			},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
				Users:   []cassdcapi.CassandraUser{{SecretName: "another-user", Superuser: true}},
				PodTemplateSpec: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name: "another-init-container",
						}},
						Containers: []corev1.Container{
							{
								Name: reconciliation.CassandraContainerName,
								Env:  []corev1.EnvVar{{Name: "ANOTHER_VAR", Value: "irrelevant"}},
							},
							{
								Name: "another-container",
							},
						},
					},
				},
			},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
				Users: []cassdcapi.CassandraUser{
					{SecretName: "another-user", Superuser: true},
					{SecretName: "cass-user", Superuser: true},
				},
				PodTemplateSpec: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: "another-init-container"},
							{
								Name:            "jmx-credentials",
								Image:           "docker.io/busybox:1.33.1",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Env: []corev1.EnvVar{
									{
										Name: "REAPER_JMX_USERNAME",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: "jmx-user"},
												Key:                  "username",
											},
										},
									},
									{
										Name: "REAPER_JMX_PASSWORD",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: "jmx-user"},
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
							}},
						Containers: []corev1.Container{
							{
								Name: reconciliation.CassandraContainerName,
								Env: []corev1.EnvVar{
									{Name: "ANOTHER_VAR", Value: "irrelevant"},
									{Name: "LOCAL_JMX", Value: "no"},
								},
							},
							{
								Name: "another-container",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddReaperSettingsToDcConfig(tt.reaperTemplate, tt.actual)
			assert.Equal(t, tt.expected, tt.actual)
		})
	}
}
