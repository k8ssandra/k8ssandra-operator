package stargate

import (
	"context"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	stargatepkg "github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

// InjectCassandraVectorAgent adds the Vector agent container to the Cassandra pods.
// If the Vector agent is already present, it is not added again.
func TestInjectVectorAgentForStargate(t *testing.T) {
	telemetrySpec := &telemetryapi.TelemetrySpec{Vector: &telemetryapi.VectorSpec{Enabled: pointer.Bool(true)}}
	stargate := &api.Stargate{}
	stargate.Spec.Telemetry = telemetrySpec

	deployments := make(map[string]v1.Deployment)
	deployment := v1.Deployment{}
	deployments["testDeployment"] = deployment

	logger := testlogr.NewTestLogger(t)
	err := InjectVectorAgentForStargate(stargate, deployments, "testDcName", "testClusterName", logger)
	require.NoError(t, err)

	deployment = deployments["testDeployment"]

	assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Containers))
	assert.Equal(t, "stargate-vector-agent", deployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorCpuLimit), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorCpuRequest), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorMemoryLimit), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorMemoryRequest), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory())
}

func TestCreateVectorTomlDefault(t *testing.T) {
	telemetrySpec := &telemetryapi.TelemetrySpec{Vector: &telemetryapi.VectorSpec{Enabled: pointer.Bool(true)}}
	fakeClient := test.NewFakeClientWRestMapper()

	toml, err := CreateVectorToml(context.Background(), telemetrySpec, fakeClient, "k8ssandra-operator")
	if err != nil {
		t.Errorf("CreateVectorToml() failed with %s", err)
	}

	assert.Contains(t, toml, "[sinks.console]")
	assert.Contains(t, toml, "[sources.stargate_metrics]")
}

func TestBuildVectorAgentConfigMap(t *testing.T) {
	vectorToml := "Test"
	vectorConfigMap := stargatepkg.CreateVectorConfigMap("k8ssandra-operator", vectorToml, test.NewCassandraDatacenter("testDc", "k8ssandra-operator"))
	assert.Equal(t, vectorToml, vectorConfigMap.Data["vector.toml"])
	assert.Equal(t, "test-cluster-testDc-stargate-vector", vectorConfigMap.Name)
	assert.Equal(t, "k8ssandra-operator", vectorConfigMap.Namespace)
}
