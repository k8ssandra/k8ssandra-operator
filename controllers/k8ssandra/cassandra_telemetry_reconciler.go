// Logic in this file reconciles telemetry resources into the state declared in the CRs.

package k8ssandra

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileCassandraDCTelemetry(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) result.ReconcileResult {
	logger.Info("reconciling telemetry")
	mergedSpec := kc.Spec.Cassandra.DatacenterOptions.Telemetry.Merge(dcTemplate.DatacenterOptions.Telemetry)
	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   actualDc.Namespace,
		MonitoringTargetName: actualDc.Name,
		ServiceMonitorName:   kc.Name + "-" + actualDc.Name + "-" + "cass-servicemonitor",
		Logger:               logger,
		CommonLabels:         mustLabels(kc.Name, kc.Namespace, actualDc.Name),
	}
	logger.Info("merged TelemetrySpec constructed", "mergedSpec", mergedSpec, "cluster", kc.Name)
	// Confirm telemetry config is valid (e.g. Prometheus is installed if it is requested.)
	promInstalled, err := telemetry.IsPromInstalled(remoteClient, logger)
	if err != nil {
		return result.Error(err)
	}
	validConfig, err := telemetry.SpecIsValid(mergedSpec, promInstalled)
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
	case mergedSpec == nil || mergedSpec.Prometheus == nil:
		logger.Info("Telemetry not present for CassDC, will delete resources", "mergedSpec", mergedSpec)
		if err := cfg.CleanupResources(ctx, remoteClient); err != nil {
			return result.Error(err)
		}
	case mergedSpec.Prometheus.IsEnabled():
		logger.Info("Prometheus config found", "mergedSpec", mergedSpec)
		desiredSM, err := cfg.NewCassServiceMonitor()
		if err != nil {
			return result.Error(err)
		}
		if err := cfg.UpdateResources(ctx, remoteClient, actualDc, &desiredSM); err != nil {
			return result.Error(err)
		}
	}
	return result.Continue()
}

// mustLabels() returns the set of labels essential to managing the Prometheus resources. These should not be overwritten by the user.
func mustLabels(klusterName string, klusterNamespace string, dcName string) map[string]string {
	return map[string]string{
		k8ssandraapi.ManagedByLabel:                 k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:                    k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterName,
		k8ssandraapi.DatacenterLabel:                dcName,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterNamespace,
		k8ssandraapi.ComponentLabel:                 k8ssandraapi.ComponentLabelTelemetry,
		k8ssandraapi.CreatedByLabel:                 k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
	}
}
