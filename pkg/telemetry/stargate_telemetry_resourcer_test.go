package telemetry

import (
	"github.com/go-logr/logr"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
)

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
