// Some of the tests in this package target functions from pkg/telemetryapi, because they need to be envtests. We prefer to keep envtests in the controller
// packages.
package stargate

import (
	"context"
	"testing"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"

	testlogr "github.com/go-logr/logr/testing"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
)

// new_DummyK8ssandraClusterReconciler gives us back a `K8ssandraClusterReconciler` with just the fields that we need to test `reconcileCassandraDCTelemetry()`.
func newDummyK8ssandraClusterReconciler() StargateReconciler {
	return StargateReconciler{ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: interval}}
}

// Test_reconcileStargateTelemetry_TracksNamespaces tests that the servicemonitor is created
func Test_reconcileStargateTelemetry_succeeds(t *testing.T) {
	// Test fixtures
	r := newDummyK8ssandraClusterReconciler()
	ctx := context.Background()
	fakeClient, _ := test.NewFakeClientWithProm()
	testLogger := testlogr.NewTestLogger(t)
	// Resources to create
	stargate := test.NewStargate("test-stargate", "test-stargate-namespace")
	stargate.Spec.Telemetry = &telemetryapi.TelemetrySpec{
		Prometheus: &telemetryapi.PrometheusTelemetrySpec{
			Enabled: true,
		},
	}
	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   stargate.Namespace,
		MonitoringTargetName: stargate.Name,
		Logger:               testLogger,
		ServiceMonitorName:   GetStargatePromSMName(stargate.Name),
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
	}
	_, err := r.reconcileStargateTelemetry(ctx, &stargate, testLogger, fakeClient)
	if err != nil {
		assert.Fail(t, "reconciliation failed", err)
	}
	currentSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: cfg.ServiceMonitorName, Namespace: cfg.MonitoringTargetNS}, currentSM); err != nil {
		assert.Fail(t, "could not get actual ServiceMonitor after reconciling k8ssandra cluster", err)
	}
	assert.NotEmpty(t, currentSM.Spec.Endpoints)
	assert.Equal(t, stargate.Name, currentSM.Spec.Endpoints[0].MetricRelabelConfigs[0].Replacement)
}
