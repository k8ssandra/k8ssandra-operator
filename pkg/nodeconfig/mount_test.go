package nodeconfig

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/k8ssandra/cass-operator/pkg/images"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestMountPerNodeConfig(t *testing.T) {
	testImage := "test-registry/test/yq-test:1.2.3"
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
				PerNodeConfigMapRef: corev1.LocalObjectReference{
					Name: "test-dc1-per-node-config",
				},
			},
			wantConfig: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name:      "dc1",
					Namespace: "dc1-ns",
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: reconciliation.ServerConfigContainerName},
							newPerNodeConfigInitContainer("", getTestImageRegistry()),
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
		{
			name: "overriding-image",
			dcConfig: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name:      "dc1",
					Namespace: "dc1-ns",
				},
				PerNodeConfigMapRef: corev1.LocalObjectReference{
					Name: "test-dc1-per-node-config",
				},
				PerNodeInitContainerImage: testImage,
			},
			wantConfig: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name:      "dc1",
					Namespace: "dc1-ns",
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: reconciliation.ServerConfigContainerName},
							newPerNodeConfigInitContainer(testImage, getTestImageRegistry()),
						},
						Volumes: []corev1.Volume{
							newPerNodeConfigVolume("test-dc1-per-node-config"),
						},
					},
				},
				PerNodeConfigMapRef: corev1.LocalObjectReference{
					Name: "test-dc1-per-node-config",
				},
				PerNodeInitContainerImage: testImage,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MountPerNodeConfig(tt.dcConfig, getTestImageRegistry())
			assert.Equal(t, tt.wantConfig, tt.dcConfig)
		})
	}
}

func TestCorrectVolumes(t *testing.T) {
	require := require.New(t)
	initContainerConfig := newPerNodeConfigInitContainer("", getTestImageRegistry())
	require.Equal(3, len(initContainerConfig.VolumeMounts))
	require.Equal("server-config", initContainerConfig.VolumeMounts[0].Name)
	require.Equal("/config", initContainerConfig.VolumeMounts[0].MountPath)

	require.Equal(PerNodeConfigVolumeName, initContainerConfig.VolumeMounts[1].Name)
	require.Equal("/per-node-config", initContainerConfig.VolumeMounts[1].MountPath)

	require.Equal("tmp", initContainerConfig.VolumeMounts[2].Name)
	require.Equal("/tmp", initContainerConfig.VolumeMounts[2].MountPath)
}

var (
	regOnce           sync.Once
	imageRegistryTest images.ImageRegistry
)

func getTestImageRegistry() images.ImageRegistry {
	regOnce.Do(func() {
		p := filepath.Clean("../../test/testdata/imageconfig/image_config_test.yaml")
		data, err := os.ReadFile(p)
		if err == nil {
			if r, e := images.NewImageRegistryV2(data); e == nil {
				imageRegistryTest = r
			}
		}
	})
	return imageRegistryTest
}
