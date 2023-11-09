package telemetry

import (
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

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
		logger.V(1).Info("Updating server-system-logger agent in Cassandra pods")
		loggerContainer := corev1.Container{
			Name: reconciliation.SystemLoggerContainerName,
		}

		loggerResources := vector.VectorContainerResources(telemetrySpec)
		dcConfig.SystemLoggerResources = &loggerResources

		if dcConfig.StorageConfig == nil {
			dcConfig.StorageConfig = &cassdcapi.StorageConfig{}
		}

		dcConfig.StorageConfig.AdditionalVolumes = append(dcConfig.StorageConfig.AdditionalVolumes, cassdcapi.AdditionalVolumes{
			Name:      "vector-config",
			MountPath: "/etc/vector",
			VolumeSource: &corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: VectorAgentConfigMapName(k8cName, dcConfig.CassDcName())},
				},
			},
		})

		if telemetrySpec.Vector.Image != "" {
			loggerContainer.Image = telemetrySpec.Vector.Image
		}

		cassandra.UpdateLoggerContainer(&dcConfig.PodTemplateSpec, func(container *corev1.Container) {
			*container = loggerContainer
		})
	}

	return nil
}

func CreateCassandraVectorToml(telemetrySpec *telemetry.TelemetrySpec, mcacEnabled bool) (string, error) {
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
		ScrapePort:     scrapePort,
		ScrapeInterval: scrapeInterval,
		ScrapeEndpoint: metricsEndpoint,
	}

	defaultSources, defaultTransformers, defaultSinks := BuildDefaultVectorComponents(config)

	if telemetrySpec.Vector.Components == nil {
		telemetrySpec.Vector.Components = &telemetry.VectorComponentsSpec{}
	}

	telemetrySpec.Vector.Components.Sources = append(telemetrySpec.Vector.Components.Sources, defaultSources...)
	telemetrySpec.Vector.Components.Sinks = append(telemetrySpec.Vector.Components.Sinks, defaultSinks...)
	telemetrySpec.Vector.Components.Transforms = append(telemetrySpec.Vector.Components.Transforms, defaultTransformers...)

	// Remove defaults if overrides are used and filter incorrect sources without sink destination
	sources, transformers, sinks := FilterUnusedPipelines(telemetrySpec.Vector.Components.Sources, telemetrySpec.Vector.Components.Transforms, telemetrySpec.Vector.Components.Sinks)
	telemetrySpec.Vector.Components.Sources = sources
	telemetrySpec.Vector.Components.Transforms = transformers
	telemetrySpec.Vector.Components.Sinks = sinks

	// Vector components are provided in the Telemetry spec, build the Vector sink config from them
	vectorConfigToml := BuildCustomVectorToml(telemetrySpec)
	return vectorConfigToml, nil
}

func BuildDefaultVectorComponents(config vector.VectorConfig) ([]telemetry.VectorSourceSpec, []telemetry.VectorTransformSpec, []telemetry.VectorSinkSpec) {
	sources := make([]telemetry.VectorSourceSpec, 0, 2)
	transformers := make([]telemetry.VectorTransformSpec, 0, 1)
	sinks := make([]telemetry.VectorSinkSpec, 0, 1)

	systemLogInput := telemetry.VectorSourceSpec{
		Name: "systemlog",
		Type: "file",
		Config: `include = [ "/var/log/cassandra/system.log" ]
read_from = "beginning"
fingerprint.strategy = "device_and_inode"
[sources.systemlog.multiline]
start_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
condition_pattern = "^(INFO|WARN|ERROR|DEBUG|TRACE|FATAL)"
mode = "halt_before"
timeout_ms = 10000
`,
	}

	metricsInput := telemetry.VectorSourceSpec{
		Name:   "cassandra_metrics_raw",
		Type:   "prometheus_scrape",
		Config: fmt.Sprintf("endpoints = [ \"http://localhost:%v%s\" ]\nscrape_interval_secs = %v", config.ScrapePort, config.ScrapeEndpoint, config.ScrapeInterval),
	}

	sources = append(sources, systemLogInput, metricsInput)

	// We provide this transform out of the box because it's likely to be a common need for users who extend the
	// configuration; however by default we don't use it, it will be filtered out unless it's referenced by one of the
	// user components.
	systemLogParser := telemetry.VectorTransformSpec{
		Name:   "parse_cassandra_log",
		Type:   "remap",
		Inputs: []string{"systemlog"},
		Config: `source = '''
del(.source_type)
. |= parse_groks!(.message, patterns: [
  "%{LOGLEVEL:loglevel}\\s+\\[(?<thread>((.+)))\\]\\s+%{TIMESTAMP_ISO8601:timestamp}\\s+%{JAVACLASS:class}:%{NUMBER:line}\\s+-\\s+(?<message>(.+\\n?)+)",
  ]
)
pod_name, err = get_env_var("POD_NAME")
if err == null {
  .pod_name = pod_name
}
node_name, err = get_env_var("NODE_NAME")
if err == null {
  .node_name = node_name
}
cluster, err = get_env_var("CLUSTER_NAME")
if err == null {
  .cluster = cluster
}
datacenter, err = get_env_var("DATACENTER_NAME")
if err == null {
  .datacenter = datacenter
}
rack, err = get_env_var("RACK_NAME")
if err == null {
  .rack = rack
}
namespace, err = get_env_var("NAMESPACE")
if err == null {
  .namespace = namespace
}
'''
`,
	}

	transformers = append(transformers, systemLogParser)

	// Add the namespace label to the Cassandra metrics
	metricsParser := telemetry.VectorTransformSpec{
		Name:   "cassandra_metrics",
		Type:   "remap",
		Inputs: []string{"cassandra_metrics_raw"},
		Config: `source = '''
namespace, err = get_env_var("NAMESPACE")
if err == null {
  .tags.namespace = namespace
}
'''
`,
	}

	transformers = append(transformers, metricsParser)

	systemLogSink := telemetry.VectorSinkSpec{
		Name:   "console_log",
		Type:   "console",
		Inputs: []string{"systemlog"},
		Config: `target = "stdout"
encoding.codec = "text"
`,
	}

	sinks = append(sinks, systemLogSink)

	return sources, transformers, sinks
}

// FilterUnusedPipelines removes sources that have no destination in the sinks. If there are duplicate transformer, source or sink names, only the first occurence wins
func FilterUnusedPipelines(sources []telemetry.VectorSourceSpec, transformers []telemetry.VectorTransformSpec, sinks []telemetry.VectorSinkSpec) ([]telemetry.VectorSourceSpec, []telemetry.VectorTransformSpec, []telemetry.VectorSinkSpec) {
	// Every source must be mapped to at least one transformer or sink

	sinkInputs := make(map[string]bool)
	for _, sink := range sinks {
		for _, input := range sink.Inputs {
			sinkInputs[input] = true
		}
	}

	transformerInputs := make(map[string]map[string]bool)
	for _, transformer := range transformers {
		for _, input := range transformer.Inputs {
			// Overwrite is fine
			if transformerInputs[input] == nil {
				transformerInputs[input] = make(map[string]bool)
			}
			transformerInputs[input][transformer.Name] = true
		}
	}

	// All transformer must have a sink input or another transformer input (thus, if you create a loop, this won't detect it as failure)
	// All sources must have sink input or transformer input

Clean:
	for {
		transSet := make(map[string]bool, len(transformers))
		for i := 0; i < len(transformers); i++ {
			trans := transformers[i]
			_, foundSinkOutput := sinkInputs[trans.Name]
			_, foundTransformerOut := transformerInputs[trans.Name]
			_, addedAlready := transSet[trans.Name]

			if !foundSinkOutput && !foundTransformerOut || addedAlready {
				// Can no longer be an output for another
				for k, v := range transformerInputs {
					if _, found := v[trans.Name]; found {
						delete(v, trans.Name)
						if len(v) == 0 {
							// No more outputs for this key at all
							delete(transformerInputs, k)
						}
					}
				}
				transformers = append(transformers[:i], transformers[i+1:]...)
				continue Clean
			}
			transSet[trans.Name] = true
		}
		break
	}

	sourceSet := make(map[string]bool, len(sources))
	safeSources := make([]telemetry.VectorSourceSpec, 0, len(sources))

	for _, source := range sources {
		_, foundSinkOutput := sinkInputs[source.Name]
		_, foundTransformerOut := transformerInputs[source.Name]
		_, added := sourceSet[source.Name]

		if (foundSinkOutput || foundTransformerOut) && !added {
			// This source can be used
			safeSources = append(safeSources, source)
			sourceSet[source.Name] = true
		}
	}

	sinkSet := make(map[string]bool, len(sinks))
	safeSinks := make([]telemetry.VectorSinkSpec, 0, len(sinks))
	for _, sink := range sinks {
		_, added := sinkSet[sink.Name]

		if !added {
			safeSinks = append(safeSinks, sink)
			sinkSet[sink.Name] = true
		}
	}

	return safeSources, transformers, safeSinks
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
			Labels: utils.MergeMap(
				map[string]string{
					k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
					k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
					k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueCassandra,
				},
				labels.CleanedUpByLabels(client.ObjectKey{Namespace: k8cNamespace, Name: k8cName})),
		},
		Data: map[string]string{
			"vector.toml": vectorToml,
		},
	}
}

func VectorAgentConfigMapName(k8cName, dcName string) string {
	return fmt.Sprintf("%s-%s-cass-vector", k8cName, dcName)
}
