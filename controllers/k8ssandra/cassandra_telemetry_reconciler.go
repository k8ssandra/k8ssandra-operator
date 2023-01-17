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
	corev1 "k8s.io/api/core/v1"
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
	clusterSpec := kc.Spec.Cassandra.DatacenterOptions.Telemetry
	dcSpec := dcTemplate.DatacenterOptions.Telemetry
	mergedSpec := dcSpec.MergeWith(clusterSpec)
	var commonLabels map[string]string
	if mergedSpec == nil {
		commonLabels = make(map[string]string)
	} else if mergedSpec.Prometheus == nil {
		commonLabels = make(map[string]string)
	} else {
		commonLabels = mergedSpec.Prometheus.CommonLabels
	}
	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   actualDc.Namespace,
		MonitoringTargetName: actualDc.Name,
		ServiceMonitorName:   kc.SanitizedName() + "-" + actualDc.Name + "-" + "cass-servicemonitor",
		Logger:               logger,
		CommonLabels:         mustLabels(kc.Name, kc.Namespace, actualDc.Name, commonLabels),
	}
	logger.Info("merged TelemetrySpec constructed", "mergedSpec", mergedSpec, "cluster", kc.Name)
	// Confirm telemetry config is valid (e.g. Prometheus is installed if it is requested.)
	promInstalled, err := telemetry.IsPromInstalled(remoteClient, logger)
	if err != nil {
		return result.Error(err)
	}
	validConfig := telemetry.SpecIsValid(mergedSpec, promInstalled)
	if !validConfig {
		return result.Error(errors.New("telemetry spec was invalid for this cluster - is Prometheus installed if you have requested it"))
	}
	// If Prometheus not installed bail here.
	if !promInstalled {
		return result.Continue()
	}
	// Determine if we want a cleanup or a resource update.
	if mergedSpec.IsPrometheusEnabled() {
		logger.Info("Prometheus config found", "mergedSpec", mergedSpec)
		desiredSM, err := cfg.NewCassServiceMonitor()
		if err != nil {
			return result.Error(err)
		}
		if err := cfg.UpdateResources(ctx, remoteClient, actualDc, &desiredSM); err != nil {
			return result.Error(err)
		}
	} else {
		logger.Info("Telemetry not enabled for CassDC, will delete resources", "mergedSpec", mergedSpec)
		if err := cfg.CleanupResources(ctx, remoteClient); err != nil {
			return result.Error(err)
		}
	}

	cassandraContainer := &corev1.Container{
		Name: "cassandra",
	}
	dcContainers := []corev1.Container{}
	if !(actualDc.Spec.PodTemplateSpec == nil || len(actualDc.Spec.PodTemplateSpec.Spec.Containers) == 0) {
		dcContainers = actualDc.Spec.PodTemplateSpec.Spec.Containers
	}
	for _, c := range dcContainers {
		if c.Name == "cassandra" {
			cassandraContainer = &c
		}
	}
	disabled := "false"
	if mergedSpec.IsMcacEnabled() {
		disabled = "true"
	}
	cassandraContainer.Env = append(
		cassandraContainer.Env,
		corev1.EnvVar{
			Name:  "MGMT_API_DISABLE_MCAC",
			Value: disabled,
		},
	)

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
	additionalLabels[k8ssandraapi.CreatedByLabel] = k8ssandraapi.CreatedByLabelValueK8ssandraClusterController
	return additionalLabels
}
