// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"context"
	"strings"

	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

type CassPrometheusResourcer struct {
	CassTelemetryResourcer
	CommonLabels       map[string]string
	ServiceMonitorName string
}

// Static configuration for ServiceMonitor's endpoints.
var endpointString = `
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  endpoints:
  - port: prometheus
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
      regex: prom_name`

// mustLabels() returns the set of labels essential to managing the Prometheus resources. These should not be overwritten by the user.
func (cfg CassPrometheusResourcer) mustLabels() map[string]string {
	return map[string]string{
		k8ssandraapi.ManagedByLabel:            k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:               k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.K8ssandraClusterNameLabel: cfg.ClusterName,
		k8ssandraapi.DatacenterLabel:           cfg.DataCenterName,
		k8ssandraapi.ComponentLabel:            k8ssandraapi.ComponentLabelTelemetry,
		k8ssandraapi.CreatedByLabel:            k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
	}
}

// NewServiceMonitor returns a Prometheus operator ServiceMonitor resource.
func (cfg CassPrometheusResourcer) NewServiceMonitor() (*promapi.ServiceMonitor, error) {
	// validate the object we're being passed.
	if cfg.CassandraNamespace == "" || cfg.ServiceMonitorName == "" || cfg.DataCenterName == "" || cfg.ClusterName == "" {
		return nil, TelemetryConfigIncomplete{}
	}
	// Overwrite any CommonLabels the user has asked for if they conflict with the labels essential for the functioning of the operator.
	mergedLabels := utils.MergeMap(cfg.CommonLabels, cfg.mustLabels())
	var endpointHolder promapi.ServiceMonitor
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err := decode([]byte(endpointString), nil, &endpointHolder)
	if err != nil {
		return nil, err
	}
	sm := promapi.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.ServiceMonitorName,
			Namespace: cfg.CassandraNamespace,
			Labels:    mergedLabels,
		},
		Spec: promapi.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					k8ssandraapi.ManagedByLabel: "cass-operator",
					//It appears that this label is always set true by cass-operator, so I don't think filtering on it adds any value and we should look to deprecate.
					//"cassandra.datastax.com/prom-metrics": "true",
				},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: cassdcapi.ClusterLabel, Operator: "In", Values: []string{cfg.ClusterName}},
					{Key: cassdcapi.DatacenterLabel, Operator: "In", Values: []string{cfg.DataCenterName}},
				},
			},
			NamespaceSelector: promapi.NamespaceSelector{
				MatchNames: []string{cfg.CassandraNamespace},
			},
			TargetLabels: []string{
				cassdcapi.ClusterLabel,
				cassdcapi.DatacenterLabel,
			},
			Endpoints: endpointHolder.Spec.Endpoints,
		},
	}
	utils.AddHashAnnotation(&sm, k8ssandraapi.ResourceHashAnnotation)
	return &sm, nil
}

// GetCassandraPromSMName gets the name for our ServiceMonitors based on
func GetCassandraPromSMName(cfg CassTelemetryResourcer) string {
	return strings.Join([]string{cfg.ClusterName, cfg.DataCenterName, "cass-servicemonitor"}, "-")
}

// UpdateResources executes the creation of the desired Prometheus resources on the cluster.
func (cfg CassPrometheusResourcer) UpdateResources(ctx context.Context, client runtimeclient.Client, owner *k8ssandraapi.K8ssandraCluster) error {
	desiredSM, err := cfg.NewServiceMonitor()
	if err != nil {
		return err
	}
	cfg.CassTelemetryResourcer.Logger.Info("checking whether Prometheus ServiceMonitor for Cassandra already exists")
	// Logic to handle case where SM does not exist.
	actualSM := &promapi.ServiceMonitor{}
	if err := client.Get(ctx, types.NamespacedName{Name: desiredSM.Name, Namespace: desiredSM.Namespace}, actualSM); err != nil {
		if errors.IsNotFound(err) {
			cfg.CassTelemetryResourcer.Logger.Info("Prometheus ServiceMonitor for Cassandra not found, creating")
			if err := controllerutil.SetControllerReference(owner, desiredSM, client.Scheme()); err != nil {
				cfg.CassTelemetryResourcer.Logger.Error(err, "could not set controller reference for ServiceMonitor", "owner", owner)
				return err
			} else if err = client.Create(ctx, desiredSM); err != nil {
				if errors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created already; simply requeue until the cache is up-to-date
					return nil
				} else {
					cfg.CassTelemetryResourcer.Logger.Error(err, "could not create ServiceMonitor resource", "resource", desiredSM, "owner", owner)
					return err
				}
			}
			return nil
		} else {
			cfg.CassTelemetryResourcer.Logger.Error(err, "could not get ServiceMonitor resource")
			return err
		}
	}
	// Logic to handle case where SM exists, but is in the wrong state.
	actualSM = actualSM.DeepCopy()
	if !utils.CompareAnnotations(actualSM, desiredSM, k8ssandraapi.ResourceHashAnnotation) {
		resourceVersion := actualSM.GetResourceVersion()
		desiredSM.DeepCopyInto(actualSM)
		actualSM.SetResourceVersion(resourceVersion)
		if err := controllerutil.SetControllerReference(owner, actualSM, client.Scheme()); err != nil {
			cfg.CassTelemetryResourcer.Logger.Error(err, "could not set controller reference for ServiceMonitor", "resource", desiredSM, "owner", owner)
			return err
		} else if err := client.Update(ctx, actualSM); err != nil {
			cfg.CassTelemetryResourcer.Logger.Error(err, "could not update ServiceMonitor resource", "resource", desiredSM, "owner", owner)
			return err
		} else {
			cfg.CassTelemetryResourcer.Logger.Info("successfully updated the Cassandra ServiceMonitor")
			return nil
		}
	}
	return nil
}

// CleanupResources executes the cleanup of any resources on the cluster, once they are no longer required.
func (cfg CassPrometheusResourcer) CleanupResources(ctx context.Context, client runtimeclient.Client) error {
	var deleteTargets promapi.ServiceMonitorList
	if err := client.List(
		ctx,
		&deleteTargets,
		runtimeclient.InNamespace(cfg.CassandraNamespace),
		runtimeclient.MatchingLabels(cfg.mustLabels())); err != nil {
		return err
	}
	if len(deleteTargets.Items) > 0 {
		for _, i := range deleteTargets.Items {
			if err := client.Delete(ctx, i); err != nil {
				return err
			}
		}
	}
	return nil
}
