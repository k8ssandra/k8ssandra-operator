package telemetry

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func Test_NewCassServiceMonitor(t *testing.T) {
	logger := testr.New(t)
	tests := []struct {
		name            string
		cfg             PrometheusResourcer
		legacyEndpoints bool
		wantPort        string
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
			wantPort:        "prometheus",
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
			wantPort:        "metrics",
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
				assert.Equal(t, test.wantPort, actualSM.Spec.Endpoints[0].Port)
			}
		})
	}
}

// The Cassandra pods carry the Cassandra cluster name, which differs from the
// K8ssandraCluster name when spec.cassandra.clusterName is set, so the selector
// must use it or the ServiceMonitor matches no pods.
func Test_NewCassServiceMonitor_SelectsCassandraClusterName(t *testing.T) {
	cfg := PrometheusResourcer{
		MonitoringTargetNS:   "test-namespace",
		MonitoringTargetName: "test-dc-name",
		Logger:               testr.New(t),
		ServiceMonitorName:   "test-servicemonitor",
		CassClusterName:      "Custom Cluster",
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
	}
	sm, err := cfg.NewCassServiceMonitor(false)
	require.NoError(t, err)
	var got []string
	for _, req := range sm.Spec.Selector.MatchExpressions {
		if req.Key == cassdcapi.ClusterLabel {
			got = req.Values
		}
	}
	assert.Equal(t, []string{cassdcapi.CleanLabelValue("Custom Cluster")}, got)
}
