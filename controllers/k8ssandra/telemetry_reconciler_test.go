// Some of the tests in this package target functions from pkg/telemetry, because they need to be envtests. We prefer to keep envtests in the controller
// packages.
package k8ssandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_IsPromInstalled tests whether the IsPromInstalled function correctly identifies whether prom is installed or not.
func Test_IsPromInstalled(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// Test_PromNotInstalled_NoTelemetrySpec tests that when prometheus is not installed and there is no TelemetrySpec,
// no errors arise in reconciliation.
func Test_PromNotInstalled_NoTelemetrySpec_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// Test_PromInstalled_NoTelemetrySpec tests that when prometheus IS installed and there is no TelemetrySpec,
// no errors arise in reconciliation.
func Test_PromInstalled_NoTelemetrySpec_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// Test_PromInstalled_TelemetrySpec tests that when prometheus IS installed and there IS a TelemetrySpec, no errors are observed.
// (We cannot obtain an actual ServiceMonitor as we are not running the Prometheus operator controller).
func Test_PromInstalled_TelemetrySpec_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// Test_PromNotInstalled_TelemetrySpec_FAIL tests that when Prometheus is not installed and a TelemetrySpec is defined,
// the reconciliation loop fails out.
func Test_PromNotInstalled_TelemetrySpec_FAIL(t *testing.T) {

	assert.Fail(t, "not implemented")
}

// TestCassPrometheusResourcer_UpdateResources_SUCCESS tests that NewServiceMonitor succeeds in creating a ServiceMonitor.
func TestCassPrometheusResourcer_UpdateResources_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// TestCassPrometheusResourcer_CleanupResources_FAIL_Incomplete tests that the correct error type is
// returned when an incomplete CassandraSMConfig is passed to NewServiceMonitor.
func TestCassPrometheusResourcer_CleanupResources_FAIL_Incomplete(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// TestCassTelemetryResourcer_CreateResources_SUCCESS tests that NewServiceMonitor succeeds in creating the expected resources.
func TestCassTelemetryResourcer_CreateResources_SUCCESS(t *testing.T) {
	assert.Fail(t, "not implemented")
}

// TestCassTelemetryResourcer_CleanupResources_FAIL_Incomplete tests that the correct error type is returned when an incomplete
// CassandraSMConfig is passed to NewServiceMonitor.
func TestCassTelemetryResourcer_CleanupResources_FAIL_Incomplete(t *testing.T) {
	assert.Fail(t, "not implemented")
}
