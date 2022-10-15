package telemetry

import (
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"

	"github.com/stretchr/testify/assert"
)

// Test_NewServiceMonitor_SUCCESS tests that a new service monitor is successfully returned.
func Test_PrometheusResourcer_NewReaperServiceMonitor_SUCCESS(t *testing.T) {
	logger := testlogr.NewTestLogger(t)
	cfg := PrometheusResourcer{
		MonitoringTargetNS:   "test-namespace",
		MonitoringTargetName: "test-dc-name",
		Logger:               logger,
		ServiceMonitorName:   "test-servicemonitor",
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
	}
	actualSM, err := cfg.NewReaperServiceMonitor()
	if err != nil {
		assert.Fail(t, "error creating new service monitor", err)
	}
	assert.Equal(t, "admin", actualSM.Spec.Endpoints[0].Port)
}
