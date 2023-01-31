package stargate

import (
	"bytes"
	"context"
	"text/template"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	stargatepkg "github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *StargateReconciler) reconcileVectorConfigMap(
	ctx context.Context,
	stargate api.Stargate,
	actualDc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	dcLogger logr.Logger,
) (ctrl.Result, error) {
	namespace := stargate.Namespace
	configMapKey := client.ObjectKey{
		Namespace: namespace,
		Name:      stargatepkg.VectorAgentConfigMapName(actualDc.Spec.ClusterName, actualDc.Name),
	}
	if stargate.Spec.Telemetry.IsVectorEnabled() {
		// Create the vector toml config content
		toml, err := CreateVectorToml(stargate.Spec.Telemetry)
		if err != nil {
			return ctrl.Result{}, err
		}

		desiredVectorConfigMap := stargatepkg.CreateVectorConfigMap(namespace, toml, *actualDc)
		annotations.AddHashAnnotation(desiredVectorConfigMap)

		// Check if the vector config map already exists
		actualVectorConfigMap := &corev1.ConfigMap{}

		if err := remoteClient.Get(ctx, configMapKey, actualVectorConfigMap); err != nil {
			if errors.IsNotFound(err) {
				if err := ctrl.SetControllerReference(&stargate, desiredVectorConfigMap, r.Scheme); err != nil {
					dcLogger.Error(err, "Failed to set controller reference on new Stargate Vector ConfigMap", "ConfigMap", configMapKey)
					return ctrl.Result{}, err
				} else if err := remoteClient.Create(ctx, desiredVectorConfigMap); err != nil {
					if errors.IsAlreadyExists(err) {
						// the read from the local cache didn't catch that the resource was created
						// already; simply requeue until the cache is up-to-date
						dcLogger.Info("Stargate Vector Agent configuration already exists, requeueing")
						return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
					} else {
						dcLogger.Error(err, "Failed to create Stargate Vector Agent ConfigMap")
						return ctrl.Result{}, err
					}
				}
				// Requeue to ensure the config map can be retrieved
				return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
			} else {
				dcLogger.Error(err, "Failed to get Stargate Vector Agent ConfigMap")
				return ctrl.Result{}, err
			}
		}

		actualVectorConfigMap = actualVectorConfigMap.DeepCopy()

		if !annotations.CompareHashAnnotations(actualVectorConfigMap, desiredVectorConfigMap) {
			resourceVersion := actualVectorConfigMap.GetResourceVersion()
			desiredVectorConfigMap.DeepCopyInto(actualVectorConfigMap)
			actualVectorConfigMap.SetResourceVersion(resourceVersion)
			if err := remoteClient.Update(ctx, actualVectorConfigMap); err != nil {
				dcLogger.Error(err, "Failed to update Stargate Vector Agent ConfigMap resource")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	} else {
		if err := deleteConfigMapIfExists(ctx, remoteClient, configMapKey, dcLogger); err != nil {
			return ctrl.Result{}, err
		}
	}

	dcLogger.Info("Stargate Vector Agent ConfigMap reconciliation complete")
	return ctrl.Result{}, nil
}

func deleteConfigMapIfExists(ctx context.Context, remoteClient client.Client, configMapKey client.ObjectKey, logger logr.Logger) error {
	configMap := &corev1.ConfigMap{}
	if err := remoteClient.Get(ctx, configMapKey, configMap); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "Failed to get ConfigMap", configMapKey)
		return err
	}
	if err := remoteClient.Delete(ctx, configMap); err != nil {
		logger.Error(err, "Failed to delete ConfigMap", configMapKey)
		return err
	}
	return nil
}

func CreateVectorToml(telemetrySpec *telemetryapi.TelemetrySpec) (string, error) {
	vectorConfigToml := `
[sinks.console]
type = "console"
inputs = [ "stargate_metrics" ]
target = "stdout"

  [sinks.console.encoding]
  codec = "json"`

	if telemetrySpec.Vector.Components != nil {
		// Vector components are provided in the Telemetry spec, build the Vector sink config from them
		vectorConfigToml = telemetry.BuildCustomVectorToml(telemetrySpec)
	}

	var scrapeInterval int32 = vector.DefaultScrapeInterval
	if telemetrySpec.Vector.ScrapeInterval != nil {
		scrapeInterval = int32(telemetrySpec.Vector.ScrapeInterval.Seconds())
	}

	config := vector.VectorConfig{
		Sinks:          vectorConfigToml,
		ScrapePort:     stargatepkg.MetricsPort,
		ScrapeInterval: scrapeInterval,
	}

	vectorTomlTemplate := `
data_dir = "/var/lib/vector"

[api]
enabled = false

[sources.stargate_metrics]
type = "prometheus_scrape"
endpoints = [ "http://localhost:{{ .ScrapePort }}/metrics" ]
scrape_interval_secs = {{ .ScrapeInterval }}

{{ .Sinks }}`

	t, err := template.New("toml").Parse(vectorTomlTemplate)
	if err != nil {
		panic(err)
	}
	vectorToml := new(bytes.Buffer)
	err = t.Execute(vectorToml, config)
	if err != nil {
		panic(err)
	}

	return vectorToml.String(), nil
}
