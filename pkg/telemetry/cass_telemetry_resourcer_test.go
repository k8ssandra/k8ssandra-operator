// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"github.com/go-logr/logr"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCassTelemetryResourcer_IS_TelemetryResourcer tests that CassTelemetryResourcer implements the Resourcer interface.
func TestCassTelemetryResourcer_IS_TelemetryResourcer(t *testing.T) {
	assert.Implements(t, (*Resourcer)(nil), CassTelemetryResourcer{})
}

// newCassTelemetryResourcer returns a new CassTelemetryResourcer
func newCassTelemetryResourcer(logger logr.Logger) CassTelemetryResourcer {
	return CassTelemetryResourcer{
		CassandraNamespace: "test-namespace",
		DataCenterName:     "test-DC",
		ClusterName:        "test-cluster",
		TelemetrySpec :     &telemetryapi.TelemetrySpec{
			Prometheus: &telemetryapi.PrometheusTelemetrySpec{
				Enabled: true,
			},
		},
		Logger:          logger,
	}
}