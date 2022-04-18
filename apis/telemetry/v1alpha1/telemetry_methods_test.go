package v1alpha1

import (
	"k8s.io/utils/pointer"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTelemetrySpec_IsPrometheusEnabled(t *testing.T) {
	tests := []struct {
		name string
		in   *TelemetrySpec
		want bool
	}{
		{
			name: "nil",
			in:   nil,
			want: false,
		},
		{
			name: "nil prometheus",
			in:   &TelemetrySpec{},
			want: false,
		},
		{
			name: "nil enabled",
			in: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{},
			},
		},
		{
			name: "false",
			in: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(false),
				},
			},
			want: false,
		},
		{
			name: "true",
			in: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.IsPrometheusEnabled())
		})
	}
}

func TestTelemetrySpec_MergeWith(t *testing.T) {
	tests := []struct {
		name   string
		in     *TelemetrySpec
		parent *TelemetrySpec
		want   *TelemetrySpec
	}{
		{
			name:   "nil receiver, nil parent",
			in:     nil,
			parent: nil,
			want:   nil,
		},
		{
			name:   "empty receiver, empty parent",
			in:     &TelemetrySpec{},
			parent: &TelemetrySpec{},
			want:   &TelemetrySpec{},
		},
		{
			name: "nil receiver, non empty parent",
			in:   nil,
			parent: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
					CommonLabels: map[string]string{
						"key1": "value1",
					},
				},
			},
			want: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
					CommonLabels: map[string]string{
						"key1": "value1",
					},
				},
			},
		},
		{
			name: "non empty receiver, nil parent",
			in: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
					CommonLabels: map[string]string{
						"key1": "value1",
					},
				},
			},
			parent: nil,
			want: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
					CommonLabels: map[string]string{
						"key1": "value1",
					},
				},
			},
		},
		{
			name: "non empty receiver, non empty parent",
			in: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(false),
					CommonLabels: map[string]string{
						"key1": "receiver",
						"key2": "receiver",
					},
				},
			},
			parent: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
					CommonLabels: map[string]string{
						"key1": "parent",
						"key3": "parent",
					},
				},
			},
			want: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(false),
					CommonLabels: map[string]string{
						"key1": "receiver",
						"key2": "receiver",
						"key3": "parent",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.MergeWith(tt.parent))
		})
	}
}

// A set of tests for the trilean logic applied to the Enabled field, and the result of calling
// IsPrometheusEnabled on the merged object.
func TestTelemetrySpec_MergeEnabled(t *testing.T) {
	tests := []struct {
		name   string
		in     *TelemetrySpec
		parent *TelemetrySpec
		want   bool
	}{
		{
			name:   "receiver enabled nil, parent enabled nil",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: nil}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: nil}},
			want:   false,
		},
		{
			name:   "receiver enabled nil, parent enabled false",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: nil}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			want:   false,
		},
		{
			name:   "receiver enabled nil, parent enabled true",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: nil}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			want:   true,
		},
		{
			name:   "receiver enabled false, parent enabled nil",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: nil}},
			want:   false,
		},
		{
			name:   "receiver enabled false, parent enabled false",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			want:   false,
		},
		{
			name:   "receiver enabled false, parent enabled true",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			want:   false,
		},
		{
			name:   "receiver enabled true, parent enabled nil",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: nil}},
			want:   true,
		},
		{
			name:   "receiver enabled true, parent enabled false",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			want:   true,
		},
		{
			name:   "receiver enabled true, parent enabled true",
			in:     &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			parent: &TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.MergeWith(tt.parent).IsPrometheusEnabled())
		})
	}
}
