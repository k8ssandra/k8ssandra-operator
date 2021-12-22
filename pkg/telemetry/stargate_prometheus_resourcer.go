// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"context"
	"fmt"
	"os"

	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
)

type StargatePrometheusResourcer struct {
	StargateTelemetryResourcer
	CommonLabels       map[string]string
	ServiceMonitorName string
}

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

var stargateEndpointHolder promapi.ServiceMonitor

func init() {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err := decode([]byte(stargateEndpointString), nil, &stargateEndpointHolder)
	if err != nil {
		fmt.Println("Fatal error initialising EndpointHolder in pks/telemetry/stargate_prometheus_resourcer.go", err)
		os.Exit(1)
	}
}

// mustLabels() returns the set of labels essential to managing the Prometheus resources. These should not be overwritten by the user.
func (cfg StargatePrometheusResourcer) mustLabels() map[string]string {
	return map[string]string{
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		stargateapi.StargateLabel:   cfg.StargateName,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelTelemetry,
		k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
	}
}

// NewServiceMonitor returns a Prometheus operator ServiceMonitor resource.
func (cfg StargatePrometheusResourcer) NewServiceMonitor() (*promapi.ServiceMonitor, error) {
	// validate the object we're being passed.
	if cfg.StargateNamespace == "" || cfg.ServiceMonitorName == "" || cfg.StargateName == "" {
		return nil, TelemetryConfigIncomplete{}
	}
	// Overwrite any CommonLabels the user has asked for if they conflict with the labels essential for the functioning of the operator.
	mergedLabels := utils.MergeMap(cfg.CommonLabels, cfg.mustLabels())
	stargateEndpointHolder.Spec.Endpoints[0].MetricRelabelConfigs[0].Replacement = cfg.StargateName
	sm := promapi.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.ServiceMonitorName,
			Namespace: cfg.StargateNamespace,
			Labels:    mergedLabels,
		},
		Spec: promapi.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					k8ssandraapi.ManagedByLabel: "k8ssandra-operator",
					stargateapi.StargateLabel:   cfg.StargateName,
				},
			},
			NamespaceSelector: promapi.NamespaceSelector{
				MatchNames: []string{cfg.StargateNamespace},
			},
			Endpoints: stargateEndpointHolder.Spec.Endpoints,
		},
	}
	annotations.AddHashAnnotation(&sm)
	return &sm, nil
}

// UpdateResources executes the creation of the desired Prometheus resources on the cluster.
func (cfg StargatePrometheusResourcer) UpdateResources(ctx context.Context, client runtimeclient.Client, owner *stargateapi.Stargate) error {
	desiredSM, err := cfg.NewServiceMonitor()
	if err != nil {
		return err
	}
	cfg.StargateTelemetryResourcer.Logger.Info("checking whether Prometheus ServiceMonitor for Stargate already exists")
	// Logic to handle case where SM does not exist.
	actualSM := &promapi.ServiceMonitor{}
	if err := client.Get(ctx, types.NamespacedName{Name: desiredSM.Name, Namespace: desiredSM.Namespace}, actualSM); err != nil {
		if errors.IsNotFound(err) {
			cfg.StargateTelemetryResourcer.Logger.Info("Prometheus ServiceMonitor for Cassandra not found, creating")
			if err := controllerutil.SetControllerReference(owner, desiredSM, client.Scheme()); err != nil {
				cfg.StargateTelemetryResourcer.Logger.Error(err, "could not set controller reference for ServiceMonitor", "owner", owner)
				return err
			} else if err = client.Create(ctx, desiredSM); err != nil {
				if errors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created already; simply requeue until the cache is up-to-date
					return nil
				} else {
					cfg.StargateTelemetryResourcer.Logger.Error(err, "could not create ServiceMonitor resource", "resource", desiredSM, "owner", owner)
					return err
				}
			}
			return nil
		} else {
			cfg.StargateTelemetryResourcer.Logger.Error(err, "could not get ServiceMonitor resource")
			return err
		}
	}
	// Logic to handle case where SM exists, but is in the wrong state.
	actualSM = actualSM.DeepCopy()
	if !annotations.CompareAnnotations(actualSM, desiredSM, k8ssandraapi.ResourceHashAnnotation) {
		resourceVersion := actualSM.GetResourceVersion()
		desiredSM.DeepCopyInto(actualSM)
		actualSM.SetResourceVersion(resourceVersion)
		if err := controllerutil.SetControllerReference(owner, actualSM, client.Scheme()); err != nil {
			cfg.StargateTelemetryResourcer.Logger.Error(err, "could not set controller reference for ServiceMonitor", "resource", desiredSM, "owner", owner)
			return err
		} else if err := client.Update(ctx, actualSM); err != nil {
			cfg.StargateTelemetryResourcer.Logger.Error(err, "could not update ServiceMonitor resource", "resource", desiredSM, "owner", owner)
			return err
		} else {
			cfg.StargateTelemetryResourcer.Logger.Info("successfully updated the Stargate ServiceMonitor")
			return nil
		}
	}
	return nil
}

// CleanupResources executes the cleanup of any resources on the cluster, once they are no longer required.
// CleanupResources does not rely on user-defined CommonLabels to select which resources to delete. It uses only
// k8ssandra-operator defined labels.
func (cfg StargatePrometheusResourcer) CleanupResources(ctx context.Context, client runtimeclient.Client) error {
	var deleteTargets promapi.ServiceMonitor
	err := client.DeleteAllOf(ctx,
		&deleteTargets,
		runtimeclient.InNamespace(cfg.StargateNamespace),
		runtimeclient.MatchingLabels(cfg.mustLabels()),
	)
	if err != nil {
		return err
	}
	return nil
}
