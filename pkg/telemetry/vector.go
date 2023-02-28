package telemetry

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InjectCassandraVectorAgentConfig adds a override Vector agent config to the Cassandra pods, overwriting the default in the cass-operator
func InjectCassandraVectorAgentConfig(telemetrySpec *telemetry.TelemetrySpec, dcConfig *cassandra.DatacenterConfig, k8cName string, logger logr.Logger) error {
	if telemetrySpec.IsVectorEnabled() {
		logger.V(1).Info("Injecting Vector agent into Cassandra pods")
		// vectorImage := vector.DefaultVectorImage
		loggerContainer := corev1.Container{
			Name:      reconciliation.SystemLoggerContainerName,
			Resources: vector.VectorContainerResources(telemetrySpec),

			// Add the VolumeMount (of the config) to the AdditionalVolumes part (to override the /etc/vector/vector.toml)
		}

		if dcConfig.StorageConfig == nil {
			dcConfig.StorageConfig = &cassdcapi.StorageConfig{}
		}

		dcConfig.StorageConfig.AdditionalVolumes = append(dcConfig.StorageConfig.AdditionalVolumes, cassdcapi.AdditionalVolumes{
			Name:      "vector-config",
			MountPath: "/etc/vector",
			VolumeSource: &corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: VectorAgentConfigMapName(k8cName, dcConfig.Meta.Name)},
				},
			},
		})

		if telemetrySpec.Vector.Image != "" {
			loggerContainer.Image = telemetrySpec.Vector.Image
		}

		logger.V(1).Info("Updating server-system-logger agent in Cassandra pods")
		cassandra.UpdateLoggerContainer(&dcConfig.PodTemplateSpec, func(container *corev1.Container) {
			*container = loggerContainer
		})
	}

	return nil
}

func CreateCassandraVectorToml(telemetrySpec *telemetry.TelemetrySpec, mcacEnabled bool) (string, error) {
	vectorConfigToml := `
[sinks.console]
type = "console"
inputs = [ "cassandra_metrics" ]
target = "stdout"

  [sinks.console.encoding]
  codec = "json"`

	if telemetrySpec.Vector.Components != nil {
		// Vector components are provided in the Telemetry spec, build the Vector sink config from them
		vectorConfigToml = BuildCustomVectorToml(telemetrySpec)
	}

	var scrapePort int32
	metricsEndpoint := ""
	if mcacEnabled {
		scrapePort = vector.CassandraMetricsPortLegacy
	} else {
		scrapePort = vector.CassandraMetricsPortModern
		metricsEndpoint = "/metrics"
	}

	var scrapeInterval int32 = vector.DefaultScrapeInterval
	if telemetrySpec.Vector.ScrapeInterval != nil {
		scrapeInterval = int32(telemetrySpec.Vector.ScrapeInterval.Seconds())
	}

	config := vector.VectorConfig{
		Sinks:          vectorConfigToml,
		ScrapePort:     scrapePort,
		ScrapeInterval: scrapeInterval,
		ScrapeEndpoint: metricsEndpoint,
	}

	vectorTomlTemplate := `
data_dir = "/var/lib/vector"

[api]
enabled = false
  
[sources.cassandra_metrics]
type = "prometheus_scrape"
endpoints = [ "http://localhost:{{ .ScrapePort }}{{ .ScrapeEndpoint }}" ]
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

func BuildCustomVectorToml(telemetrySpec *telemetry.TelemetrySpec) string {
	vectorConfigToml := ""
	for _, source := range telemetrySpec.Vector.Components.Sources {
		vectorConfigToml += fmt.Sprintf("\n[sources.%s]\n", source.Name)
		vectorConfigToml += fmt.Sprintf("type = \"%s\"\n", source.Type)
		if source.Config != "" {
			vectorConfigToml += source.Config + "\n"
		}
	}

	for _, transform := range telemetrySpec.Vector.Components.Transforms {
		vectorConfigToml += fmt.Sprintf("\n[transforms.%s]\n", transform.Name)
		vectorConfigToml += fmt.Sprintf("type = \"%s\"\n", transform.Type)
		vectorConfigToml += fmt.Sprintf("inputs = [\"%s\"]\n", strings.Join(transform.Inputs, "\", \""))
		if transform.Config != "" {
			vectorConfigToml += transform.Config + "\n"
		}
	}

	for _, sink := range telemetrySpec.Vector.Components.Sinks {
		vectorConfigToml += fmt.Sprintf("\n[sinks.%s]\n", sink.Name)
		vectorConfigToml += fmt.Sprintf("type = \"%s\"\n", sink.Type)
		vectorConfigToml += fmt.Sprintf("inputs = [\"%s\"]\n", strings.Join(sink.Inputs, "\", \""))
		if sink.Config != "" {
			vectorConfigToml += sink.Config + "\n"
		}
	}

	return vectorConfigToml
}

func BuildVectorAgentConfigMap(namespace, k8cName, dcName, k8cNamespace, vectorToml string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VectorAgentConfigMapName(k8cName, dcName),
			Namespace: namespace,
			Labels: map[string]string{
				k8ssandraapi.NameLabel:                      k8ssandraapi.NameLabelValue,
				k8ssandraapi.PartOfLabel:                    k8ssandraapi.PartOfLabelValue,
				k8ssandraapi.ComponentLabel:                 k8ssandraapi.ComponentLabelValueCassandra,
				k8ssandraapi.CreatedByLabel:                 k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
				k8ssandraapi.K8ssandraClusterNameLabel:      k8cName,
				k8ssandraapi.K8ssandraClusterNamespaceLabel: k8cNamespace,
			},
		},
		Data: map[string]string{
			"vector.toml": vectorToml,
		},
	}
}

func VectorAgentConfigMapName(k8cName, dcName string) string {
	return fmt.Sprintf("%s-%s-cass-vector", k8cName, dcName)
}
