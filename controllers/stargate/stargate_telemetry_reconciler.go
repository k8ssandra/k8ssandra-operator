// Logic in this file reconciles telemetry resources into the state declared in the CRs.

package stargate

import (
	"context"
	"errors"

	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *StargateReconciler) reconcileStargateTelemetry(
	ctx context.Context,
	thisStargate *stargateapi.Stargate,
	logger logr.Logger,
	remoteClient client.Client,
) result.ReconcileResult {
	logger.Info("reconciling telemetry", "stargate", thisStargate.Name)
	dcCfg := telemetry.StargateTelemetryResourcer{
		StargateNamespace: thisStargate.Namespace,
		StargateName:      thisStargate.Name,
		Logger:            logger,
	}
	// Confirm telemetry config is valid (e.g. Prometheus is installed if it is requested.)
	promInstalled, err := telemetry.IsPromInstalled(remoteClient, logger)
	if err != nil {
		return result.Error(err)
	}
	validConfig, err := telemetry.SpecIsValid(thisStargate.Spec.Telemetry, promInstalled)
	if err != nil {
		return result.Error(errors.New("could not determine if telemetry config is valid"))
	}
	if !validConfig {
		return result.Error(errors.New("telemetry spec was invalid for this cluster - is Prometheus installed if you have requested it"))
	}
	// If Prometheus not installed bail here.
	if !promInstalled {
		return result.Continue()
	}
	// Determine if we want a cleanup or a resource update.
	switch {
	case thisStargate.Spec.Telemetry == nil:
		logger.Info("Telemetry not present for Stargate, will delete resources", "TelemetrySpec", thisStargate.Spec.Telemetry)
		if err := dcCfg.CleanupResources(ctx, remoteClient); err != nil {
			return result.Error(err)
		}
		return result.Continue()
	case thisStargate.Spec.Telemetry.Prometheus == nil:
		logger.Info("Telemetry not present for Stargate, will delete resources", "TelemetrySpec", thisStargate.Spec.Telemetry)
		if err := dcCfg.CleanupResources(ctx, remoteClient); err != nil {
			return result.Error(err)
		}
		return result.Continue()
	case thisStargate.Spec.Telemetry.Prometheus.Enabled:
		logger.Info("Prometheus config found", "TelemetrySpec", thisStargate.Spec.Telemetry)
		dcCfg.TelemetrySpec = thisStargate.Spec.Telemetry
		if err := dcCfg.UpdateResources(ctx, remoteClient, thisStargate); err != nil {
			return result.Error(err)
		}
		return result.Continue()
	default:
		logger.Info("Telemetry not present for Stargate, will delete resources", "mergedSpec", thisStargate.Spec.Telemetry)
		if err := dcCfg.CleanupResources(ctx, remoteClient); err != nil {
			return result.Error(err)
		}
		return result.Continue()
	}
}
