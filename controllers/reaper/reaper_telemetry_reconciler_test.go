// Some of the tests in this package target functions from pkg/telemetryapi, because they need to be envtests. We prefer to keep envtests in the controller
// packages.
package reaper

import (
	"context"
	"k8s.io/utils/pointer"
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
func newDummyK8ssandraClusterReconciler() ReaperReconciler {
	return ReaperReconciler{ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: interval}}
}

// Test_reconcilereaperTelemetry_TracksNamespaces tests that the servicemonitor is created
func Test_reconcilereaperTelemetry_succeeds(t *testing.T) {
	// Test fixtures
	r := newDummyK8ssandraClusterReconciler()
	ctx := context.Background()
	fakeClient := test.NewFakeClientWRestMapper()
	testLogger := testlogr.NewTestLogger(t)
	// Resources to create
	reaper := test.NewReaper("test-reaper", "test-reaper-namespace")
	reaper.Spec.Telemetry = &telemetryapi.TelemetrySpec{
		Prometheus: &telemetryapi.PrometheusTelemetrySpec{
			Enabled: pointer.Bool(true),
		},
	}
	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   reaper.Namespace,
		MonitoringTargetName: reaper.Name,
		Logger:               testLogger,
		ServiceMonitorName:   GetReaperPromSMName(reaper.Name),
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
	}
	if err := r.reconcileReaperTelemetry(ctx, &reaper, testLogger, fakeClient); err != nil {
		assert.Fail(t, "reconciliation failed", err)
	}
	currentSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: cfg.ServiceMonitorName, Namespace: cfg.MonitoringTargetNS}, currentSM); err != nil {
		assert.Fail(t, "could not get actual ServiceMonitor after reconciling k8ssandra cluster", err)
	}
	assert.NotEmpty(t, currentSM.Spec.Endpoints)
}
