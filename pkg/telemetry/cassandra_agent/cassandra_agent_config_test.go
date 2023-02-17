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
	expectedYaml string = `endpoint:
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
)

func getExpectedConfigMap() corev1.ConfigMap {
	expectedCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Cfg.DcNamespace,
			Name:      Cfg.Kluster.Name + "-" + Cfg.DcName + "-metrics-agent-config",
		},
		Data: map[string]string{filepath.Base(agentConfigLocation): expectedYaml},
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
	tspec.Cassandra.Endpoint.Address = "127.0.0.1"
	tspec.Cassandra.Endpoint.Port = "10000"
	return *tspec
}

func Test_GetTelemetryAgentConfigMap(t *testing.T) {
	expectedCm := getExpectedConfigMap()
	Cfg.RemoteClient = testutils.NewFakeClientWRestMapper() // Reset the Client
	Cfg.TelemetrySpec = getExampleTelemetrySpec()
	cm, err := Cfg.GetTelemetryAgentConfigMap()
	assert.NoError(t, err)
	assert.Equal(t, expectedCm.Data["metrics-collector.yaml"], cm.Data["metrics-collector.yaml"])
	assert.Equal(t, expectedCm.Name, cm.Name)
	assert.Equal(t, expectedCm.Namespace, cm.Namespace)
}
