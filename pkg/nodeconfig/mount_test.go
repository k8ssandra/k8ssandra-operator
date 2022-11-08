package nodeconfig

import (
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestMountPerNodeConfig(t *testing.T) {
	tests := []struct {
		name       string
		dcConfig   *cassandra.DatacenterConfig
		wantConfig *cassandra.DatacenterConfig
	}{
		{
			name: "simple",
			dcConfig: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name:      "dc1",
					Namespace: "dc1-ns",
				},
				PodTemplateSpec: &corev1.PodTemplateSpec{},
				PerNodeConfigMapRef: corev1.LocalObjectReference{
					Name: "test-dc1-per-node-config",
				},
			},
			wantConfig: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name:      "dc1",
					Namespace: "dc1-ns",
				},
				PodTemplateSpec: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: reconciliation.ServerConfigContainerName},
							perNodeConfigInitContainer,
						},
						Volumes: []corev1.Volume{
							newPerNodeConfigVolume("test-dc1-per-node-config"),
						},
					},
				},
				PerNodeConfigMapRef: corev1.LocalObjectReference{
					Name: "test-dc1-per-node-config",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MountPerNodeConfig(tt.dcConfig)
			assert.Equal(t, tt.wantConfig, tt.dcConfig)
		})
	}
}
