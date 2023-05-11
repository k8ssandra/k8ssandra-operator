package telemetry

import (
	"testing"

	"github.com/go-logr/logr/testr"

	k8ssandra "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

func TestInjectCassandraVectorAgentConfig(t *testing.T) {
	telemetrySpec := &telemetry.TelemetrySpec{Vector: &telemetry.VectorSpec{Enabled: pointer.Bool(true)}}
	dcConfig := &cassandra.DatacenterConfig{
		Meta: k8ssandra.EmbeddedObjectMeta{
			Name: "dc1",
		},
		Cluster:         "test1",
		PodTemplateSpec: corev1.PodTemplateSpec{},
	}

	logger := testr.New(t)

	err := InjectCassandraVectorAgentConfig(telemetrySpec, dcConfig, "test", logger)
	require.NoError(t, err)

	assert.Equal(t, 1, len(dcConfig.PodTemplateSpec.Spec.Containers))
	assert.Equal(t, "server-system-logger", dcConfig.PodTemplateSpec.Spec.Containers[0].Name)
	assert.Equal(t, resource.MustParse(vector.DefaultVectorCpuLimit), *dcConfig.SystemLoggerResources.Limits.Cpu())
	assert.Equal(t, resource.MustParse(vector.DefaultVectorCpuRequest), *dcConfig.SystemLoggerResources.Requests.Cpu())
	assert.Equal(t, resource.MustParse(vector.DefaultVectorMemoryLimit), *dcConfig.SystemLoggerResources.Limits.Memory())
	assert.Equal(t, resource.MustParse(vector.DefaultVectorMemoryRequest), *dcConfig.SystemLoggerResources.Requests.Memory())
}

func TestCreateCassandraVectorTomlDefault(t *testing.T) {
	telemetrySpec := &telemetry.TelemetrySpec{Vector: &telemetry.VectorSpec{Enabled: pointer.Bool(true)}}

	toml, err := CreateCassandraVectorToml(telemetrySpec, true)
	if err != nil {
		t.Errorf("CreateCassandraVectorToml() failed with %s", err)
	}

	assert.Contains(t, toml, "[sinks.console_log]")
	assert.NotContains(t, toml, "http://localhost:9000/metrics")
}

func TestCreateCassandraVectorTomlMcacDisabled(t *testing.T) {
	telemetrySpec := &telemetry.TelemetrySpec{Mcac: &telemetry.McacTelemetrySpec{Enabled: pointer.Bool(false)},
		Vector: &telemetry.VectorSpec{
			Enabled: pointer.Bool(true),
			Components: &telemetry.VectorComponentsSpec{
				Sinks: []telemetry.VectorSinkSpec{
					{
						Name:   "metrics_output",
						Inputs: []string{"cassandra_metrics"},
					},
				},
			},
		}}

	toml, err := CreateCassandraVectorToml(telemetrySpec, false)
	if err != nil {
		t.Errorf("CreateCassandraVectorToml() failed with %s", err)
	}

	assert.Contains(t, toml, "http://localhost:9000/metrics")
}

func TestBuildVectorAgentConfigMap(t *testing.T) {
	vectorToml := "Test"
	vectorConfigMap := BuildVectorAgentConfigMap("k8ssandra-operator", "k8ssandra", "dc1", "k8ssandra-operator", vectorToml)
	assert.Equal(t, vectorToml, vectorConfigMap.Data["vector.toml"])
	assert.Equal(t, "k8ssandra-dc1-cass-vector", vectorConfigMap.Name)
	assert.Equal(t, "k8ssandra-operator", vectorConfigMap.Namespace)
}

func TestBuildCustomVectorToml(t *testing.T) {
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
								Name: "console",
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
[sinks.console]
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

func TestBuildCustomConfigWithDefaults(t *testing.T) {
	assert := assert.New(t)
	telemetrySpec := &telemetry.TelemetrySpec{
		Vector: &telemetry.VectorSpec{
			Enabled: pointer.Bool(true),
			Components: &telemetry.VectorComponentsSpec{
				Sinks: []telemetry.VectorSinkSpec{
					{
						Name: "console",
						Type: "console",
						Inputs: []string{
							"cassandra_metrics",
						},
					},
				},
			},
		},
	}

	expectedOutput := `
[sources.systemlog]
type = "file"
include = [ "/var/log/cassandra/system.log" ]
read_from = "beginning"
fingerprint.strategy = "device_and_inode"
[sources.systemlog.multiline]
start_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
condition_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
mode = "halt_before"
timeout_ms = 10000


[sources.cassandra_metrics]
type = "prometheus_scrape"
endpoints = [ "http://localhost:9000/metrics" ]
scrape_interval_secs = 30

[sinks.console]
type = "console"
inputs = ["cassandra_metrics"]

[sinks.console_log]
type = "console"
inputs = ["systemlog"]
target = "stdout"
encoding.codec = "text"

`

	output, err := CreateCassandraVectorToml(telemetrySpec, false)
	assert.NoError(err)
	assert.Equal(expectedOutput, output)
}

func TestDefaultRemoveUnusedSources(t *testing.T) {
	assert := assert.New(t)
	sources, transformers, sinks := BuildDefaultVectorComponents(vector.VectorConfig{})
	assert.Equal(2, len(sources))
	assert.Equal(1, len(transformers))
	assert.Equal(1, len(sinks))

	sources, transformers, sinks = FilterUnusedPipelines(sources, transformers, sinks)

	assert.Equal(1, len(sources))
	assert.Equal(0, len(transformers))
	assert.Equal(1, len(sinks))
}

func TestRemoveUnusedSourcesModified(t *testing.T) {
	assert := assert.New(t)
	sources, transformers, sinks := BuildDefaultVectorComponents(vector.VectorConfig{})
	assert.Equal(2, len(sources))
	assert.Equal(1, len(transformers))
	assert.Equal(1, len(sinks))

	sinks = append(sinks, telemetry.VectorSinkSpec{Name: "a", Inputs: []string{"cassandra_metrics"}})

	sources, transformers, sinks = FilterUnusedPipelines(sources, transformers, sinks)

	assert.Equal(2, len(sources))
	assert.Equal(0, len(transformers))
	assert.Equal(2, len(sinks))
}

func TestRemoveUnusedTransformers(t *testing.T) {
	assert := assert.New(t)
	sources := []telemetry.VectorSourceSpec{
		{
			Name: "a",
		},
	}

	transformers := []telemetry.VectorTransformSpec{
		{
			Name:   "b",
			Inputs: []string{"a"},
		},
		{
			Name:   "c",
			Inputs: []string{"b"},
		},
		{
			Name:   "d",
			Inputs: []string{"b"},
		},
	}

	sinks := []telemetry.VectorSinkSpec{
		{
			Name:   "e",
			Inputs: []string{"c"},
		},
		{
			Name:   "f",
			Inputs: []string{"d"},
		},
	}

	filteredSources, filteredTransformers, filteredSinks := FilterUnusedPipelines(sources, transformers, sinks)

	assert.Equal(sources, filteredSources)
	assert.Equal(transformers, filteredTransformers)
	assert.Equal(sinks, filteredSinks)

	// Remove f, we should get rid of transformer d, but not b,c
	sinks = sinks[:1]

	filteredSources, filteredTransformers, filteredSinks = FilterUnusedPipelines(sources, transformers, sinks)

	assert.Equal(1, len(filteredSources))
	assert.Equal(2, len(filteredTransformers))
	assert.Equal(sinks, filteredSinks)

	// Remove e, we should get rid of everything
	sinks = []telemetry.VectorSinkSpec{}

	filteredSources, filteredTransformers, filteredSinks = FilterUnusedPipelines(sources, transformers, sinks)

	assert.Equal(0, len(filteredSources))
	assert.Equal(0, len(filteredTransformers))
	assert.Equal(sinks, filteredSinks)
}

func TestOverrideSourcePossible(t *testing.T) {
	assert := assert.New(t)
	sources, transformers, sinks := BuildDefaultVectorComponents(vector.VectorConfig{})
	assert.Equal(2, len(sources))
	assert.Equal(1, len(transformers))
	assert.Equal(1, len(sinks))

	newSources := []telemetry.VectorSourceSpec{
		{
			Name: "systemlog",
			Type: "stdin",
		},
	}

	newSources = append(newSources, sources...)

	sources, transformers, sinks = FilterUnusedPipelines(newSources, transformers, sinks)

	assert.Equal(1, len(sources))
	assert.Equal(0, len(transformers))
	assert.Equal(1, len(sinks))

	assert.Equal("stdin", sources[0].Type)
}
