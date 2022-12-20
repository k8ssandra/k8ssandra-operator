package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/go-logr/logr"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Default resources for Vector agent
	DefaultVectorCpuRequest    = "100m"
	DefaultVectorMemoryRequest = "128Mi"
	DefaultVectorCpuLimit      = "2"
	DefaultVectorMemoryLimit   = "2Gi"
	DefaultScrapeInterval      = 30
)

// InjectCassandraVectorAgent adds the Vector agent container to the Cassandra pods.
// If the Vector agent is already present, it is not added again.
func InjectCassandraVectorAgent(telemetrySpec *telemetry.TelemetrySpec, dcConfig *cassandra.DatacenterConfig, k8cName string, logger logr.Logger) error {
	if telemetrySpec.IsVectorEnabled() {
		logger.Info("Injecting Vector agent into Cassandra pods")
		vectorImage := defaultVectorImage
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
			Resources: vectorContainerResources(telemetrySpec),
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

		cassandra.AddVolumesToPodTemplateSpec(dcConfig, vectorAgentVolume)
	}

	return nil
}

func vectorContainerResources(telemetrySpec *telemetry.TelemetrySpec) corev1.ResourceRequirements {
	if telemetrySpec.Vector.Resources != nil {
		return *telemetrySpec.Vector.Resources
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(DefaultVectorCpuRequest),
			corev1.ResourceMemory: resource.MustParse(DefaultVectorMemoryRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(DefaultVectorMemoryLimit),
			corev1.ResourceCPU:    resource.MustParse(DefaultVectorCpuLimit),
		},
	}
}

type VectorConfig struct {
	Sinks          string
	ScrapePort     int32
	ScrapeInterval int32
}

func CreateCassandraVectorToml(ctx context.Context, telemetrySpec *telemetry.TelemetrySpec, remoteClient client.Client, namespace string) (string, error) {
	sinks := `
[sinks.console]
type = "console"
inputs = [ "cassandra_metrics" ]
target = "stdout"

  [sinks.console.encoding]
  codec = "json"`

	if telemetrySpec.Vector.Config != nil {
		// Read the Vector provided config map content and use it as the Vector sink config
		vectorConfigMap := &corev1.ConfigMap{}
		err := remoteClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: telemetrySpec.Vector.Config.Name}, vectorConfigMap)
		if err != nil {
			return "", err
		} else {
			if toml, found := vectorConfigMap.Data["vector.toml"]; found {
				sinks = toml
			} else {
				return "", fmt.Errorf("vector.toml not found in config map %s", telemetrySpec.Vector.Config.Name)
			}
		}
	}

	var scrapeInterval int32 = DefaultScrapeInterval
	if telemetrySpec.Vector.ScrapeInterval != nil {
		scrapeInterval = int32(telemetrySpec.Vector.ScrapeInterval.Seconds())
	}

	config := VectorConfig{
		Sinks:          sinks,
		ScrapePort:     CassandraMetricsPort,
		ScrapeInterval: scrapeInterval,
	}

	vectorTomlTemplate := `
data_dir = "/var/lib/vector"

[api]
enabled = false
  
[sources.cassandra_metrics]
type = "prometheus_scrape"
endpoints = [ "http://localhost:{{ .ScrapePort }}" ]
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
