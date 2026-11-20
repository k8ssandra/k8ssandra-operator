package telemetry

import (
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	k8ssandra "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func vectorSpecWithSinks(inputs ...string) *telemetry.TelemetrySpec {
	return &telemetry.TelemetrySpec{
		Vector: &telemetry.VectorSpec{
			Enabled: ptr.To(true),
			Components: &telemetry.VectorComponentsSpec{
				Sinks: []telemetry.VectorSinkSpec{{Name: "metrics_output", Type: "console", Inputs: inputs}},
			},
		},
	}
}

func medusaWithTLS() *medusaapi.MedusaClusterTemplate {
	return &medusaapi.MedusaClusterTemplate{
		ServiceProperties: medusaapi.Service{
			MetricsTLS: true,
			Encryption: &medusaapi.GRPCEncryption{ServerSecretName: "server-certs", ClientSecretName: "client-certs"},
		},
	}
}

func TestCreateCassandraVectorTomlMedusa(t *testing.T) {
	t.Run("Scraped", func(t *testing.T) {
		toml, err := CreateCassandraVectorToml(vectorSpecWithSinks(CassandraMetricsTransformName), false, &medusaapi.MedusaClusterTemplate{})
		require.NoError(t, err)

		assert.Contains(t, toml, "[sources.medusa_metrics_raw]\ntype = \"prometheus_scrape\"\nendpoints = [ \"http://127.0.0.1:9500/metrics\" ]\nscrape_interval_secs = 30\n")
		assert.Contains(t, toml, "[transforms.medusa_metrics]\ntype = \"remap\"\ninputs = [\"medusa_metrics_raw\"]\n")
		assert.Contains(t, toml, `.tags.container = "medusa"`)
		for _, env := range []string{"POD_NAME", "CLUSTER_NAME", "DATACENTER_NAME", "RACK_NAME"} {
			assert.Contains(t, toml, `get_env_var("`+env+`")`)
		}
		assert.Contains(t, toml, "[transforms.cassandra_metrics]\ntype = \"remap\"\ninputs = [\"cassandra_metrics_raw\", \"medusa_metrics\"]\n")
		assert.NotContains(t, toml, "tls.")
	})

	t.Run("ScrapeIntervalAndPort", func(t *testing.T) {
		spec := vectorSpecWithSinks(CassandraMetricsTransformName)
		spec.Vector.ScrapeInterval = &metav1.Duration{Duration: 15 * time.Second}
		toml, err := CreateCassandraVectorToml(spec, false, &medusaapi.MedusaClusterTemplate{ServiceProperties: medusaapi.Service{MetricsPort: 9600}})
		require.NoError(t, err)

		assert.Contains(t, toml, "endpoints = [ \"http://127.0.0.1:9600/metrics\" ]\nscrape_interval_secs = 15")
		assert.Contains(t, toml, "endpoints = [ \"http://localhost:9000/metrics\" ]\nscrape_interval_secs = 15")
	})

	t.Run("TLS", func(t *testing.T) {
		toml, err := CreateCassandraVectorToml(vectorSpecWithSinks(CassandraMetricsTransformName), false, medusaWithTLS())
		require.NoError(t, err)

		assert.Contains(t, toml, "endpoints = [ \"https://127.0.0.1:9500/metrics\" ]\nscrape_interval_secs = 30\n"+
			"tls.ca_file = \"/etc/certificates/medusa-client-certs/ca.crt\"\n"+
			"tls.crt_file = \"/etc/certificates/medusa-client-certs/tls.crt\"\n"+
			"tls.key_file = \"/etc/certificates/medusa-client-certs/tls.key\"\n"+
			"tls.verify_hostname = false\n")
	})

	t.Run("MedusaTransformAsSinkInput", func(t *testing.T) {
		toml, err := CreateCassandraVectorToml(vectorSpecWithSinks(MedusaMetricsTransformName), false, &medusaapi.MedusaClusterTemplate{})
		require.NoError(t, err)

		assert.Contains(t, toml, "[sources.medusa_metrics_raw]")
		assert.Contains(t, toml, "[transforms.medusa_metrics]")
		assert.NotContains(t, toml, "[sources.cassandra_metrics_raw]")
		assert.NotContains(t, toml, "[transforms.cassandra_metrics]")
	})

	t.Run("NoMetricsSink", func(t *testing.T) {
		toml, err := CreateCassandraVectorToml(&telemetry.TelemetrySpec{Vector: &telemetry.VectorSpec{Enabled: ptr.To(true)}}, false, &medusaapi.MedusaClusterTemplate{})
		require.NoError(t, err)

		assert.NotContains(t, toml, "metrics_raw")
		assert.NotContains(t, toml, "medusa_metrics")
	})

	t.Run("TLSWithServerSecretOnly", func(t *testing.T) {
		medusaSpec := medusaWithTLS()
		medusaSpec.ServiceProperties.Encryption.ClientSecretName = ""
		toml, err := CreateCassandraVectorToml(vectorSpecWithSinks(CassandraMetricsTransformName), false, medusaSpec)
		require.NoError(t, err)

		assert.Contains(t, toml, "http://127.0.0.1:9500/metrics")
		assert.NotContains(t, toml, "tls.")
	})

	t.Run("UserSourceReplacesScrape", func(t *testing.T) {
		spec := vectorSpecWithSinks(CassandraMetricsTransformName)
		spec.Vector.Components.Sources = []telemetry.VectorSourceSpec{{Name: MedusaMetricsSourceName, Type: "internal_metrics"}}
		toml, err := CreateCassandraVectorToml(spec, false, &medusaapi.MedusaClusterTemplate{})
		require.NoError(t, err)

		assert.Contains(t, toml, "[sources.medusa_metrics_raw]\ntype = \"internal_metrics\"\n")
		assert.NotContains(t, toml, "127.0.0.1:9500")
	})

	t.Run("NoMedusa", func(t *testing.T) {
		toml, err := CreateCassandraVectorToml(vectorSpecWithSinks(CassandraMetricsTransformName), false, nil)
		require.NoError(t, err)

		assert.NotContains(t, toml, "medusa")
		assert.Contains(t, toml, "[transforms.cassandra_metrics]\ntype = \"remap\"\ninputs = [\"cassandra_metrics_raw\"]\n")
	})
}

func TestInjectCassandraVectorAgentConfigMedusa(t *testing.T) {
	inject := func(medusaSpec *medusaapi.MedusaClusterTemplate) *cassandra.DatacenterConfig {
		dcConfig := &cassandra.DatacenterConfig{Meta: k8ssandra.EmbeddedObjectMeta{Name: "dc1"}, Cluster: "test1"}
		require.NoError(t, InjectCassandraVectorAgentConfig(&telemetry.TelemetrySpec{Vector: &telemetry.VectorSpec{Enabled: ptr.To(true)}}, dcConfig, "test", medusaSpec, testr.New(t)))
		return dcConfig
	}
	volumeNames := func(dcConfig *cassandra.DatacenterConfig) []string {
		names := []string{}
		for _, v := range dcConfig.StorageConfig.AdditionalVolumes {
			names = append(names, v.Name)
		}
		return names
	}

	t.Run("TLS", func(t *testing.T) {
		dcConfig := inject(medusaWithTLS())
		require.Contains(t, volumeNames(dcConfig), MedusaClientCertsVolumeName)
		for _, v := range dcConfig.StorageConfig.AdditionalVolumes {
			if v.Name == MedusaClientCertsVolumeName {
				assert.Equal(t, MedusaClientCertsMountPath, v.MountPath)
				require.NotNil(t, v.VolumeSource.Secret)
				assert.Equal(t, "client-certs", v.VolumeSource.Secret.SecretName)
			}
		}
	})

	t.Run("EncryptionWithoutMetricsTLS", func(t *testing.T) {
		medusaSpec := medusaWithTLS()
		medusaSpec.ServiceProperties.MetricsTLS = false
		assert.NotContains(t, volumeNames(inject(medusaSpec)), MedusaClientCertsVolumeName)
	})

	t.Run("NoMedusa", func(t *testing.T) {
		assert.Equal(t, []string{"vector-config"}, volumeNames(inject(nil)))
	})
}
