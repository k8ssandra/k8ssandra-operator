package telemetry

import (
	"strings"

	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type CassandraSMConfig struct {
	CassandraNamespace      *string
	ServiceMonitorName      *string
	ServiceMonitorNameSpace *string
	DataCenterName          *string
	CommonLabels            map[string]string
	ClusterName             *string
}

// Static configuration for ServiceMonitor's endpoints.
var endpointsString = `- port: prometheus
interval: 15s
path: /metrics
scheme: http
scrapeTimeout: 15s
targetPort: 9103
metricRelabelings:
- regex: collectd_mcac_(meter|histogram).*
  sourceLabels:
  - __name__
  replacement: ${1}
  targetLabel: kind
- action: drop
  regex: .*rate_(mean|1m|5m|15m)
  sourceLabels:
  - __name__
- regex: (collectd_mcac_.+)
  replacement: ${1}
  sourceLabels:
  - __name__
  targetLabel: prom_name
- regex: .+_bucket_(\d+)
  replacement: ${1}
  sourceLabels:
  - prom_name
  targetLabel: le
- regex: .+_bucket_inf
  replacement: +Inf
  sourceLabels:
  - prom_name
  targetLabel: le
- regex: .*_histogram_p(\d+)
  replacement: .${1}
  sourceLabels:
  - prom_name
  targetLabel: quantile
- regex: .*_histogram_min
  replacement: '0'
  sourceLabels:
  - prom_name
  targetLabel: quantile
- regex: .*_histogram_max
  replacement: '1'
  sourceLabels:
  - prom_name
  targetLabel: quantile
- action: drop
  regex: org\.apache\.cassandra\.metrics\.table\.(\w+)
  sourceLabels:
  - mcac
- regex: org\.apache\.cassandra\.metrics\.table\.(\w+)\.(\w+)\.(\w+)
  replacement: ${3}
  sourceLabels:
  - mcac
  targetLabel: table
- regex: org\.apache\.cassandra\.metrics\.table\.(\w+)\.(\w+)\.(\w+)
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: keyspace
- regex: org\.apache\.cassandra\.metrics\.table\.(\w+)\.(\w+)\.(\w+)
  replacement: mcac_table_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.keyspace\.(\w+)\.(\w+)
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: keyspace
- regex: org\.apache\.cassandra\.metrics\.keyspace\.(\w+)\.(\w+)
  replacement: mcac_keyspace_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.thread_pools\.(\w+)\.(\w+)\.(\w+).*
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: pool_type
- regex: org\.apache\.cassandra\.metrics\.thread_pools\.(\w+)\.(\w+)\.(\w+).*
  replacement: ${3}
  sourceLabels:
  - mcac
  targetLabel: pool_name
- regex: org\.apache\.cassandra\.metrics\.thread_pools\.(\w+)\.(\w+)\.(\w+).*
  replacement: mcac_thread_pools_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.client_request\.(\w+)\.(\w+)$
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: request_type
- regex: org\.apache\.cassandra\.metrics\.client_request\.(\w+)\.(\w+)$
  replacement: mcac_client_request_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.client_request\.(\w+)\.(\w+)\.(\w+)$
  replacement: ${3}
  sourceLabels:
  - mcac
  targetLabel: cl
- regex: org\.apache\.cassandra\.metrics\.client_request\.(\w+)\.(\w+)\.(\w+)$
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: request_type
- regex: org\.apache\.cassandra\.metrics\.client_request\.(\w+)\.(\w+)\.(\w+)$
  replacement: mcac_client_request_${1}_cl
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.cache\.(\w+)\.(\w+)
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: cache_name
- regex: org\.apache\.cassandra\.metrics\.cache\.(\w+)\.(\w+)
  replacement: mcac_cache_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.cql\.(\w+)
  replacement: mcac_cql_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.dropped_message\.(\w+)\.(\w+)
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: message_type
- regex: org\.apache\.cassandra\.metrics\.dropped_message\.(\w+)\.(\w+)
  replacement: mcac_dropped_message_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.streaming\.(\w+)\.(.+)$
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: peer_ip
- regex: org\.apache\.cassandra\.metrics\.streaming\.(\w+)\.(.+)$
  replacement: mcac_streaming_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.streaming\.(\w+)$
  replacement: mcac_streaming_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.commit_log\.(\w+)
  replacement: mcac_commit_log_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.compaction\.(\w+)
  replacement: mcac_compaction_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.storage\.(\w+)
  replacement: mcac_storage_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.batch\.(\w+)
  replacement: mcac_batch_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.client\.(\w+)
  replacement: mcac_client_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.buffer_pool\.(\w+)
  replacement: mcac_buffer_pool_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.index\.(\w+)
  replacement: mcac_sstable_index_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.hinted_hand_off_manager\.([^\-]+)-(\w+)
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: peer_ip
- regex: org\.apache\.cassandra\.metrics\.hinted_hand_off_manager\.([^\-]+)-(\w+)
  replacement: mcac_hints_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.hints_service\.hints_delays\-(\w+)
  replacement: ${1}
  sourceLabels:
  - mcac
  targetLabel: peer_ip
- regex: org\.apache\.cassandra\.metrics\.hints_service\.hints_delays\-(\w+)
  replacement: mcac_hints_hints_delays
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.hints_service\.([^\-]+)
  replacement: mcac_hints_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.memtable_pool\.(\w+)
  replacement: mcac_memtable_pool_${1}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: com\.datastax\.bdp\.type\.performance_objects\.name\.cql_slow_log\.metrics\.queries_latency
  replacement: mcac_cql_slow_log_query_latency
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: org\.apache\.cassandra\.metrics\.read_coordination\.(.*)
  replacement: $1
  sourceLabels:
  - mcac
  targetLabel: read_type
- regex: jvm\.gc\.(\w+)\.(\w+)
  replacement: ${1}
  sourceLabels:
  - mcac
  targetLabel: collector_type
- regex: jvm\.gc\.(\w+)\.(\w+)
  replacement: mcac_jvm_gc_${2}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: jvm\.memory\.(\w+)\.(\w+)
  replacement: ${1}
  sourceLabels:
  - mcac
  targetLabel: memory_type
- regex: jvm\.memory\.(\w+)\.(\w+)
  replacement: mcac_jvm_memory_${2}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: jvm\.memory\.pools\.(\w+)\.(\w+)
  replacement: ${2}
  sourceLabels:
  - mcac
  targetLabel: pool_name
- regex: jvm\.memory\.pools\.(\w+)\.(\w+)
  replacement: mcac_jvm_memory_pool_${2}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: jvm\.fd\.usage
  replacement: mcac_jvm_fd_usage
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: jvm\.buffers\.(\w+)\.(\w+)
  replacement: ${1}
  sourceLabels:
  - mcac
  targetLabel: buffer_type
- regex: jvm\.buffers\.(\w+)\.(\w+)
  replacement: mcac_jvm_buffer_${2}
  sourceLabels:
  - mcac
  targetLabel: __name__
- regex: (mcac_.*);.*(_micros_bucket|_bucket|_micros_count_total|_count_total|_total|_micros_sum|_sum|_stddev).*
  replacement: ${1}${2}
  separator: ;
  sourceLabels:
  - __name__
  - prom_name
  targetLabel: __name__
- action: labeldrop
  regex: prom_name
`

// NewServiceMonitor returns a Prometheus operator ServiceMonitor resource.
func (cfg CassandraSMConfig) NewServiceMonitor() (promapi.ServiceMonitor, error) {
	// validate the object we're being passed.
	if cfg.CassandraNamespace == nil || cfg.ServiceMonitorName == nil || cfg.ServiceMonitorNameSpace == nil || cfg.DataCenterName == nil || cfg.ClusterName == nil {
		return promapi.ServiceMonitor{}, TelemetryConfigIncomplete{}
	}
	var endpointsObject []promapi.Endpoint
	err := yaml.NewYAMLOrJSONDecoder(strings.NewReader(endpointsString), 4).Decode(endpointsObject)
	if err != nil {
		return promapi.ServiceMonitor{}, err
	}
	sm := promapi.ServiceMonitor{
		metav1.TypeMeta{},
		metav1.ObjectMeta{
			Name:      *cfg.ServiceMonitorName,
			Namespace: *cfg.ServiceMonitorNameSpace,
			Labels:    cfg.CommonLabels,
		},
		promapi.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/managed-by": "cass-operator",
					//It appears that this label is always set true by cass-operator, so I don't think filtering on it adds any value and we should look to deprecate.
					//"cassandra.datastax.com/prom-metrics": "true",
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "cassandra.datastax.com/cluster", Operator: "In", Values: []string{*cfg.ClusterName}},
					{Key: "cassandra.datastax.com/datacenter", Operator: "In", Values: []string{*cfg.DataCenterName}},
				},
			},
			NamespaceSelector: promapi.NamespaceSelector{
				MatchNames: []string{*cfg.CassandraNamespace},
			},
			TargetLabels: []string{
				"cassandra.datastax.com/cluster",
				"cassandra.datastax.com/datacenter",
			},
		},
	}
	return sm, nil

}
