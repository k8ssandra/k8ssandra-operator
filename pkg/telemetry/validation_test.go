package telemetry

import (
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
	"testing"
)

func TestSpecIsValid(t *testing.T) {
	tests := []struct {
		name          string
		tspec         *telemetryapi.TelemetrySpec
		promInstalled bool
		want          bool
	}{
		{
			"nil spec, prom not installed",
			nil,
			false,
			true,
		},
		{
			"nil prom, prom not installed",
			&telemetryapi.TelemetrySpec{},
			false,
			true,
		},
		{
			"prom empty, prom not installed",
			&telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{}},
			false,
			true,
		},
		{
			"prom not enabled, prom not installed",
			&telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			false,
			true,
		},
		{
			"prom enabled, prom not installed",
			&telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			false,
			false,
		},
		{
			"nil spec, prom installed",
			nil,
			true,
			true,
		},
		{
			"nil prom, prom installed",
			&telemetryapi.TelemetrySpec{},
			true,
			true,
		},
		{
			"prom empty, prom installed",
			&telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{}},
			true,
			true,
		},
		{
			"prom not enabled, prom installed",
			&telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			true,
			true,
		},
		{
			"prom enabled, prom installed",
			&telemetryapi.TelemetrySpec{Prometheus: &telemetryapi.PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			true,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpecIsValid(tt.tspec, tt.promInstalled)
			assert.Equal(t, tt.want, got)
		})
	}
}
