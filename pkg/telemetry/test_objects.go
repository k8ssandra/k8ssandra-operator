package telemetry

import (
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
)

func NewTelemetrySpec() telemetryapi.TelemetrySpec {
	return telemetryapi.TelemetrySpec{
		Cassandra: &telemetryapi.CassandraAgentSpec{},
	}

}
