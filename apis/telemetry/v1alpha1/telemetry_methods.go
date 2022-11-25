package v1alpha1

import goalesceutils "github.com/k8ssandra/k8ssandra-operator/pkg/goalesce"

// MergeWith merges the given cluster-level template into this (DC-level) template.
func (in *TelemetrySpec) MergeWith(clusterTemplate *TelemetrySpec) *TelemetrySpec {
	return goalesceutils.MergeCRDs(clusterTemplate, in)
}

func (in *TelemetrySpec) IsPrometheusEnabled() bool {
	return in != nil && in.Prometheus != nil && in.Prometheus.Enabled != nil && *in.Prometheus.Enabled
}
