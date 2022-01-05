// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"context"
	"errors"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type PrometheusResourcer struct {
	MonitoringTargetNS   string
	MonitoringTargetName string
	ServiceMonitorName   string
	Logger               logr.Logger
	CommonLabels         map[string]string
}

func (cfg PrometheusResourcer) validate() error {
	if cfg.MonitoringTargetNS == "" || cfg.MonitoringTargetName == "" || cfg.ServiceMonitorName == "" || cfg.Logger == nil {
		return TelemetryConfigIncomplete{}
	}
	return nil
}

// UpdateResources executes the creation of the desired Prometheus resources on the cluster.
func (cfg PrometheusResourcer) UpdateResources(
	ctx context.Context,
	client runtimeclient.Client,
	owner metav1.Object,
	desiredSM *promapi.ServiceMonitor,
) error {
	if err := cfg.validate(); err != nil {
		return TelemetryConfigIncomplete{}
	}
	cfg.Logger.Info("checking whether Prometheus ServiceMonitor for Cassandra already exists")
	// Logic to handle case where SM does not exist.
	actualSM := &promapi.ServiceMonitor{}
	if err := client.Get(ctx, types.NamespacedName{Name: desiredSM.Name, Namespace: desiredSM.Namespace}, actualSM); err != nil {
		if k8serrors.IsNotFound(err) {
			cfg.Logger.Info("Prometheus ServiceMonitor for Cassandra not found, creating")
			if err := controllerutil.SetControllerReference(owner, desiredSM, client.Scheme()); err != nil {
				cfg.Logger.Error(err, "could not set controller reference for ServiceMonitor", "owner", owner)
				return err
			} else if err = client.Create(ctx, desiredSM); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created already; simply requeue until the cache is up-to-date
					return nil
				} else {
					cfg.Logger.Error(err, "could not create ServiceMonitor resource", "resource", desiredSM, "owner", owner)
					return err
				}
			}
			return nil
		} else {
			cfg.Logger.Error(err, "could not get ServiceMonitor resource")
			return err
		}
	}
	// Logic to handle case where SM exists, but is in the wrong state.
	actualSM = actualSM.DeepCopy()
	if !annotations.CompareHashAnnotations(actualSM, desiredSM) {
		resourceVersion := actualSM.GetResourceVersion()
		desiredSM.DeepCopyInto(actualSM)
		actualSM.SetResourceVersion(resourceVersion)
		if err := controllerutil.SetControllerReference(owner, actualSM, client.Scheme()); err != nil {
			cfg.Logger.Error(err, "could not set controller reference for ServiceMonitor", "resource", desiredSM, "owner", owner)
			return err
		} else if err := client.Update(ctx, actualSM); err != nil {
			cfg.Logger.Error(err, "could not update ServiceMonitor resource", "resource", desiredSM, "owner", owner)
			return err
		} else {
			cfg.Logger.Info("successfully updated the Cassandra ServiceMonitor")
			return nil
		}
	}
	return nil
}

// CleanupResources executes the cleanup of any resources on the cluster, once they are no longer required.
// CleanupResources does not rely on user-defined CommonLabels to select which resources to delete. It uses only
// k8ssandra-operator defined labels.
func (cfg PrometheusResourcer) CleanupResources(ctx context.Context, client runtimeclient.Client) error {
	if len(cfg.CommonLabels) < 1 {
		return errors.New("no labels were found to target deletion request")
	}
	if err := cfg.validate(); err != nil {
		return TelemetryConfigIncomplete{}
	}
	var deleteTargets promapi.ServiceMonitor
	err := client.DeleteAllOf(ctx,
		&deleteTargets,
		runtimeclient.InNamespace(cfg.MonitoringTargetNS),
		runtimeclient.MatchingLabels(cfg.CommonLabels),
	)
	if err != nil {
		return err
	}
	return nil
}
