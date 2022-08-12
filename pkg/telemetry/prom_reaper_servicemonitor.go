package telemetry

import (
	"fmt"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"os"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

// Static configuration for ServiceMonitor's endpoints.
const reaperEndpointString = `
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  endpoints:
    - interval: 15s
      path: /prometheusMetrics
      port: admin
      scheme: http
      scrapeTimeout: 15s
`

var reaperServiceMonitorTemplate = &promapi.ServiceMonitor{}

func init() {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err := decode([]byte(reaperEndpointString), nil, reaperServiceMonitorTemplate)
	if err != nil {
		fmt.Println("Fatal error initialising EndpointHolder in pks/telemetry/prom_reaper_servicemonitor.go", err)
		os.Exit(1)
	}
}

// NewReaperServiceMonitor returns a Prometheus operator ServiceMonitor resource.
func (cfg PrometheusResourcer) NewReaperServiceMonitor() (promapi.ServiceMonitor, error) {
	// validate the object we're being passed.
	if err := cfg.validate(); err != nil {
		return promapi.ServiceMonitor{}, err
	}
	// Overwrite any CommonLabels the user has asked for if they conflict with the labels essential for the functioning of the operator.
	sm := promapi.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.ServiceMonitorName,
			Namespace: cfg.MonitoringTargetNS,
			Labels:    cfg.CommonLabels,
		},
		Spec: promapi.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueReaperController,
					reaperapi.ReaperLabel:       cfg.MonitoringTargetName,
				},
			},
			NamespaceSelector: promapi.NamespaceSelector{
				MatchNames: []string{cfg.MonitoringTargetNS},
			},
			Endpoints: reaperServiceMonitorTemplate.Spec.Endpoints,
		},
	}
	annotations.AddHashAnnotation(&sm)
	return sm, nil
}
