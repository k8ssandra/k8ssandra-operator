// Some of the tests in this package target functions from pkg/telemetryapi, because they need to be envtests. We prefer to keep envtests in the controller
// packages.
package k8ssandra

import (
	"context"
	testlogr "github.com/go-logr/logr/testing"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"testing"

	"github.com/stretchr/testify/assert"
)

// new_DummyK8ssandraClusterReconciler gives us back a `K8ssandraClusterReconciler` with just the fields that we need to test `reconcileCassandraDCTelemetry()`.
func newDummyK8ssandraClusterReconciler() K8ssandraClusterReconciler {
	return K8ssandraClusterReconciler{ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: interval}}
}

// Some magic to override the RESTMapper().KindsFor(...) call. fake client blows up with a panic otherwise.
type composedClient struct {
	client.Client
}
type fakeRESTMapper struct {
	meta.RESTMapper
}

func (c composedClient) RESTMapper() meta.RESTMapper {
	return fakeRESTMapper{}
}
func (rm fakeRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return []schema.GroupVersionKind{
		{
			Group:   promapi.SchemeGroupVersion.Group,
			Version: promapi.Version,
			Kind:    promapi.ServiceMonitorsKind,
		},
	}, nil
}
func NewFakeClientWRestMapper() client.Client {
	fakeClient, _ := test.NewFakeClient()
	return composedClient{fakeClient}
}

// Test_CassPrometheusResourcer_UpdateResources_TracksNamespaces tests that the servicemonitor is created in the namespace of the CassandraDC,
// not the namespace of the k8ssandraCluster.
func Test_reconcileCassandraDCTelemetry_TracksNamespaces(t *testing.T) {
	// Test fixtures
	r := newDummyK8ssandraClusterReconciler()
	ctx := context.Background()
	fakeClient := NewFakeClientWRestMapper()
	testLogger := testlogr.TestLogger{t}
	// Resources to create
	cassDC := test.NewCassandraDatacenter()
	cfg := telemetry.CassPrometheusResourcer{
		CassTelemetryResourcer: telemetry.CassTelemetryResourcer{
			CassandraNamespace: cassDC.Namespace,
			DataCenterName:     cassDC.Name,
			ClusterName:        "test-kc",
			TelemetrySpec: &telemetryapi.TelemetrySpec{
				Prometheus: &telemetryapi.PrometheusTelemetrySpec{
					Enabled: true,
				},
			},
			Logger: testLogger,
		},
	}
	cfg.ServiceMonitorName = telemetry.GetCassandraPromSMName(cfg.CassTelemetryResourcer)
	kc := test.NewK8ssandraCluster(cfg.ClusterName, "test-kc-namespace")
	kc.Spec.Cassandra.Datacenters = []k8ssandraapi.CassandraDatacenterTemplate{
		{
			Meta: k8ssandraapi.EmbeddedObjectMeta{
				Namespace: cassDC.Namespace,
				Name:      cassDC.Name,
			},
			CassandraTelemetry: &telemetryapi.TelemetrySpec{
				Prometheus: &telemetryapi.PrometheusTelemetrySpec{
					Enabled: true,
				},
			},
		},
	}
	_, err := r.reconcileCassandraDCTelemetry(ctx, &kc, kc.Spec.Cassandra.Datacenters[0], &cassDC, testLogger, fakeClient)
	if err != nil {
		assert.Fail(t, "reconciliation failed", err)
	}
	currentSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Name: cfg.ServiceMonitorName, Namespace: cfg.CassandraNamespace}, currentSM); err != nil {
		assert.Fail(t, "could not get actual ServiceMonitor after reconciling k8ssandra cluster", err)
	}
	assert.NotEmpty(t, currentSM.Spec.Endpoints)
	assert.NotEqual(t, currentSM.Namespace, kc.Namespace)
}
