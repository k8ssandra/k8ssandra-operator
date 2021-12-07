// This file contains the generic resource creation functions for Cassandra, to be called in the reconciliation logic. It does not contain logic to create specific types of
// telemetry resources, which should be in separate files.

package telemetry

import (
	"context"

	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resourcer interface {
	CreateResources(ctx context.Context, client client.Client) error
	CleanupResources(ctx context.Context, client client.Client) error
}

type CassTelemetryResourcer struct {
	CassandraNamespace string
	DataCenterName     string
	ClusterName        string
	TelemetrySpec      *telemetry.TelemetrySpec
}

// CreateResources creates the required resources for a Cassandra DC and for the telemetry provider specified in the TelemetrySpec.
func (cfg CassTelemetryResourcer) CreateResources(ctx context.Context, client client.Client) error {
	if cfg.TelemetrySpec == nil {
		return TelemetryConfigIncomplete{"cfg.TelemetrySpec"}
	}
	// Check for Prometheus resources and instantiate if required.
	if cfg.TelemetrySpec.Prometheus != nil {
		if *cfg.TelemetrySpec.Prometheus.Enabled {
			promResourcer := CassPrometheusResourcer{
				CassTelemetryResourcer: cfg,
				ServiceMonitorName:     GetCassandraPromSMName(cfg),
				CommonLabels:           cfg.TelemetrySpec.Prometheus.CommonLabels,
			}
			if err := promResourcer.CreateResources(ctx, client); err != nil {
				return err
			}
		}
	}
	return nil
}

// CleanupResources finds resources created by CassTelemetryResourcer and cleans them up.
func (cfg CassTelemetryResourcer) CleanupResources(ctx context.Context, client client.Client) error {
	//Cleanup prometheus resources
	promResourcer := CassPrometheusResourcer{
		CassTelemetryResourcer: cfg,
		ServiceMonitorName:     GetCassandraPromSMName(cfg),
		CommonLabels:           cfg.TelemetrySpec.Prometheus.CommonLabels,
	}
	if err := promResourcer.CleanupResources(ctx, client); err != nil {
		return err
	}
	return nil
}
