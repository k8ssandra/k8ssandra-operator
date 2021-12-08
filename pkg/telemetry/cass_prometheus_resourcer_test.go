// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCassPrometheusResourcer_UpdateResources_SUCCESS tests that NewServiceMonitor succeeds in creating a ServiceMonitor.
func TestCassPrometheusResourcer_UpdateResources_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

//TestNewServiceMonitor_SUCCESS tests that a new service monitor is successfully returned.
func TestNewServiceMonitor_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// TestCassPrometheusResourcer_CleanupResources_FAIL_Incomplete tests that the correct error type is returned when an incomplete
// CassandraSMConfig is passed to NewServiceMonitor.
func TestCassPrometheusResourcer_CleanupResources_FAIL_Incomplete(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// TestCassPrometheusResourcer_IS_TelemetryResourcer tests that CassPrometheusResourcer implements the Resourcer interface.
func TestCassPrometheusResourcer_IS_TelemetryResourcer(t *testing.T) {
	assert.Fail(t, "This should be implemented in an envtest probably")
}
