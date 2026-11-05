package v1alpha1

import (
	"strings"

	goalesceutils "github.com/k8ssandra/k8ssandra-operator/pkg/goalesce"
)

// MergeWith merges the given cluster-level template into this (DC-level) template.
func (in *TelemetrySpec) MergeWith(clusterTemplate *TelemetrySpec) *TelemetrySpec {
	merged := goalesceutils.MergeCRs(clusterTemplate, in)
	components := mergeVectorComponents(vectorComponents(clusterTemplate), vectorComponents(in))
	if components != nil {
		if merged == nil {
			merged = &TelemetrySpec{}
		}
		if merged.Vector == nil {
			merged.Vector = &VectorSpec{}
		}
		merged.Vector.Components = components
	}
	config := mergeVectorConfig(vectorConfig(clusterTemplate), vectorConfig(in))
	if config != "" {
		if merged == nil {
			merged = &TelemetrySpec{}
		}
		if merged.Vector == nil {
			merged.Vector = &VectorSpec{}
		}
		merged.Vector.Config = config
	}
	return merged
}

func (in *TelemetrySpec) IsPrometheusEnabled() bool {
	return in != nil && in.Prometheus != nil && in.Prometheus.Enabled != nil && *in.Prometheus.Enabled
}

func (in *TelemetrySpec) IsMcacEnabled() bool {
	return in != nil && in.Mcac != nil && in.Mcac.Enabled != nil && *in.Mcac.Enabled
}

func (in *TelemetrySpec) IsVectorEnabled() bool {
	return in != nil && in.Vector != nil && in.Vector.Enabled != nil && *in.Vector.Enabled
}

func vectorComponents(spec *TelemetrySpec) *VectorComponentsSpec {
	if spec == nil || spec.Vector == nil {
		return nil
	}
	return spec.Vector.Components
}

func vectorConfig(spec *TelemetrySpec) string {
	if spec == nil || spec.Vector == nil {
		return ""
	}
	return spec.Vector.Config
}

func mergeVectorConfig(cluster, dc string) string {
	switch {
	case cluster == "":
		return dc
	case dc == "":
		return cluster
	default:
		// This is to ensure our TOMLs have enough separation
		return strings.TrimRight(cluster, "\n") + "\n" + strings.TrimLeft(dc, "\n")
	}
}

func mergeVectorComponents(cluster, dc *VectorComponentsSpec) *VectorComponentsSpec {
	if cluster == nil && dc == nil {
		return nil
	}

	merged := &VectorComponentsSpec{}
	if cluster != nil {
		merged.Sources = append(merged.Sources, cluster.Sources...)
		merged.Sinks = append(merged.Sinks, cluster.Sinks...)
		merged.Transforms = append(merged.Transforms, cluster.Transforms...)
	}
	if dc != nil {
		merged.Sources = append(merged.Sources, dc.Sources...)
		merged.Sinks = append(merged.Sinks, dc.Sinks...)
		merged.Transforms = append(merged.Transforms, dc.Transforms...)
	}
	return merged
}
