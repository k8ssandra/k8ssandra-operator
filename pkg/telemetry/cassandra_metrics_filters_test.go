package telemetry

import (
	"strings"
	"testing"

	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// Test_InjectCassandraTelemetryFilters tests that metrics filters from the CRD are correctly injected.
func Test_InjectCassandraTelemetryFilters(t *testing.T) {
	dcConfig := &cassandra.DatacenterConfig{
		PodTemplateSpec: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "cassandra",
						Env:  []corev1.EnvVar{},
					},
				},
			},
		},
	}

	telemetrySpec := &telemetry.TelemetrySpec{
		Prometheus: &telemetry.PrometheusTelemetrySpec{
			Enabled: ptr.To(true),
		},
		Mcac: &telemetry.McacTelemetrySpec{
			MetricFilters: &[]string{
				"deny:org.apache.cassandra.metrics.Table",
				"deny:org.apache.cassandra.metrics.table"},
		},
	}

	InjectCassandraTelemetryFilters(telemetrySpec, dcConfig)
	cassandraEnvVariables := dcConfig.PodTemplateSpec.Spec.Containers[0].Env
	assert.Equal(t, 1, len(cassandraEnvVariables), "Expected 1 env variable to be injected")
	assert.Equal(t, cassandraEnvVariables[0].Name, "METRIC_FILTERS", "Expected METRIC_FILTERS env variable to be injected")
	assert.Equal(t, cassandraEnvVariables[0].Value, "deny:org.apache.cassandra.metrics.Table deny:org.apache.cassandra.metrics.table", "Expected METRIC_FILTERS env variable to be injected")
}

// Test_InjectCassandraTelemetryFiltersDefaults tests that default metrics filters are injected when no custom ones are defined.
func Test_InjectCassandraTelemetryFiltersDefaults(t *testing.T) {
	dcConfig := &cassandra.DatacenterConfig{
		PodTemplateSpec: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "cassandra",
						Env:  []corev1.EnvVar{},
					},
				},
			},
		},
	}

	telemetrySpec := &telemetry.TelemetrySpec{
		Prometheus: &telemetry.PrometheusTelemetrySpec{
			Enabled: ptr.To(true),
		},
	}

	InjectCassandraTelemetryFilters(telemetrySpec, dcConfig)
	cassandraEnvVariables := dcConfig.PodTemplateSpec.Spec.Containers[0].Env
	assert.Equal(t, 1, len(cassandraEnvVariables), "Expected 1 env variable to be injected")
	assert.Equal(t, cassandraEnvVariables[0].Value, strings.Join(DefaultFilters, " "))
}

func Test_InjectCassandraTelemetryFilters_Empty(t *testing.T) {
	dcConfig := &cassandra.DatacenterConfig{
		PodTemplateSpec: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "cassandra",
						Env:  []corev1.EnvVar{},
					},
				},
			},
		},
	}

	// Test with an empty filters slice, which should result in an empty env variable to be injected
	telemetrySpec := &telemetry.TelemetrySpec{
		Mcac: &telemetry.McacTelemetrySpec{
			MetricFilters: &[]string{},
		},
	}
	InjectCassandraTelemetryFilters(telemetrySpec, dcConfig)
	cassandraEnvVariables := dcConfig.PodTemplateSpec.Spec.Containers[0].Env
	assert.Equal(t, 1, len(cassandraEnvVariables), "Expected 1 env variable to be injected")
	assert.Equal(t, cassandraEnvVariables[0].Name, "METRIC_FILTERS", "Expected METRIC_FILTERS env variable to be injected")
	assert.Equal(t, cassandraEnvVariables[0].Value, "", "Expected empty METRIC_FILTERS env variable to be injected")

}
