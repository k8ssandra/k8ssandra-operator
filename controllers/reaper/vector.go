package reaper

import (
	"bytes"
	"context"
	"text/template"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	reaperpkg "github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReaperReconciler) reconcileVectorConfigMap(
	ctx context.Context,
	reaper api.Reaper,
	actualDc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	dcLogger logr.Logger,
) (ctrl.Result, error) {
	namespace := reaper.Namespace
	configMapKey := client.ObjectKey{
		Namespace: namespace,
		Name:      reaperpkg.VectorAgentConfigMapName(actualDc.Spec.ClusterName, actualDc.Name),
	}
	if reaper.Spec.Telemetry.IsVectorEnabled() {
		// Create the vector toml config content
		toml, err := CreateVectorToml(reaper.Spec.Telemetry)
		if err != nil {
			return ctrl.Result{}, err
		}

		desiredVectorConfigMap := reaperpkg.CreateVectorConfigMap(namespace, toml, *actualDc)
		if err := ctrl.SetControllerReference(&reaper, desiredVectorConfigMap, r.Scheme); err != nil {
			dcLogger.Error(err, "Failed to set controller reference on new Reaper Vector ConfigMap", "ConfigMap", configMapKey)
			return ctrl.Result{}, err
		}
		recRes := reconciliation.ReconcileObject(ctx, remoteClient, r.DefaultDelay, *desiredVectorConfigMap)
		switch {
		case recRes.IsError():
			return ctrl.Result{}, recRes.GetError()
		case recRes.IsRequeue():
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

	} else {
		if err := deleteConfigMapIfExists(ctx, remoteClient, configMapKey, dcLogger); err != nil {
			return ctrl.Result{}, err
		}
	}

	dcLogger.Info("Reaper Vector Agent ConfigMap reconciliation complete")
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
inputs = [ "reaper_metrics" ]
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
		ScrapePort:     reaperpkg.MetricsPort,
		ScrapeInterval: scrapeInterval,
	}

	vectorTomlTemplate := `
data_dir = "/var/lib/vector"

[api]
enabled = false

[sources.reaper_metrics]
type = "prometheus_scrape"
endpoints = [ "http://localhost:{{ .ScrapePort }}/prometheusMetrics" ]
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
