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
	fakeClient := test.NewFakeClientWRestMapper()

	toml, err := CreateCassandraVectorToml(context.Background(), telemetrySpec, fakeClient, "k8ssandra-operator")
	if err != nil {
		t.Errorf("CreateCassandraVectorToml() failed with %s", err)
	}

	assert.Contains(t, toml, "[sinks.console]")
}

func TestBuildVectorAgentConfigMap(t *testing.T) {
	vectorToml := "Test"
	vectorConfigMap := BuildVectorAgentConfigMap("k8ssandra-operator", "k8ssandra", "dc1", "k8ssandra-operator", vectorToml)
	assert.Equal(t, vectorToml, vectorConfigMap.Data["vector.toml"])
	assert.Equal(t, "k8ssandra-dc1-cass-vector", vectorConfigMap.Name)
	assert.Equal(t, "k8ssandra-operator", vectorConfigMap.Namespace)
}

func TestBuildVectorToml(t *testing.T) {
	tests := []struct {
		name  string
		tspec *telemetry.TelemetrySpec
		want  string
	}{
		{
			"Single sink",
			&telemetry.TelemetrySpec{
				Vector: &telemetry.VectorSpec{
					Enabled: pointer.Bool(true),
					Components: &telemetry.VectorComponentsSpec{
						Sinks: []telemetry.VectorSinkSpec{
							{
								Name: "console_sink",
								Type: "console",
								Inputs: []string{
									"test",
									"test2",
								},
							},
						},
					},
				},
			},
			`
[sinks.console_sink]
type = "console"
inputs = ["test", "test2"]
`,
		},
		{
			"Source, sink and transform",
			&telemetry.TelemetrySpec{
				Vector: &telemetry.VectorSpec{
					Enabled: pointer.Bool(true),
					Components: &telemetry.VectorComponentsSpec{
						Sources: []telemetry.VectorSourceSpec{
							{
								Name: "custom_source",
								Type: "whatever",
								Config: `foo = "bar"
baz = 1`,
							},
						},
						Transforms: []telemetry.VectorTransformSpec{
							{
								Name:   "custom_transform1",
								Type:   "remap",
								Inputs: []string{"custom_source"},
								Config: `foo = "bar"
baz = 2`,
							},
							{
								Name:   "custom_transform2",
								Type:   "remap",
								Inputs: []string{"custom_transform1"},
								Config: `foo = "bar"
baz = 3
bulk.index = "vector-%Y-%m-%d"`,
							},
						},
						Sinks: []telemetry.VectorSinkSpec{
							{
								Name: "console_sink",
								Type: "console",
								Inputs: []string{
									"test",
									"test2",
								},
							},
						},
					},
				},
			}, `
[sources.custom_source]
type = "whatever"
foo = "bar"
baz = 1

[transforms.custom_transform1]
type = "remap"
inputs = ["custom_source"]
foo = "bar"
baz = 2

[transforms.custom_transform2]
type = "remap"
inputs = ["custom_transform1"]
foo = "bar"
baz = 3
bulk.index = "vector-%Y-%m-%d"

[sinks.console_sink]
type = "console"
inputs = ["test", "test2"]
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCustomVectorToml(tt.tspec)
			assert.Equal(t, tt.want, got)
		})
	}
}
