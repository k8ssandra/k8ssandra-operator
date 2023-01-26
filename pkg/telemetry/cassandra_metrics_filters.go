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
// If filter list is set to nil, the default filters are used, otherwise the provided filters are used.
func InjectCassandraTelemetryFilters(telemetrySpec *telemetry.TelemetrySpec, dcConfig *cassandra.DatacenterConfig) {
	filtersEnvVar := v1.EnvVar{}
	if telemetrySpec == nil || telemetrySpec.Mcac == nil || telemetrySpec.Mcac.MetricFilters == nil {
		// Default filters are applied
		filtersEnvVar = v1.EnvVar{Name: "METRIC_FILTERS", Value: strings.Join(DefaultFilters, " ")}
	} else {
		// Custom filters are applied
		filtersEnvVar = v1.EnvVar{Name: "METRIC_FILTERS", Value: strings.Join(*telemetrySpec.Mcac.MetricFilters, " ")}
	}
	cassandra.UpdateCassandraContainer(&dcConfig.PodTemplateSpec, func(container *v1.Container) {
		container.Env = append(container.Env, filtersEnvVar)
	})
}
