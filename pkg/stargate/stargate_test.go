package stargate

import (
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
)

const (
	namespace = "default"
)

func TestNewDeployment(t *testing.T) {
	t.Run("CassandraConfigMap", testCassandraConfigMap)
}

func testCassandraConfigMap(t *testing.T) {
	configMapName := "cassandra-config"

	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "dc1",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ServerVersion: "3.11.11",
			ClusterName:   "test",
		},
	}

	heapSize := resource.MustParse("256m")

	stargate := &api.Stargate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "s1",
		},
		Spec: api.StargateSpec{
			DatacenterRef: corev1.LocalObjectReference{Name: dc.Name},
			StargateTemplate: api.StargateTemplate{
				Size:               1,
				HeapSize:           &heapSize,
				CassandraConfigMap: &corev1.LocalObjectReference{Name: configMapName},
			},
		},
	}

	deployment := NewDeployment(stargate, dc)
	container := findContainer(deployment, getStargateContainerName(dc))
	require.NotNil(t, container, "failed to find stargate container")

	envVar := findEnvVar(container, "JAVA_OPTS")
	require.NotNil(t, envVar, "failed to find JAVA_OPTS env var")
	assert.True(t, strings.Contains(envVar.Value, "-Dstargate.unsafe.cassandra_config_path="+cassandraConfigPath))

	volumeMount := findVolumeMount(container, "cassandra-config")
	require.NotNil(t, volumeMount, "failed to find cassandra-config volume mount")
	assert.Equal(t, "/config", volumeMount.MountPath)

	volume := findVolume(deployment, "cassandra-config")
	require.NotNil(t, volume, "failed to find cassandra-config volume")
	expected := corev1.Volume{
		Name: "cassandra-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
			},
		},
	}
	assert.Equal(t, expected, *volume, "cassandra-config volume does not match expected value")
}

func findContainer(deployment *appsv1.Deployment, name string) *corev1.Container {
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func findEnvVar(container *corev1.Container, name string) *corev1.EnvVar {
	for _, v := range container.Env {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func findVolumeMount(container *corev1.Container, name string) *corev1.VolumeMount {
	for _, v := range container.VolumeMounts {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func findVolume(deployment *appsv1.Deployment, name string) *corev1.Volume {
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == name {
			return &v
		}
	}
	return nil
}
