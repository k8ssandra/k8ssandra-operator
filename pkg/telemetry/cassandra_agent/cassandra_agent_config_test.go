package cassandra_agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
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
relabels:
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
relabels:
- regex: .+
  replacement: "true"
  sourceLabels:
  - table
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_live_ss_table_count
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_live_disk_space_used
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_memtable.*
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_all_memtables.*
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_compaction_bytes_written
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_pending_compactions
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_read_.*
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_write_.*
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_range.*
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_coordinator_.*
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- regex: org_apache_cassandra_metrics_table_dropped_mutations
  replacement: "false"
  sourceLabels:
  - __name__
  targetLabel: should_drop
- action: drop
  regex: "true"
  sourceLabels:
  - should_drop
`
	relabelsDefinedYaml = `endpoint:
  address: 127.0.0.1
  port: "9000"
relabels:
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
	tspec.Cassandra.Relabels = []promapi.RelabelConfig{
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

// Make sure when both endpoint and relabels are defined they come through to yaml.
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

// Make sure we get default relabels when only endpoint is defined in spec.
func Test_GetTelemetryAgentConfigMapWithDefinedEndpoint(t *testing.T) {
	expectedCm := getExpectedConfigMap()
	expectedCm.Data[filepath.Base(agentConfigLocation)] = endpointDefinedYaml
	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
	Cfg.TelemetrySpec = getExampleTelemetrySpec()
	Cfg.TelemetrySpec.Cassandra.Relabels = nil
	Cfg.TelemetrySpec.Cassandra.Endpoint = &telemetryapi.Endpoint{
		Address: "192.168.1.10",
		Port:    "50000",
	}
	cm, err := Cfg.GetTelemetryAgentConfigMap()
	println(cm.Data)
	assert.NoError(t, err)
	assert.Equal(t, expectedCm.Data["metrics-collector.yaml"], cm.Data["metrics-collector.yaml"])
	assert.Equal(t, expectedCm.Name, cm.Name)
	assert.Equal(t, expectedCm.Namespace, cm.Namespace)
}

func Test_GetTelemetryAgentConfigMapWithDefinedRelabels(t *testing.T) {
	expectedCm := getExpectedConfigMap()
	expectedCm.Data[filepath.Base(agentConfigLocation)] = relabelsDefinedYaml
	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
	Cfg.TelemetrySpec = getExampleTelemetrySpec()
	Cfg.TelemetrySpec.Cassandra.Relabels = []promapi.RelabelConfig{
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
	assert.Equal(t, expectedCm.Data["metrics-collector.yaml"], cm.Data["metrics-collector.yaml"])
	assert.Equal(t, expectedCm.Name, cm.Name)
	assert.Equal(t, expectedCm.Namespace, cm.Namespace)
}
