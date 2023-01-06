// Some of the tests in this package target functions from pkg/telemetryapi, because they need to be envtests. We prefer to keep envtests in the controller
// packages.
package k8ssandra

import (
	"context"

	"k8s.io/utils/pointer"

	"testing"

	testlogr "github.com/go-logr/logr/testing"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
)

// new_DummyK8ssandraClusterReconciler gives us back a `K8ssandraClusterReconciler` with just the fields that we need to test `reconcileCassandraDCTelemetry()`.
func newDummyK8ssandraClusterReconciler() K8ssandraClusterReconciler {
	return K8ssandraClusterReconciler{ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: interval}}
}

// Test_reconcileCassandraDCTelemetry_TracksNamespaces tests that the servicemonitor is created in the namespace of the CassandraDC,
// not the namespace of the k8ssandraCluster.
func Test_reconcileCassandraDCTelemetry_TracksNamespaces(t *testing.T) {
	// Test fixtures
	r := newDummyK8ssandraClusterReconciler()
	ctx := context.Background()
	fakeClient := test.NewFakeClientWRestMapper()
	testLogger := testlogr.NewTestLogger(t)
	// Resources to create
	cfg := telemetry.PrometheusResourcer{
		MonitoringTargetNS:   "test-namespace",
		MonitoringTargetName: "test-dc-name",
		Logger:               testLogger,
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-cluster-name"},
		ServiceMonitorName:   "test-cluster-name" + "-" + "test-dc-name" + "-" + "cass-servicemonitor",
	}
	cassDC := test.NewCassandraDatacenter(cfg.MonitoringTargetName, cfg.MonitoringTargetNS)
	kc := test.NewK8ssandraCluster("test-cluster-name", "test-kc-namespace")
	kc.Spec.Cassandra.Datacenters = []k8ssandraapi.CassandraDatacenterTemplate{
		{
			Meta: k8ssandraapi.EmbeddedObjectMeta{
				Namespace: cassDC.Namespace,
				Name:      cassDC.Name,
			},
			DatacenterOptions: k8ssandraapi.DatacenterOptions{
				Telemetry: &telemetryapi.TelemetrySpec{
					Prometheus: &telemetryapi.PrometheusTelemetrySpec{
						Enabled:      pointer.Bool(true),
						CommonLabels: map[string]string{"test-label": "test"},
					},
				},
			},
		},
	}
	if recResult := r.reconcileCassandraDCTelemetry(ctx, &kc, kc.Spec.Cassandra.Datacenters[0], &cassDC, testLogger, fakeClient); recResult.Completed() {
		_, err := recResult.Output()
		if err != nil {
			assert.Fail(t, "reconciliation failed", err)
		}
	}
	currentSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: cfg.ServiceMonitorName, Namespace: cfg.MonitoringTargetNS}, currentSM); err != nil {
		assert.Fail(t, "could not get actual ServiceMonitor after reconciling k8ssandra cluster", err)
	}
	assert.NotEmpty(t, currentSM.Spec.Endpoints)
	assert.NotEqual(t, kc.Namespace, currentSM.Namespace)
	assert.Contains(t, currentSM.Labels, "test-label")
}
