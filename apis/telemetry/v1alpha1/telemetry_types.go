// Package v1alpha1: Types in this package are instantiated in the other types in k8ssandra-operator, especially Stargate types and Cassandra types.
// +kubebuilder:object:generate=true
package v1alpha1

type TelemetrySpec struct {
	Prometheus *PrometheusTelemetrySpec `json:"prometheus,omitempty"`
	Mcac       *McacTelemetrySpec       `json:"mcac,omitempty"`
}

type PrometheusTelemetrySpec struct {
	// Enable the creation of Prometheus serviceMonitors for this resource (Cassandra or Stargate).
	Enabled *bool `json:"enabled,omitempty"`
	// CommonLabels are applied to all serviceMonitors created.
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
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
