package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTelemetrySpec_Merge tests that the TelemetrySpec Merge function behaves as expected.
func TestTelemetrySpec_Merge_OneNil(t *testing.T) {
	a := TelemetrySpec{}
	b := TelemetrySpec{
		Prometheus: &PrometheusTelemetrySpec{
			Enabled: false,
		},
	}
	actual := a.Merge(&b)
	expected := &TelemetrySpec{
		Prometheus: &PrometheusTelemetrySpec{
			Enabled: false,
		},
	}
	assert.Equal(t, expected, actual)
}

func TestTelemetrySpec_Merge_BothNil(t *testing.T) {
	a := (*TelemetrySpec)(nil)
	b := (*TelemetrySpec)(nil)
	actual := a.Merge(b)
	expected := (*TelemetrySpec)(nil)
	assert.Equal(t, expected, actual)
}

func TestPrometheusTelemetrySpec_Merge_OneNil(t *testing.T) {
	a := PrometheusTelemetrySpec{}
	b := PrometheusTelemetrySpec{
		Enabled: false,
	}
	actual := a.Merge(&b)
	expected := &PrometheusTelemetrySpec{
		Enabled: false,
	}
	assert.Equal(t, expected, actual)
}

func TestTelemetrySpec_Merge_ATrueBFalse(t *testing.T) {
	a := TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{
		Enabled: true,
	},
	}
	b := TelemetrySpec{
		Prometheus: &PrometheusTelemetrySpec{
			Enabled: false,
		},
	}
	actual := a.Merge(&b)
	expected := &TelemetrySpec{
		Prometheus: &PrometheusTelemetrySpec{
			Enabled: false,
		},
	}
	assert.Equal(t, *expected, *actual)
}
func TestPrometheusTelemetrySpec_Merge_AFalseBTrue(t *testing.T) {
	a := PrometheusTelemetrySpec{
		Enabled: false,
	}
	b := PrometheusTelemetrySpec{
		Enabled: true,
	}
	actual := a.Merge(&b)
	expected := &PrometheusTelemetrySpec{
		Enabled: true,
	}
	assert.Equal(t, expected, actual)
}
