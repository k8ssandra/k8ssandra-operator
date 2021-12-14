// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"context"
	testlogr "github.com/go-logr/logr/testing"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"testing"

	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
)

//Test_NewServiceMonitor_SUCCESS tests that a new service monitor is successfully returned.
func Test_NewServiceMonitor_SUCCESS(t *testing.T) {
	cfg := CassPrometheusResourcer{
		CassTelemetryResourcer: CassTelemetryResourcer{
			CassandraNamespace: "test-namespace",
			DataCenterName:     "test-dc-name",
			ClusterName:        "test-cluster-name",
			TelemetrySpec: &telemetryapi.TelemetrySpec{
				Prometheus: &telemetryapi.PrometheusTelemetrySpec{
					Enabled: true,
				},
			},
		},
		ServiceMonitorName: "test-servicemonitor",
	}
	actualSM, err := cfg.NewServiceMonitor()
	if err != nil {
		assert.Fail(t, "error creating new service monitor", err)
	}
	assert.Equal(t, "prometheus", actualSM.Spec.Endpoints[0].Port)
}

// Test_CassPrometheusResourcer_IS_TelemetryResourcer tests that CassPrometheusResourcer implements the Resourcer interface.
func Test_CassPrometheusResourcer_IS_TelemetryResourcer(t *testing.T) {
	assert.Implements(t, (*Resourcer)(nil), CassPrometheusResourcer{})
}

// Test_CassPrometheusResourcer_UpdateResources_Create_SUCCESS tests that a serviceMonitor is created if one does not exist.
func Test_CassPrometheusResourcer_UpdateResources_Create_SUCCESS(t *testing.T) {
	fakeClient, err := test.NewFakeClient()
	if err != nil {
		assert.Fail(t, "could not create fake client", err)
	}
	ctx := context.Background()
	testLogger := testlogr.TestLogger{T: t}
	// Create k8ssandra cluster and pass through to CassPrometheusResourcer.UpdateResources()
	cfg := CassPrometheusResourcer{
		CassTelemetryResourcer: newCassTelemetryResourcer(testLogger),
		CommonLabels:           nil,
		ServiceMonitorName:     "test-sm",
	}
	ownerCassDC := test.NewCassandraDatacenter()
	if cfg.UpdateResources(ctx, fakeClient, &ownerCassDC); err != nil {
		assert.Fail(t, "could not update resources as expected", err)
	}
	// Check that the expected resources were created.
	createdSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Namespace: cfg.CassandraNamespace, Name: "test-sm"}, createdSM); err != nil {
		assert.Fail(t, "could not get expected ServiceMonitor", err)
	}
	assert.NotEmpty(t, createdSM)
	assert.Equal(t, "test-sm", createdSM.Name)
	assert.Equal(t, "test-namespace", createdSM.Namespace)
}

// TODO: This test not currently passing. We need to look at whether evaluating the resourceHash is sufficient to trigger healing.
// Test_CassPrometheusResourcer_Cleanup_SUCCESS tests that the servicemonitor is cleaned up successfully,
// when the TelemetrySpec is no longer in the CassPrometheusResourcer config.
func Test_CassPrometheusResourcer_Cleanup(t *testing.T) {
	fakeClient, err := test.NewFakeClient()
	if err != nil {
		assert.Fail(t, "could not create fake client", err)
	}
	ctx := context.Background()
	testLogger := testlogr.TestLogger{T: t}
	// Create ServiceMonitor in the fakeClient
	cfg := CassPrometheusResourcer{
		CassTelemetryResourcer: newCassTelemetryResourcer(testLogger),
		CommonLabels:           nil,
		ServiceMonitorName:     "test-sm",
	}
	testSM, err := cfg.NewServiceMonitor()
	if err != nil {
		assert.Fail(t, "could not create service monitor", err)
	}
	fakeClient.Create(ctx, testSM)
	// Ensure that the ServiceMonitor was created
	createdSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Namespace: cfg.CassandraNamespace, Name: "test-sm"}, createdSM); err != nil {
		assert.Fail(t, "could not get expected ServiceMonitor", err)
	}
	assert.Equal(t, "test-sm", createdSM.Name)
	// Clean up the ServiceMonitor and ensure it has been deleted
	cfg.CleanupResources(ctx, fakeClient)
	if err := fakeClient.Get(ctx, types.NamespacedName{Namespace: cfg.CassandraNamespace, Name: "test-sm"}, createdSM); err != nil {
		assert.IsType(t, &errors.StatusError{}, err)
		return
	} else {
		assert.Fail(t, "We still found a resource that had not been cleaned up.")
	}
}
