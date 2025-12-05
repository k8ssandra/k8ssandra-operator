// Logic in this file reconciles telemetry resources into the state declared in the CRs.

package k8ssandra

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileCassandraDCTelemetry(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcTemplate k8ssandraapi.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) result.ReconcileResult {
	logger.V(1).Info("reconciling telemetry")

	mergedSpec := MergeTelemetrySpecs(kc, dcTemplate)
	commonLabels := make(map[string]string)

	if mergedSpec != nil && mergedSpec.Prometheus != nil {
		commonLabels = mergedSpec.Prometheus.CommonLabels
	}

	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   actualDc.Namespace,
		MonitoringTargetName: actualDc.Name,
		ServiceMonitorName:   cassdcapi.CleanupForKubernetes(kc.CassClusterName() + "-" + actualDc.LabelResourceName() + "-" + "cass-servicemonitor"),
		Logger:               logger,
		CommonLabels:         mustLabels(kc.Name, kc.Namespace, actualDc.LabelResourceName(), commonLabels),
	}

	logger.V(1).Info("merged TelemetrySpec constructed", "mergedSpec", mergedSpec, "cluster", kc.Name)

	// Confirm telemetry config is valid (e.g. Prometheus is installed if it is requested.)
	promInstalled, err := telemetry.IsPromInstalled(remoteClient, logger)
	if err != nil {
		return result.Error(err)
	}

	validConfig := telemetry.SpecIsValid(mergedSpec, promInstalled)
	if !validConfig {
		return result.Error(errors.New("telemetry spec was invalid for this cluster - is Prometheus installed if you have requested it"))
	}

	// The new metrics endpoint is available since 3.11.13 and 4.0.4.
	// If MCAC is disabled and the new metrics endpoint is not available then bail here.
	if !mergedSpec.IsMcacEnabled() && !telemetry.IsNewMetricsEndpointAvailable(actualDc.Spec.ServerVersion) && kc.Spec.Cassandra.ServerType == k8ssandraapi.ServerDistributionCassandra {
		return result.Error(errors.New("new metrics endpoint is only available since Cassandra 3.11.13/4.0.4, so MCAC cannot be disabled"))
	}

	// If Prometheus not installed bail here.
	if !promInstalled {
		return result.Continue()
	}

	// Determine if we want a cleanup or a resource update.
	if mergedSpec.IsPrometheusEnabled() {
		logger.V(1).Info("Prometheus config found", "mergedSpec", mergedSpec)
		desiredSM, err := cfg.NewCassServiceMonitor(mergedSpec.IsMcacEnabled())
		if err != nil {
			return result.Error(err)
		}
		if err := cfg.UpdateResources(ctx, remoteClient, actualDc, desiredSM); err != nil {
			return result.Error(err)
		}
	} else {
		logger.Info("Telemetry not enabled for CassDC, will delete resources", "mergedSpec", mergedSpec)
		if err := cfg.CleanupResources(ctx, remoteClient); err != nil {
			return result.Error(err)
		}
	}

	return result.Continue()
}

// mustLabels() returns the set of labels essential to managing the Prometheus resources. These should not be overwritten by the user.
func mustLabels(klusterName string, klusterNamespace string, dcName string, additionalLabels map[string]string) map[string]string {
	if additionalLabels == nil {
		additionalLabels = make(map[string]string)
	}
	additionalLabels[k8ssandraapi.ManagedByLabel] = k8ssandraapi.NameLabelValue
	additionalLabels[k8ssandraapi.PartOfLabel] = k8ssandraapi.PartOfLabelValue
	additionalLabels[k8ssandraapi.K8ssandraClusterNameLabel] = klusterName
	additionalLabels[k8ssandraapi.DatacenterLabel] = dcName
	additionalLabels[k8ssandraapi.K8ssandraClusterNamespaceLabel] = klusterNamespace
	additionalLabels[k8ssandraapi.ComponentLabel] = k8ssandraapi.ComponentLabelTelemetry
	return additionalLabels
}

// MergeTelemetrySpecs merges the cluster and dc level telemetry specs, prioritizing the dc level spec.
func MergeTelemetrySpecs(kc *k8ssandraapi.K8ssandraCluster, dcTemplate k8ssandraapi.CassandraDatacenterTemplate) *v1alpha1.TelemetrySpec {
	clusterSpec := kc.Spec.Cassandra.DatacenterOptions.Telemetry
	dcSpec := dcTemplate.DatacenterOptions.Telemetry
	mergedSpec := dcSpec.MergeWith(clusterSpec)
	return mergedSpec
}
