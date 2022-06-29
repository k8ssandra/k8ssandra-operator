package v1alpha1

import (
	"k8s.io/utils/pointer"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTelemetrySpec_Merge(t *testing.T) {
	type fields struct {
	}
	type args struct {
	}
	tests := []struct {
		name string
		a    *TelemetrySpec
		b    *TelemetrySpec
		want *TelemetrySpec
	}{
		{"nil nil", nil, nil, nil},
		{"nil zero struct", nil, &TelemetrySpec{}, &TelemetrySpec{}},
		{"zero struct nil", &TelemetrySpec{}, nil, &TelemetrySpec{}},
		{"zero struct zero struct", &TelemetrySpec{}, &TelemetrySpec{}, &TelemetrySpec{}},
		{
			"prom nil prom non nil",
			&TelemetrySpec{},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
		},
		{
			"prom non nil prom nil",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
		},
		{
			"prom non nil prom non nil",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
		},
		{
			"enabled nil enabled false",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
		},
		{
			"enabled nil enabled true",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
		},
		{
			"enabled false enabled nil",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
		},
		{
			"enabled false enabled false",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
		},
		{
			"enabled false enabled true",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
		},
		{
			"enabled true enabled nil",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
		},
		{
			"enabled true enabled false",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(false)}}, // because of trilean logic
		},
		{
			"enabled true enabled true",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{Enabled: pointer.Bool(true)}},
		},
		{
			"labels nil labels empty",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{}}},
		},
		{
			"labels empty labels nil",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{}}},
		},
		{
			"labels empty labels non-empty",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1"}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1"}}},
		},
		{
			"labels non-empty labels empty",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1"}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1"}}},
		},
		{
			"labels non-empty labels non-empty",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1"}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label2": "value2"}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1", "label2": "value2"}}},
		},
		{
			"labels non-empty labels non-empty conflicting key",
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1", "label2": "value2"}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label2": "SHOULD WIN", "label3": "value3"}}},
			&TelemetrySpec{Prometheus: &PrometheusTelemetrySpec{CommonLabels: map[string]string{"label1": "value1", "label2": "SHOULD WIN", "label3": "value3"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Merge(tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}
