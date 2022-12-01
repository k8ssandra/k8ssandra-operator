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
			want: false,
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
		name    string
		cluster *TelemetrySpec
		dc      *TelemetrySpec
		want    *TelemetrySpec
	}{
		{
			name:    "nil cluster, nil dc",
			cluster: nil,
			dc:      nil,
			want:    nil,
		},
		{
			name:    "empty cluster, empty dc",
			cluster: &TelemetrySpec{},
			dc:      &TelemetrySpec{},
			want:    &TelemetrySpec{},
		},
		{
			name: "non empty cluster, nil dc",
			cluster: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
					CommonLabels: map[string]string{
						"key1": "value1",
					},
				},
			},
			dc: nil,
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
			name:    "nil cluster, non empty dc",
			cluster: nil,
			dc: &TelemetrySpec{
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
			name: "non empty cluster, non empty dc",
			cluster: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(true),
					CommonLabels: map[string]string{
						"key1": "cluster",
						"key2": "cluster",
					},
				},
			},
			dc: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(false),
					CommonLabels: map[string]string{
						"key1": "dc",
						"key3": "dc",
					},
				},
			},
			want: &TelemetrySpec{
				Prometheus: &PrometheusTelemetrySpec{
					Enabled: pointer.Bool(false),
					CommonLabels: map[string]string{
						"key1": "dc",
						"key2": "cluster",
						"key3": "dc",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.dc.MergeWith(tt.cluster))
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
