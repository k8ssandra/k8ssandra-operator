package vector

import (
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DefaultVectorImage = "timberio/vector:0.26.0-alpine"
	// Default resources for Vector agent
	DefaultVectorCpuRequest    = "100m"
	DefaultVectorMemoryRequest = "128Mi"
	DefaultVectorCpuLimit      = "2"
	DefaultVectorMemoryLimit   = "2Gi"
	DefaultScrapeInterval      = 30
	// CassandraMetricsPortLegacy is the metrics port to scrape for the legacy MCAC stack (Metrics
	// Collector for Apache Cassandra).
	CassandraMetricsPortLegacy = 9103
	// CassandraMetricsPortModern is the metrics port to scrape for the modern stack (metrics
	// exposed by management-api).
	CassandraMetricsPortModern = 9000
)

type VectorConfig struct {
	Sinks          string
	ScrapePort     int32
	ScrapeInterval int32
}

func VectorContainerResources(telemetrySpec *telemetry.TelemetrySpec) corev1.ResourceRequirements {
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
