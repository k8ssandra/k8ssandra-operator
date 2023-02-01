package telemetry

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InjectCassandraVectorAgent adds the Vector agent container to the Cassandra pods.
// If the Vector agent is already present, it is not added again.
func InjectCassandraVectorAgent(telemetrySpec *telemetry.TelemetrySpec, dcConfig *cassandra.DatacenterConfig, k8cName string, logger logr.Logger) error {
	if telemetrySpec.IsVectorEnabled() {
		logger.Info("Injecting Vector agent into Cassandra pods")
		vectorImage := vector.DefaultVectorImage
		if telemetrySpec.Vector.Image != "" {
			vectorImage = telemetrySpec.Vector.Image
		}

		// Create the definition of the Vector agent container
		vectorAgentContainer := corev1.Container{
			Name:            cassandra.VectorContainerName,
			Image:           vectorImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env: []corev1.EnvVar{
				{Name: "VECTOR_CONFIG", Value: "/etc/vector/vector.toml"},
				{Name: "VECTOR_ENVIRONMENT", Value: "kubernetes"},
				{Name: "VECTOR_HOSTNAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "vector-config", MountPath: "/etc/vector"},
			},
			Resources: vector.VectorContainerResources(telemetrySpec),
		}

		logger.Info("Updating Vector agent in Cassandra pods")
		cassandra.UpdateVectorContainer(&dcConfig.PodTemplateSpec, func(container *corev1.Container) {
			*container = vectorAgentContainer
		})

		// Create the definition of the Vector agent config map volume
		vectorAgentVolume := corev1.Volume{
			Name: "vector-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: VectorAgentConfigMapName(k8cName, dcConfig.Meta.Name)},
				},
			},
		}

		cassandra.AddVolumesToPodTemplateSpec(&dcConfig.PodTemplateSpec, vectorAgentVolume)
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
