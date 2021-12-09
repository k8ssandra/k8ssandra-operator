// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"testing"

	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/assert"
)

//TestNewServiceMonitor_SUCCESS tests that a new service monitor is successfully returned.

func TestNewServiceMonitor_SUCCESS(t *testing.T) {
	enabled := true
	cfg := CassPrometheusResourcer{
		CassTelemetryResourcer: CassTelemetryResourcer{
			CassandraNamespace: "test-namespace",
			DataCenterName:     "test-dc-name",
			ClusterName:        "test-cluster-name",
			TelemetrySpec: &telemetryapi.TelemetrySpec{
				Prometheus: &telemetryapi.PrometheusTelemetrySpec{
					Enabled: &enabled,
				},
			},
		},
		ServiceMonitorName: "test-servicemonitor",
	}
	actualSM, err := cfg.NewServiceMonitor()
	if err != nil {
		assert.Fail(t, "error creating new service monitor", err)
	}
	assert.Equal(t, "prometheus", actualSM.Spec.Endpoints[0].Port)
}

// TestCassPrometheusResourcer_IS_TelemetryResourcer tests that CassPrometheusResourcer implements the Resourcer interface.
func TestCassPrometheusResourcer_IS_TelemetryResourcer(t *testing.T) {
	assert.Implements(t, (*Resourcer)(nil), CassPrometheusResourcer{})
}
