package telemetry

import (
	"context"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

// InjectCassandraVectorAgent adds the Vector agent container to the Cassandra pods.
// If the Vector agent is already present, it is not added again.
func TestInjectCassandraVectorAgent(t *testing.T) {
	telemetrySpec := &telemetry.TelemetrySpec{Vector: &telemetry.VectorSpec{Enabled: pointer.Bool(true)}}
	dcConfig := &cassandra.DatacenterConfig{
		PodTemplateSpec: corev1.PodTemplateSpec{},
	}

	logger := testlogr.NewTestLogger(t)

	err := InjectCassandraVectorAgent(telemetrySpec, dcConfig, "test", logger)
	require.NoError(t, err)

	assert.Equal(t, 1, len(dcConfig.PodTemplateSpec.Spec.Containers))
	assert.Equal(t, "vector-agent", dcConfig.PodTemplateSpec.Spec.Containers[0].Name)
	assert.Equal(t, resource.MustParse(DefaultVectorCpuLimit), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse(DefaultVectorCpuRequest), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse(DefaultVectorMemoryLimit), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse(DefaultVectorMemoryRequest), *dcConfig.PodTemplateSpec.Spec.Containers[0].Resources.Requests.Memory())
}

func TestCreateCassandraVectorTomlDefault(t *testing.T) {
	telemetrySpec := &telemetry.TelemetrySpec{Vector: &telemetry.VectorSpec{Enabled: pointer.Bool(true)}}
	dcConfig := &cassandra.DatacenterConfig{
		PodTemplateSpec: corev1.PodTemplateSpec{},
	}

	fakeClient := test.NewFakeClientWRestMapper()

	toml, err := CreateCassandraVectorToml(context.Background(), telemetrySpec, dcConfig, fakeClient, "k8ssandra-operator")
	if err != nil {
		t.Errorf("CreateCassandraVectorToml() failed with %s", err)
	}

	assert.Contains(t, toml, "[sinks.console]")
}

func TestBuildVectorAgentConfigMap(t *testing.T) {
	vectorToml := "Test"
	vectorConfigMap := BuildVectorAgentConfigMap("k8ssandra-operator", "k8ssandra", "k8ssandra-operator", vectorToml)
	assert.Equal(t, vectorToml, vectorConfigMap.Data["vector.toml"])
	assert.Equal(t, "k8ssandra-cass-vector", vectorConfigMap.Name)
	assert.Equal(t, "k8ssandra-operator", vectorConfigMap.Namespace)
}
