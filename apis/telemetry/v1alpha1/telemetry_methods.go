package v1alpha1

import goalesceutils "github.com/k8ssandra/k8ssandra-operator/pkg/goalesce"

// MergeWith merges the given cluster-level template into this (DC-level) template.
func (in *TelemetrySpec) MergeWith(clusterTemplate *TelemetrySpec) *TelemetrySpec {
	return goalesceutils.MergeCRs(clusterTemplate, in)
}

func (in *TelemetrySpec) IsPrometheusEnabled() bool {
	return in != nil && in.Prometheus != nil && in.Prometheus.Enabled != nil && *in.Prometheus.Enabled
}

func (in *TelemetrySpec) IsMcacEnabled() bool {
	return in == nil && in.Mcac == nil && in.Mcac.Enabled == nil && *in.Mcac.Enabled
}

func (in *TelemetrySpec) IsVectorEnabled() bool {
	return in != nil && in.Vector != nil && in.Vector.Enabled != nil && *in.Vector.Enabled
}
