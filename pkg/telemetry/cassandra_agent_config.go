package telemetry

import (
	"context"

	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileTelemetryAgentConfigMap(ctx context.Context, remoteClient client.Client, telemetrySpec telemetryapi.TelemetrySpec) error {
	yamlData, err := yaml.Marshal(&telemetrySpec.Cassandra)
	if err != nil {
		return err
	}
	cm := corev1.ConfigMap{}

}
