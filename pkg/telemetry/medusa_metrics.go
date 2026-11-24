package telemetry

import (
	"path"

	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	telemetry "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
)

const (
	// CassandraMetricsTransformName is the default transform that every Cassandra pod metric goes through.
	CassandraMetricsTransformName = "cassandra_metrics"

	MedusaMetricsSourceName    = "medusa_metrics_raw"
	MedusaMetricsTransformName = "medusa_metrics"

	// MedusaClientCertsVolumeName holds the Medusa gRPC client certificate, which Vector presents to Medusa's
	// metrics endpoint when it is served over mutual TLS.
	MedusaClientCertsVolumeName = "medusa-client-certs"
	MedusaClientCertsMountPath  = "/etc/certificates/medusa-client-certs"
)

// medusaMetricsTransform tags the Medusa metrics so they can be told apart from the Cassandra metrics and traced to
// their pod. The tag names are the ones the Cassandra metrics carry.
const medusaMetricsTransform = `source = '''
.tags.container = "medusa"
pod_name, err = get_env_var("POD_NAME")
if err == null {
  .tags.pod_name = pod_name
}
cluster, err = get_env_var("CLUSTER_NAME")
if err == null {
  .tags.cluster = cluster
}
datacenter, err = get_env_var("DATACENTER_NAME")
if err == null {
  .tags.datacenter = datacenter
}
rack, err = get_env_var("RACK_NAME")
if err == null {
  .tags.rack = rack
}
'''
`

// BuildMedusaMetricsComponents returns the Vector source that scrapes Medusa's metrics endpoint and the transform that
// tags its metrics. The transform's output is meant to be an input of the cassandra_metrics transform.
func BuildMedusaMetricsComponents(medusaSpec *medusaapi.MedusaClusterTemplate, scrapeInterval int32) (telemetry.VectorSourceSpec, telemetry.VectorTransformSpec) {
	scrape := PrometheusScrapeConfig{
		ScrapePort:     int32(medusaSpec.ServiceProperties.MetricsPortOrDefault()),
		ScrapeInterval: scrapeInterval,
		// Medusa listens on 0.0.0.0, which is IPv4 only, and localhost may resolve to ::1. Vector runs in the
		// same pod, so the loopback address also works with hostNetwork.
		ScrapeAddress:  "127.0.0.1",
		ScrapeEndpoint: "/metrics",
	}
	if medusaMetricsTLS(medusaSpec) {
		scrape.TLS = &telemetry.TLSConfig{
			CAFile:   path.Join(MedusaClientCertsMountPath, "ca.crt"),
			CertFile: path.Join(MedusaClientCertsMountPath, "tls.crt"),
			KeyFile:  path.Join(MedusaClientCertsMountPath, "tls.key"),
		}
		// Like the operator's Medusa gRPC client, check the certificate chain but not the hostname: the server
		// certificate does not need to carry 127.0.0.1.
		scrape.SkipVerifyHostname = true
	}

	source := telemetry.VectorSourceSpec{
		Name:   MedusaMetricsSourceName,
		Type:   "prometheus_scrape",
		Config: scrape.String(),
	}
	transform := telemetry.VectorTransformSpec{
		Name:   MedusaMetricsTransformName,
		Type:   "remap",
		Inputs: []string{MedusaMetricsSourceName},
		Config: medusaMetricsTransform,
	}
	return source, transform
}

// medusaMetricsTLS tells whether Medusa's metrics endpoint is served over mutual TLS.
func medusaMetricsTLS(medusaSpec *medusaapi.MedusaClusterTemplate) bool {
	return medusaSpec != nil && medusaSpec.ServiceProperties.MetricsTLSEnabled()
}
