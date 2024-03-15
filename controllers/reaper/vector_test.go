package reaper

import (
	"testing"

	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	reaperpkg "github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestCreateVectorTomlDefault(t *testing.T) {
	telemetrySpec := &telemetryapi.TelemetrySpec{Vector: &telemetryapi.VectorSpec{Enabled: pointer.Bool(true)}}

	toml, err := CreateVectorToml(telemetrySpec)
	if err != nil {
		t.Errorf("CreateVectorToml() failed with %s", err)
	}

	assert.Contains(t, toml, "[sinks.console]")
	assert.Contains(t, toml, "[sources.reaper_metrics]")
}

func TestBuildVectorAgentConfigMap(t *testing.T) {
	vectorToml := "Test"
	vectorConfigMap := reaperpkg.CreateVectorConfigMap("k8ssandra-operator", vectorToml, test.NewCassandraDatacenter("testDc", "k8ssandra-operator"))
	assert.Equal(t, vectorToml, vectorConfigMap.Data["vector.toml"])
	assert.Equal(t, "test-cluster-testdc-reaper-vector", vectorConfigMap.Name)
	assert.Equal(t, "k8ssandra-operator", vectorConfigMap.Namespace)
}
