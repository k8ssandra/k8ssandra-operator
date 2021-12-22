package telemetry

import (
	"fmt"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
)

// Static configuration for ServiceMonitor's endpoints.
const stargateEndpointString = `
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  endpoints:
    - interval: 15s
      path: /metrics
      port: health
      scheme: http
      scrapeTimeout: 15s
      metricRelabelings:
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_.*
        replacement: dc_name_goes_here
        sourceLabels:
        - __name__
        targetLabel: dc
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_.*
        replacement: k8ssandra
        sourceLabels:
        - __name__
        targetLabel: cluster
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_.*
        replacement: default
        sourceLabels:
        - __name__
        targetLabel: rack
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_(\w+)_Read.*
        replacement: read
        sourceLabels:
        - __name__
        targetLabel: request_type
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_(\w+)_Write.*
        replacement: write
        sourceLabels:
        - __name__
        targetLabel: request_type
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_(\w+)_CASRead.*
        replacement: cas_read
        sourceLabels:
        - __name__
        targetLabel: request_type
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_(\w+)_CASWrite.*
        replacement: cas_write
        sourceLabels:
        - __name__
        targetLabel: request_type
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_(\w+)_RangeSlice.*
        replacement: range_slice
        sourceLabels:
        - __name__
        targetLabel: request_type
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_(\w+)_ViewWrite.*
        replacement: view_write
        sourceLabels:
        - __name__
        targetLabel: request_type
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_Latency_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)_count
        replacement: stargate_client_request_latency_total
        sourceLabels:
        - __name__
        targetLabel: __name__
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_Latency_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)
        replacement: stargate_client_request_latency_quantile
        sourceLabels:
        - __name__
        targetLabel: __name__
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_Failures_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)_total
        replacement: stargate_client_request_failures_total
        sourceLabels:
        - __name__
        targetLabel: __name__
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_Timeouts_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)_total
        replacement: stargate_client_request_timeouts_total
        sourceLabels:
        - __name__
        targetLabel: __name__
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_Unavailables_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)_total
        replacement: stargate_client_request_unavailables_total
        sourceLabels:
        - __name__
        targetLabel: __name__
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_ConditionNotMet_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)
        replacement: stargate_client_request_condition_not_met_total
        sourceLabels:
        - __name__
        targetLabel: __name__
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_UnfinishedCommit_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)
        replacement: stargate_client_request_unfinished_commit_total
        sourceLabels:
        - __name__
        targetLabel: __name__
      - regex: persistence_cassandra_(\d_\d+)_org_apache_cassandra_metrics_ClientRequest_ContentionHistogram_(Read|Write|CASRead|CASWrite|RangeSlice|ViewWrite)_count
        replacement: stargate_client_request_contention_histogran_total
        sourceLabels:
        - __name__
        targetLabel: __name__
`

var stargateServiceMonitorTemplate = &promapi.ServiceMonitor{}

func init() {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err := decode([]byte(stargateEndpointString), nil, stargateServiceMonitorTemplate)
	if err != nil {
		fmt.Println("Fatal error initialising EndpointHolder in pks/telemetry/prom_stargate_servicemonitor.go", err)
		os.Exit(1)
	}
}
