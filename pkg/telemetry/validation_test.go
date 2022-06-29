package telemetry

import (
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
	"testing"
)

// Test_IsValid_Valid tests that the spec is evaluated as valid when... it is valid.
func Test_SpecIsValid_Valid(t *testing.T) {
	spec := telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{
		Enabled: pointer.Bool(false),
	}}
	isValid, err := SpecIsValid(&spec, false)
	if err != nil {
		assert.Fail(t, "couldn't determine if spec was valid or not", err)
	}
	assert.True(t, isValid)
}

// Test_IsValid_Valid tests that the spec is evaluated as valid when... it is not valid.
func Test_SpecIsValid_NotValid(t *testing.T) {
	spec := telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{
		Enabled: pointer.Bool(true),
	}}
	isValid, err := SpecIsValid(&spec, false)
	if err != nil {
		assert.Fail(t, "couldn't determine if spec was valid or not", err)
	}
	assert.False(t, isValid)

}
