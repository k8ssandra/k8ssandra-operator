package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestAddReaperSettingsToDcConfig(t *testing.T) {
	tests := []struct {
		name           string
		reaperTemplate *reaperapi.ReaperClusterTemplate
		actual         *cassandra.DatacenterConfig
		expected       *cassandra.DatacenterConfig
		kcKey          client.ObjectKey
		auth           bool
	}{
		{
			"defaults auth",
			&reaperapi.ReaperClusterTemplate{},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: reconciliation.CassandraContainerName,
						}},
					},
				},
			},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
				Users:   []cassdcapi.CassandraUser{{SecretName: "cluster1-reaper", Superuser: true}},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: reconciliation.CassandraContainerName,
							Env:  []corev1.EnvVar{{Name: "LOCAL_JMX", Value: "no"}},
						}},
					},
				},
			},
			client.ObjectKey{Namespace: "ns1", Name: "k8c"},
			true,
		},
		{
			"defaults no auth",
			&reaperapi.ReaperClusterTemplate{},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
			},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: reconciliation.CassandraContainerName,
							Env:  []corev1.EnvVar{{Name: "LOCAL_JMX", Value: "no"}},
						}},
					},
				},
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						AdditionalOptions: []string{"-Dcom.sun.management.jmxremote.authenticate=false"},
					},
				},
			},
			client.ObjectKey{Namespace: "ns1", Name: "k8c"},
			false,
		},
		{
			"existing objects auth",
			&reaperapi.ReaperClusterTemplate{
				ReaperTemplate: reaperapi.ReaperTemplate{
					CassandraUserSecretRef: corev1.LocalObjectReference{Name: "cass-user"},
				},
			},
			&cassandra.DatacenterConfig{
				Meta:               api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster:            "cluster1",
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "cassandra-superuser"},
				Users:              []cassdcapi.CassandraUser{{SecretName: "another-user", Superuser: true}},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: reconciliation.CassandraContainerName,
								Env: []corev1.EnvVar{
									{Name: "ANOTHER_VAR", Value: "irrelevant"},
								},
							},
							{
								Name: "another-container",
							},
						},
					},
				},
			},
			&cassandra.DatacenterConfig{
				Meta:               api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster:            "cluster1",
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "cassandra-superuser"},
				Users: []cassdcapi.CassandraUser{
					{SecretName: "another-user", Superuser: true},
					{SecretName: "cass-user", Superuser: true},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
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
			client.ObjectKey{Namespace: "ns1", Name: "k8c"},
			true,
		},
		{
			"existing objects no auth",
			&reaperapi.ReaperClusterTemplate{
				ReaperTemplate: reaperapi.ReaperTemplate{
					// should be ignored
					CassandraUserSecretRef: corev1.LocalObjectReference{Name: "cass-user"},
				},
			},
			&cassandra.DatacenterConfig{
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster: "cluster1",
				Users:   []cassdcapi.CassandraUser{{SecretName: "another-user", Superuser: true}},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
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
				Users:   []cassdcapi.CassandraUser{{SecretName: "another-user", Superuser: true}},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
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
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						AdditionalOptions: []string{"-Dcom.sun.management.jmxremote.authenticate=false"},
					},
				},
			},
			client.ObjectKey{Namespace: "ns1", Name: "k8c"},
			false,
		},
		{
			"default auth, external secrets",
			&reaperapi.ReaperClusterTemplate{
				ReaperTemplate: reaperapi.ReaperTemplate{},
			},
			&cassandra.DatacenterConfig{
				Meta:            api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster:         "cluster1",
				ExternalSecrets: true,
			},
			&cassandra.DatacenterConfig{
				Meta:            api.EmbeddedObjectMeta{Name: "dc1"},
				Cluster:         "cluster1",
				ExternalSecrets: true,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: reconciliation.CassandraContainerName,
							Env:  []corev1.EnvVar{{Name: "LOCAL_JMX", Value: "no"}},
						}},
					},
				},
			},
			client.ObjectKey{Namespace: "ns1", Name: "k8c"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddReaperSettingsToDcConfig(tt.reaperTemplate, tt.actual, tt.auth)
			assert.Equal(t, tt.expected, tt.actual)
		})
	}
}
