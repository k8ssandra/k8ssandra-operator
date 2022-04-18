/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reaper

import (
	"context"
	"errors"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"

	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReaperReconciler) reconcileReaperTelemetry(
	ctx context.Context,
	thisReaper *reaperapi.Reaper,
	logger logr.Logger,
	remoteClient client.Client,
) error {
	logger.Info("reconciling telemetry", "reaper", thisReaper.Name)
	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   thisReaper.Namespace,
		MonitoringTargetName: thisReaper.Name,
		ServiceMonitorName:   GetReaperPromSMName(thisReaper.Name),
		Logger:               logger,
		CommonLabels:         mustLabels(thisReaper.Name),
	}
	klusterName, ok := thisReaper.Labels[k8ssandraapi.K8ssandraClusterNameLabel]
	if ok {
		cfg.CommonLabels[k8ssandraapi.K8ssandraClusterNameLabel] = klusterName
	}
	// Confirm telemetry config is valid (e.g. Prometheus is installed if it is requested.)
	promInstalled, err := telemetry.IsPromInstalled(remoteClient, logger)
	if err != nil {
		return err
	}
	validConfig := telemetry.SpecIsValid(thisReaper.Spec.Telemetry, promInstalled)
	if err != nil {
		return errors.New("could not determine if telemetry config is valid")
	}
	if !validConfig {
		return errors.New("telemetry spec was invalid for this cluster - is Prometheus installed if you have requested it")
	}
	// If Prometheus not installed bail here.
	if !promInstalled {
		return nil
	}
	// If Reaper is attached
	// Determine if we want a cleanup or a resource update.
	if thisReaper.Spec.Telemetry.IsPrometheusEnabled() {
		logger.Info("Prometheus config found", "TelemetrySpec", thisReaper.Spec.Telemetry)
		desiredSM, err := cfg.NewReaperServiceMonitor()
		if err != nil {
			return err
		}
		if err := cfg.UpdateResources(ctx, remoteClient, thisReaper, &desiredSM); err != nil {
			return err
		}
	} else {
		logger.Info("Telemetry not present for Reaper, will delete resources", "TelemetrySpec", thisReaper.Spec.Telemetry)
		if err := cfg.CleanupResources(ctx, remoteClient); err != nil {
			return err
		}
	}
	return nil
}

// mustLabels() returns the set of labels essential to managing the Prometheus resources. These should not be overwritten by the user.
func mustLabels(reaperName string) map[string]string {
	return map[string]string{
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		reaperapi.ReaperLabel:       reaperName,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelTelemetry,
		k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
	}
}

// GetReaperPromSMName gets the name for our ServiceMonitors based on cluster and DC name.
func GetReaperPromSMName(reaperName string) string {
	return reaperName + "-" + "reaper-servicemonitor"
}
