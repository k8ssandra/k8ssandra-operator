// Logic in this file reconciles telemetry resources into the state declared in the CRs.

package stargate

import (
	"context"
	"errors"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"

	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *StargateReconciler) reconcileStargateTelemetry(
	ctx context.Context,
	thisStargate *stargateapi.Stargate,
	logger logr.Logger,
	remoteClient client.Client,
) (ctrl.Result, error) {
	logger.Info("reconciling telemetry", "stargate", thisStargate.Name)
	var commonLabels map[string]string
	if thisStargate.Spec.Telemetry == nil {
		commonLabels = make(map[string]string)
	} else if thisStargate.Spec.Telemetry.Prometheus == nil {
		commonLabels = make(map[string]string)
	} else {
		commonLabels = thisStargate.Spec.Telemetry.Prometheus.CommonLabels
	}

	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   thisStargate.Namespace,
		MonitoringTargetName: thisStargate.Name,
		ServiceMonitorName:   GetStargatePromSMName(thisStargate.Name),
		Logger:               logger,
		CommonLabels:         mustLabels(thisStargate.Name, commonLabels),
	}
	klusterName, ok := thisStargate.Labels[k8ssandraapi.K8ssandraClusterNameLabel]
	if ok {
		cfg.CommonLabels[k8ssandraapi.K8ssandraClusterNameLabel] = klusterName
	}
	// Confirm telemetry config is valid (e.g. Prometheus is installed if it is requested.)
	promInstalled, err := telemetry.IsPromInstalled(remoteClient, logger)
	if err != nil {
		return ctrl.Result{}, err
	}
	validConfig := telemetry.SpecIsValid(thisStargate.Spec.Telemetry, promInstalled)
	if !validConfig {
		return ctrl.Result{}, errors.New("telemetry spec was invalid for this cluster - is Prometheus installed if you have requested it")
	}
	// If Prometheus not installed bail here.
	if !promInstalled {
		return ctrl.Result{}, nil
	}
	// If Stargate is attached
	// Determine if we want a cleanup or a resource update.
	if thisStargate.Spec.Telemetry.IsPrometheusEnabled() {
		logger.Info("Prometheus config found", "TelemetrySpec", thisStargate.Spec.Telemetry)
		desiredSM, err := cfg.NewStargateServiceMonitor()
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := cfg.UpdateResources(ctx, remoteClient, thisStargate, &desiredSM); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		logger.Info("Telemetry not present for Stargate, will delete resources", "TelemetrySpec", thisStargate.Spec.Telemetry)
		if err := cfg.CleanupResources(ctx, remoteClient); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// mustLabels() returns the set of labels essential to managing the Prometheus resources. These should not be overwritten by the user.
func mustLabels(stargateName string, additionalLabels map[string]string) map[string]string {
	builtInLabels := labels.MapOf(
		labels.StargateCommon,
		labels.ManagedByStargateServiceMonitor(stargateName),
	)
	if additionalLabels == nil {
		additionalLabels = builtInLabels
	} else {
		for k, v := range builtInLabels {
			additionalLabels[k] = v
		}
	}
	return additionalLabels

}

// GetStargatePromSMName gets the name for our ServiceMonitors based on cluster and DC name.
func GetStargatePromSMName(stargateName string) string {
	return stargateName + "-" + "stargate-servicemonitor"
}
