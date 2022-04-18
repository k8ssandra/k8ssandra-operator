package v1alpha1

import goalesceutils "github.com/k8ssandra/k8ssandra-operator/pkg/goalesce"

// MergeWith merges the parent object into this one.
func (in *TelemetrySpec) MergeWith(parent *TelemetrySpec) *TelemetrySpec {
	return goalesceutils.Merge(parent, in)
}

func (in *TelemetrySpec) IsPrometheusEnabled() bool {
	return in != nil && in.Prometheus != nil && in.Prometheus.Enabled != nil && *in.Prometheus.Enabled
}
