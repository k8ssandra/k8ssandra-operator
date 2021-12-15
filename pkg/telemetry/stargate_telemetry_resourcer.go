// This file contains the generic resource creation functions for Cassandra, to be called in the reconciliation logic. It does not contain logic to create specific types of
// telemetry resources, which should be in separate files.

package telemetry

import (
	"context"
	"github.com/go-logr/logr"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StargateTelemetryResourcer struct {
	StargateNamespace string
	StargateName      string
	TelemetrySpec     *telemetry.TelemetrySpec
	Logger            logr.Logger
}

// UpdateResources creates the required resources for a Cassandra DC and for the telemetry provider specified in the TelemetrySpec.
func (cfg StargateTelemetryResourcer) UpdateResources(ctx context.Context, client client.Client, owner *stargateapi.Stargate) error {
	if cfg.TelemetrySpec == nil {
		return TelemetryConfigIncomplete{"cfg.TelemetrySpec"}
	}
	// Check for Prometheus resources and instantiate if required.
	if cfg.TelemetrySpec.Prometheus != nil {
		if cfg.TelemetrySpec.Prometheus.Enabled {
			promResourcer := StargatePrometheusResourcer{
				StargateTelemetryResourcer: cfg,
				ServiceMonitorName:         GetStargatePromSMName(cfg),
				CommonLabels:               cfg.TelemetrySpec.Prometheus.CommonLabels,
			}
			if err := promResourcer.UpdateResources(ctx, client, owner); err != nil {
				cfg.Logger.Error(err, "failed to update prometheus resources")
				return err
			}
		}
	}
	return nil
}

// CleanupResources finds resources created by CassTelemetryResourcer and cleans them up.
func (cfg StargateTelemetryResourcer) CleanupResources(ctx context.Context, client client.Client) error {
	//Cleanup prometheus resources
	promResourcer := StargatePrometheusResourcer{
		StargateTelemetryResourcer: cfg,
		ServiceMonitorName:         GetStargatePromSMName(cfg),
	}
	if err := promResourcer.CleanupResources(ctx, client); err != nil {
		cfg.Logger.Error(err, "error cleaning up telemetry resources")
		return err
	}
	return nil
}

// GetStargatePromSMName gets the name for our ServiceMonitors based on cluster and DC name.
func GetStargatePromSMName(cfg StargateTelemetryResourcer) string {
	return cfg.StargateName + "-" + "stargate-servicemonitor"
}
