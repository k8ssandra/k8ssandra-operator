package telemetry

import (
	"fmt"
	"os"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

// Static configuration for ServiceMonitor's endpoints when using the legacy MCAC stack (Metrics
// Collector for Apache Cassandra).  This configuration will monitor the legacy 'prometheus' port of
// the DC's all-pods service, whose number is 9103.
const endpointStringLegacy = `
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  endpoints:
  - port: prometheus
    interval: 15s
    path: /metrics
    scheme: http
    scrapeTimeout: 15s
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
    - regex: org\.apache\.cassandra\.metrics\.cdc_agent\.(\w+)
      replacement: mcac_cdc_agent_${1}
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

// Static configuration for ServiceMonitor's endpoints, when using modern metrics endpoints. This
// configuration will monitor the 'metrics' port of the DC's all-pods service, whose number is 9000.
// Note that in this configuration it is not required to apply relabelings to the exposed metrics.
const endpointStringModern = `
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
    scheme: http
    scrapeTimeout: 15s
`

var (
	cassServiceMonitorTemplateLegacy = &promapi.ServiceMonitor{}
	cassServiceMonitorTemplateModern = &promapi.ServiceMonitor{}
)

func init() {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	if _, _, err := decode([]byte(endpointStringLegacy), nil, cassServiceMonitorTemplateLegacy); err != nil {
		fmt.Println("Fatal error initialising ServiceMonitor template", err)
		os.Exit(1)
	}
	if _, _, err := decode([]byte(endpointStringModern), nil, cassServiceMonitorTemplateModern); err != nil {
		fmt.Println("Fatal error initialising ServiceMonitor template", err)
		os.Exit(1)
	}
}

// NewCassServiceMonitor returns a Prometheus operator ServiceMonitor resource.
func (cfg PrometheusResourcer) NewCassServiceMonitor(legacyEndpoints bool) (*promapi.ServiceMonitor, error) {
	// validate the object we're being passed.
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	clusterName, ok := cfg.CommonLabels[k8ssandraapi.K8ssandraClusterNameLabel]
	if !ok {
		return nil, TelemetryConfigIncomplete{"PrometheusResourcer.CommonLabels[k8ssandraapi.K8ssandraClusterNameLabel]"}
	}
	var cassServiceMonitorTemplate *promapi.ServiceMonitor
	if legacyEndpoints {
		cassServiceMonitorTemplate = cassServiceMonitorTemplateLegacy.DeepCopy()
	} else {
		cassServiceMonitorTemplate = cassServiceMonitorTemplateModern.DeepCopy()
	}

	// Overwrite any CommonLabels the user has asked for if they conflict with the labels essential for the functioning of the operator.
	sm := &promapi.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.ServiceMonitorName,
			Namespace: cfg.MonitoringTargetNS,
			Labels:    cfg.CommonLabels,
		},
		Spec: promapi.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					k8ssandraapi.ManagedByLabel:           "cass-operator",
					"cassandra.datastax.com/prom-metrics": "true",
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: cassdcapi.ClusterLabel, Operator: "In", Values: []string{cassdcapi.CleanLabelValue(clusterName)}},
					{Key: cassdcapi.DatacenterLabel, Operator: "In", Values: []string{cfg.MonitoringTargetName}},
				},
			},
			NamespaceSelector: promapi.NamespaceSelector{
				MatchNames: []string{cfg.MonitoringTargetNS},
			},
			TargetLabels: []string{
				cassdcapi.ClusterLabel,
				cassdcapi.DatacenterLabel,
			},
			Endpoints: cassServiceMonitorTemplate.Spec.Endpoints,
		},
	}
	annotations.AddHashAnnotation(sm)
	return sm, nil
}
