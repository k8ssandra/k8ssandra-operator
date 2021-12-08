// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCassTelemetryResourcer_CreateResources_SUCCESS tests that NewServiceMonitor succeeds in creating the expected resources.
func TestCassTelemetryResourcer_CreateResources_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// TestCassTelemetryResourcer_CleanupResources_FAIL_Incomplete tests that the correct error type is returned when an incomplete
// CassandraSMConfig is passed to NewServiceMonitor.
func TestCassTelemetryResourcer_CleanupResources_FAIL_Incomplete(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// TestCassTelemetryResourcer_IS_TelemetryResourcer tests that CassTelemetryResourcer implements the Resourcer interface.
func TestCassTelemetryResourcer_IS_TelemetryResourcer(t *testing.T) {
	assert.Fail(t, "This should be implemented in an envtest probably")
}
