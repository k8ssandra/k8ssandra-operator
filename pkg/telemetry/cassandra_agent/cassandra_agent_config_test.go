package cassandra_agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	telemetry "github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testCluster k8ssandraapi.K8ssandraCluster = testutils.NewK8ssandraCluster("test-cluster", "test-namespace")
	Cfg         Configurator                  = Configurator{
		TelemetrySpec: telemetry.NewTelemetrySpec(),
		Kluster:       &testCluster,
		Ctx:           context.Background(),
		RemoteClient:  testutils.NewFakeClientWRestMapper(),
		RequeueDelay:  time.Second * 1,
		DcNamespace:   testCluster.Spec.Cassandra.Datacenters[0].Meta.Namespace,
		DcName:        testCluster.Spec.Cassandra.Datacenters[0].Meta.Name,
	}
	allDefinedYaml string = `endpoint:
  address: 127.0.0.1
  port: "10000"
filters:
- action: drop
  regex: (.*);(b.*)
  separator: ;
  sourceLabels:
  - tag1
  - tag2
`
	endpointDefinedYaml string = `endpoint:
  address: 192.168.1.10
  port: "50000"
filters:
- regex: org\.apache\.cassandra\.metrics\.Table.*
  replacement: "true"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table.*
  replacement: "true"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.live_ss_table_count
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.Table\.LiveSSTableCount
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.live_disk_space_used
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.LiveDiskSpaceUsed
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.Table\.Memtable
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.Table\.Compaction
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.read
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.write
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.range
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.coordinator
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- regex: org\.apache\.cassandra\.metrics\.table\.dropped_mutations
  replacement: "false"
  sourceLabels:
  - __origname__
  targetLabel: should_drop
- action: drop
  regex: "true"
  sourceLabels:
  - should_drop
`
	filtersDefinedYaml = `endpoint:
  address: 127.0.0.1
  port: "9000"
filters:
- action: drop
  regex: (.*);(b.*)
  separator: ;
  sourceLabels:
  - tag1
  - tag2
`
)

func getExpectedConfigMap() corev1.ConfigMap {
	expectedCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Cfg.DcNamespace,
			Name:      Cfg.Kluster.Name + "-" + Cfg.DcName + "-metrics-agent-config",
		},
		Data: map[string]string{filepath.Base(agentConfigLocation): allDefinedYaml},
	}
	return expectedCm
}

func getExampleTelemetrySpec() telemetryapi.TelemetrySpec {
	tspec := &Cfg.TelemetrySpec
	tspec.Cassandra.Filters = []promapi.RelabelConfig{
		{
			SourceLabels: []string{"tag1", "tag2"},
			Separator:    ";",
			Regex:        "(.*);(b.*)",
			Action:       "drop",
		},
	}
	tspec.Cassandra.Endpoint = &telemetryapi.Endpoint{
		Address: "127.0.0.1",
		Port:    "10000",
	}
	return *tspec
}

// Make sure when both endpoint and filters are defined they come through to yaml.
func Test_GetTelemetryAgentConfigMapAllDefined(t *testing.T) {
	expectedCm := getExpectedConfigMap()
	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
	Cfg.TelemetrySpec = getExampleTelemetrySpec()
	cm, err := Cfg.GetTelemetryAgentConfigMap()
	assert.NoError(t, err)
	assert.Equal(t, expectedCm.Data["metrics-collector.yaml"], cm.Data["metrics-collector.yaml"])
	assert.Equal(t, expectedCm.Name, cm.Name)
	assert.Equal(t, expectedCm.Namespace, cm.Namespace)
}

// Make sure we get default filters when only endpoint is defined in spec.
func Test_GetTelemetryAgentConfigMapWithDefinedEndpoint(t *testing.T) {
	expectedCm := getExpectedConfigMap()
	expectedCm.Data[filepath.Base(agentConfigLocation)] = endpointDefinedYaml
	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
	Cfg.TelemetrySpec = getExampleTelemetrySpec()
	Cfg.TelemetrySpec.Cassandra.Filters = nil
	Cfg.TelemetrySpec.Cassandra.Endpoint = &telemetryapi.Endpoint{
		Address: "192.168.1.10",
		Port:    "50000",
	}
	cm, err := Cfg.GetTelemetryAgentConfigMap()
	println(cm.Data)
	assert.NoError(t, err)
	assert.Equal(t, expectedCm.Data["metric-collector.yaml"], cm.Data["metric-collector.yaml"])
	assert.Equal(t, expectedCm.Name, cm.Name)
	assert.Equal(t, expectedCm.Namespace, cm.Namespace)
}

func Test_GetTelemetryAgentConfigMapWithDefinedFilters(t *testing.T) {
	expectedCm := getExpectedConfigMap()
	expectedCm.Data[filepath.Base(agentConfigLocation)] = filtersDefinedYaml
	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
	Cfg.TelemetrySpec = getExampleTelemetrySpec()
	Cfg.TelemetrySpec.Cassandra.Filters = []promapi.RelabelConfig{
		{
			SourceLabels: []string{"tag1", "tag2"},
			Separator:    ";",
			Regex:        "(.*);(b.*)",
			Action:       "drop",
		},
	}
	Cfg.TelemetrySpec.Cassandra.Endpoint = nil
	cm, err := Cfg.GetTelemetryAgentConfigMap()
	println(cm.Data)
	assert.NoError(t, err)
	assert.Equal(t, expectedCm.Data["metric-collector.yaml"], cm.Data["metric-collector.yaml"])
	assert.Equal(t, expectedCm.Name, cm.Name)
	assert.Equal(t, expectedCm.Namespace, cm.Namespace)
}

func Test_AddStsVolumes(t *testing.T) {
	dc := testutils.NewCassandraDatacenter("test-dc", "test-namespace")
	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
	Cfg.AddStsVolumes(&dc)
	expectedVol := corev1.Volume{
		Name: "metrics-agent-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				Items: []corev1.KeyToPath{
					{
						Key:  filepath.Base(agentConfigLocation),
						Path: filepath.Base(agentConfigLocation),
					},
				},
				LocalObjectReference: corev1.LocalObjectReference{
					Name: Cfg.Kluster.Name + "-" + Cfg.DcName + "-metrics-agent-config",
				},
			},
		},
	}
	assert.Contains(t, dc.Spec.PodTemplateSpec.Spec.Volumes, expectedVol)
	cassContainer, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "cassandra")
	if !found {
		assert.Fail(t, "no cassandra container found")
	}
	expectedVm := corev1.VolumeMount{
		Name:      "metrics-agent-config",
		MountPath: "/opt/management-api/configs/metric-collector.yaml",
		SubPath:   "metric-collector.yaml",
	}
	assert.Contains(t, dc.Spec.PodTemplateSpec.Spec.Containers[cassContainer].VolumeMounts, expectedVm)
}
