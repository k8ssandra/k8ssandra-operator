package telemetry

import (
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	"testing"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func Test_NewCassServiceMonitor(t *testing.T) {
	logger := testr.New(t)
	tests := []struct {
		name            string
		cfg             PrometheusResourcer
		legacyEndpoints bool
		wantTargetPort  int32
		wantErr         require.ErrorAssertionFunc
	}{
		{
			name: "MCAC enabled",
			cfg: PrometheusResourcer{
				MonitoringTargetNS:   "test-namespace",
				MonitoringTargetName: "test-dc-name",
				Logger:               logger,
				ServiceMonitorName:   "test-servicemonitor",
				CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
			},
			legacyEndpoints: true,
			wantTargetPort:  CassandraMetricsPortLegacy,
			wantErr:         require.NoError,
		},
		{
			name: "MCAC disabled",
			cfg: PrometheusResourcer{
				MonitoringTargetNS:   "test-namespace",
				MonitoringTargetName: "test-dc-name",
				Logger:               logger,
				ServiceMonitorName:   "test-servicemonitor",
				CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
			},
			legacyEndpoints: false,
			wantTargetPort:  CassandraMetricsPortModern,
			wantErr:         require.NoError,
		},
		{
			name: "validation error",
			cfg: PrometheusResourcer{
				Logger:             logger,
				ServiceMonitorName: "test-servicemonitor",
				CommonLabels:       map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
			},
			wantErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.EqualError(t, err, "TelemetryConfig did not contain required fields to process into a TelemetryConfig, missing field MonitoringTargetNS", msgAndArgs...)
			},
		},
		{
			name: "labels error",
			cfg: PrometheusResourcer{
				MonitoringTargetNS:   "test-namespace",
				MonitoringTargetName: "test-dc-name",
				Logger:               logger,
				ServiceMonitorName:   "test-servicemonitor",
			},
			wantErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.EqualError(t, err, "TelemetryConfig did not contain required fields to process into a TelemetryConfig, missing field PrometheusResourcer.CommonLabels[k8ssandraapi.K8ssandraClusterNameLabel]", msgAndArgs...)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualSM, err := test.cfg.NewCassServiceMonitor(test.legacyEndpoints)
			test.wantErr(t, err)
			if err == nil {
				assert.Len(t, actualSM.Spec.Endpoints, 1)
				assert.Equal(t, "", actualSM.Spec.Endpoints[0].Port)
				assert.Equal(t, test.wantTargetPort, actualSM.Spec.Endpoints[0].TargetPort.IntVal)
			}
		})
	}
}
