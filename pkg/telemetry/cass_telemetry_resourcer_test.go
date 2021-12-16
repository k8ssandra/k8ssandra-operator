// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"github.com/go-logr/logr"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
)

// newCassTelemetryResourcer returns a new CassTelemetryResourcer
func newCassTelemetryResourcer(logger logr.Logger) CassTelemetryResourcer {
	return CassTelemetryResourcer{
		CassandraNamespace: "test-namespace",
		DataCenterName:     "test-DC",
		ClusterName:        "test-cluster",
		TelemetrySpec: &telemetryapi.TelemetrySpec{
			Prometheus: &telemetryapi.PrometheusTelemetrySpec{
				Enabled: true,
			},
		},
		Logger: logger,
	}
}
