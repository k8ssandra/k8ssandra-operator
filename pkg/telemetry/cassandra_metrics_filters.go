package telemetry

import (
	"strings"

	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	v1 "k8s.io/api/core/v1"
)

var (
	DefaultFilters = []string{"deny:org.apache.cassandra.metrics.Table",
		"deny:org.apache.cassandra.metrics.table",
		"allow:org.apache.cassandra.metrics.table.live_ss_table_count",
		"allow:org.apache.cassandra.metrics.Table.LiveSSTableCount",
		"allow:org.apache.cassandra.metrics.table.live_disk_space_used",
		"allow:org.apache.cassandra.metrics.table.LiveDiskSpaceUsed",
		"allow:org.apache.cassandra.metrics.Table.Pending",
		"allow:org.apache.cassandra.metrics.Table.Memtable",
		"allow:org.apache.cassandra.metrics.Table.Compaction",
		"allow:org.apache.cassandra.metrics.table.read",
		"allow:org.apache.cassandra.metrics.table.write",
		"allow:org.apache.cassandra.metrics.table.range",
		"allow:org.apache.cassandra.metrics.table.coordinator",
		"allow:org.apache.cassandra.metrics.table.dropped_mutations"}
)

// InjectCassandraTelemetryFilters adds MCAC filters to the cassandra container as an env variable.
// If no filters are defined and telemetry was set in the CRD, the default filters are used.
func InjectCassandraTelemetryFilters(telemetrySpec *telemetry.TelemetrySpec, dcConfig *cassandra.DatacenterConfig) {
	if telemetrySpec == nil || telemetrySpec.Prometheus == nil {
		return
	}

	metricsFilters := make([]string, len(telemetrySpec.Prometheus.McacMetricFilters))
	containerIndex, containerFound := cassandra.FindContainer(dcConfig.PodTemplateSpec, "cassandra")

	if len(telemetrySpec.Prometheus.McacMetricFilters) > 0 {
		// Custom filters were defined in the CRD.
		metricsFilters = append(metricsFilters, telemetrySpec.Prometheus.McacMetricFilters...)
	} else {
		// No custom filters were defined in the CRD, using the defaults.
		metricsFilters = append(metricsFilters, DefaultFilters...)
	}

	if containerFound {
		dcConfig.PodTemplateSpec.Spec.Containers[containerIndex].Env = append(dcConfig.PodTemplateSpec.Spec.Containers[containerIndex].Env,
			v1.EnvVar{Name: "METRIC_FILTERS", Value: strings.Trim(strings.Join(metricsFilters, " "), " ")})
	}
}
