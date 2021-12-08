// Logic in this file reconciles telemetry resources into the state declared in the CRs.

package k8ssandra

import (
	"context"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileCassandraDCTelemetry(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) (ctrl.Result, error) {
	logger.Info("reconciling telemetry", "cluster", kc.Name, "datacenter", actualDc.Name)
	mergedSpec := kc.Spec.Cassandra.CassandraTelemetry.Merge(dcTemplate.CassandraTelemetry)
	dcCfg := telemetry.CassTelemetryResourcer{
		CassandraNamespace: kc.Namespace,
		DataCenterName:     actualDc.Name,
		ClusterName:        kc.Name,
	}
	logger.Info("merged TelemetrySpec constructed", "mergedSpec", mergedSpec, "cluster", kc.Name, "datacenter", actualDc.Name)
	switch {
	case mergedSpec == nil:
		logger.Info("Telemetry not present for CassDC, will delete resources", "datacenter", actualDc.Name)
		if err := dcCfg.CleanupResources(ctx, remoteClient); err != nil {
			return ctrl.Result{}, err
		}
	case mergedSpec.Prometheus == nil:
		logger.Info("Telemetry not present for CassDC, will delete resources", "datacenter", actualDc.Name)
		if err := dcCfg.CleanupResources(ctx, remoteClient); err != nil {
			return ctrl.Result{}, err
		}
	case mergedSpec.Prometheus.Enabled == nil:
		logger.Info("Telemetry not present for CassDC, will delete resources", "datacenter", actualDc.Name, "mergedSpec.Prometheus.Enabled", mergedSpec.Prometheus.Enabled)
		if err := dcCfg.CleanupResources(ctx, remoteClient); err != nil {
			return ctrl.Result{}, err
		}
	case *mergedSpec.Prometheus.Enabled:
		logger.Info("Prometheus config found", "datacenter", actualDc.Name, "mergedSpec.Prometheus.Enabled", mergedSpec.Prometheus.Enabled)
		dcCfg.TelemetrySpec = mergedSpec
		if err := dcCfg.UpdateResources(ctx, remoteClient, kc); err != nil {
			return ctrl.Result{}, err
		}
	default:
		logger.Info("Telemetry not present for CassDC, will delete resources", "datacenter", actualDc.Name)
		if err := dcCfg.CleanupResources(ctx, remoteClient); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
}

func (r *K8ssandraClusterReconciler) setCassandraTelemetryStatus() (ctrl.Result, error) {
	// TODO
	return ctrl.Result{}, nil
}
