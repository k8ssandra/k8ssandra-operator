// Package v1alpha1: Types in this package are instantiated in the other types in k8ssandra-operator, especially Stargate types and Cassandra types.
// +kubebuilder:object:generate=true
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TelemetrySpec struct {
	Prometheus *PrometheusTelemetrySpec `json:"prometheus,omitempty"`
	Mcac       *McacTelemetrySpec       `json:"mcac,omitempty"`
	Vector     *VectorSpec              `json:"vector,omitempty"`
}

type PrometheusTelemetrySpec struct {
	// Enable the creation of Prometheus serviceMonitors for this resource (Cassandra or Stargate).
	Enabled *bool `json:"enabled,omitempty"`
	// CommonLabels are applied to all serviceMonitors created.
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
}

type VectorSpec struct {
	// Enabled enables the Vector agent for this resource (Cassandra, Reaper or Stargate).
	// Enabling the vector agent will inject a sidecar container into the pod.
	Enabled *bool `json:"enabled,omitempty"`

	// Resources is the resource requirements for the Vector agent.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Image is the name of the Vector image to use. If not set, the default image will be used.
	// +optional
	// kube:default="timberio/vector:0.26.0-alpine"
	Image string `json:"image,omitempty"`

	// ScrapeInterval is the interval at which the Vector agent will scrape the metrics endpoint.
	// Use values like 30s, 1m, 5m.
	// +optional
	// kube:default=30s
	ScrapeInterval *metav1.Duration `json:"scrapeInterval,omitempty"`

	Components *VectorComponentsSpec `json:"components,omitempty"`
}

type VectorComponentsSpec struct {
	// Sources is the list of sources to use for the Vector agent.
	// +optional
	Sources []VectorSourceSpec `json:"sources,omitempty"`

	// Sinks is the list of sinks to use for the Vector agent.
	// +optional
	Sinks []VectorSinkSpec `json:"sinks,omitempty"`

	// Transforms is the list of transforms to use for the Vector agent.
	// +optional
	Transforms []VectorTransformSpec `json:"transforms,omitempty"`
}

type VectorSourceSpec struct {
	// Name is the name of the source.
	Name string `json:"name"`

	// Type is the type of the source.
	Type string `json:"type"`

	// Config is the configuration for the source.
	// +optional
	Config string `json:"config,omitempty"`
}

type VectorSinkSpec struct {
	// Name is the name of the sink.
	Name string `json:"name"`

	// Type is the type of the sink.
	Type string `json:"type"`

	// Inputs is the list of inputs for the transform.
	// +optional
	Inputs []string `json:"inputs,omitempty"`

	// Config is the configuration for the sink.
	// +optional
	Config string `json:"config,omitempty"`
}

type VectorTransformSpec struct {
	// Name is the name of the transform.
	Name string `json:"name"`

	// Type is the type of the transform.
	Type string `json:"type"`

	// Inputs is the list of inputs for the transform.
	// +optional
	Inputs []string `json:"inputs,omitempty"`

	// Config is the configuration for the transform.
	// +optional
	Config string `json:"config,omitempty"`
}

type McacTelemetrySpec struct {
	// MetricFilters allows passing filters to MCAC in order to reduce the amount of extracted metrics.
	// Not setting this field will result in the default filters being used:
	// - "deny:org.apache.cassandra.metrics.Table"
	// - "deny:org.apache.cassandra.metrics.table"
	// - "allow:org.apache.cassandra.metrics.table.live_ss_table_count"
	// - "allow:org.apache.cassandra.metrics.Table.LiveSSTableCount"
	// - "allow:org.apache.cassandra.metrics.table.live_disk_space_used"
	// - "allow:org.apache.cassandra.metrics.table.LiveDiskSpaceUsed"
	// - "allow:org.apache.cassandra.metrics.Table.Pending"
	// - "allow:org.apache.cassandra.metrics.Table.Memtable"
	// - "allow:org.apache.cassandra.metrics.Table.Compaction"
	// - "allow:org.apache.cassandra.metrics.table.read"
	// - "allow:org.apache.cassandra.metrics.table.write"
	// - "allow:org.apache.cassandra.metrics.table.range"
	// - "allow:org.apache.cassandra.metrics.table.coordinator"
	// - "allow:org.apache.cassandra.metrics.table.dropped_mutations"
	// Setting it to an empty list will result in all metrics being extracted.
	// +optional
	MetricFilters *[]string `json:"metricFilters,omitempty"`
}
