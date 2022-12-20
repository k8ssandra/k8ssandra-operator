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

	// Config is the name of the configmap containing custom sinks and transformers for the Vector agent.
	// The configmap must be in the same namespace as the CassandraDatacenter and contain a vector.toml entry
	// with the Vector configuration in toml format.
	// The agent is already configured with a "cassandra_metrics" source that needs to be used as input for the sinks.
	// If not set, the default console sink will be used.
	Config *corev1.LocalObjectReference `json:"config,omitempty"`

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
