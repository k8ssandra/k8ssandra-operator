// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// CassPrometheusResourcer_CreateResources_SUCCESS_Test tests that NewServiceMonitor succeeds in returning a ServiceMonitor.
func CassPrometheusResourcer_CreateResources_SUCCESS_Test(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// CassPrometheusResourcer_CleanupResources_FAIL_Incomplete_Test tests that the correct error type is returned when an incomplete
// CassandraSMConfig is passed to NewServiceMonitor.
func CassPrometheusResourcer_CleanupResources_FAIL_Incomplete_Test(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// CassPrometheusResourcer_IS_Resourcer_Test tests that CassPrometheusResourcer implements the Resourcer interface.
func CassPrometheusResourcer_IS_TelemetryResourcer_Test(t *testing.T) {
	assert.Fail(t, "This should be implemented in an envtest probably")
}
