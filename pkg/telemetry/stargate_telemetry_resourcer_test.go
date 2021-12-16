package telemetry

import (
	"testing"

	"github.com/go-logr/logr"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"

	"github.com/stretchr/testify/assert"
)

// TestCassTelemetryResourcer_IS_TelemetryResourcer tests that CassTelemetryResourcer implements the Resourcer interface.
func TestStargateTelemetryResourcer_IS_TelemetryResourcer(t *testing.T) {
	assert.Implements(t, (*Resourcer)(nil), StargateTelemetryResourcer{})
}

// newCassTelemetryResourcer returns a new CassTelemetryResourcer
func newStargateTelemetryResourcer(logger logr.Logger) StargateTelemetryResourcer {
	return StargateTelemetryResourcer{
		StargateNamespace: "test-namespace",
		StargateName:      "test-DC",
		TelemetrySpec: &telemetryapi.TelemetrySpec{
			Prometheus: &telemetryapi.PrometheusTelemetrySpec{
				Enabled: true,
			},
		},
		Logger: logger,
	}
}
