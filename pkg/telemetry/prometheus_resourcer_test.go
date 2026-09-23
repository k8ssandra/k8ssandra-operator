// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"

	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_PrometheusResourcer_UpdateResources_Create_CassDC tests that a serviceMonitor is created if one does not exist.
func Test_PrometheusResourcer_UpdateResources_Create_CassDC(t *testing.T) {
	fakeClient, err := test.NewFakeClient()
	if err != nil {
		assert.Fail(t, "could not create fake client", err)
	}
	ctx := context.Background()
	// Create K8ssandra cluster and pass through to PrometheusResourcer.UpdateResources()
	logger := testr.New(t)
	cfg := PrometheusResourcer{
		MonitoringTargetNS:   "test-namespace",
		MonitoringTargetName: "test-dc-name",
		Logger:               logger,
		ServiceMonitorName:   "test-servicemonitor",
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-k8ssandracluster"},
	}
	ownerCassDC := test.NewCassandraDatacenter("test-cassdc", "test-namespace")
	serviceMonitor, err := cfg.NewCassServiceMonitor(true)
	if err != nil {
		assert.Fail(t, "couldn't create new ServiceMonitor for CassDC", "error", err)
	}
	if err := cfg.UpdateResources(ctx, fakeClient, &ownerCassDC, serviceMonitor); err != nil {
		assert.Fail(t, "could not update resources as expected", err)
	}
	// Check that the expected resources were created.
	createdSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Namespace: cfg.MonitoringTargetNS, Name: cfg.ServiceMonitorName}, createdSM); err != nil {
		assert.Fail(t, "could not get expected ServiceMonitor", err)
	}
	assert.NotEmpty(t, createdSM)
	assert.Equal(t, cfg.ServiceMonitorName, createdSM.Name)
	assert.Equal(t, cfg.MonitoringTargetNS, createdSM.Namespace)
	assertNonBlockingOwnerReference(t, createdSM, ownerCassDC.Name)
}

// Test_PrometheusResourcer_UpdateResources_Update_CassDC tests that an out of date serviceMonitor is updated, and that
// a controller reference left by an earlier version of the operator is replaced with a non-blocking owner reference.
func Test_PrometheusResourcer_UpdateResources_Update_CassDC(t *testing.T) {
	fakeClient, err := test.NewFakeClient()
	require.NoError(t, err, "could not create fake client")
	ctx := context.Background()
	cfg := PrometheusResourcer{
		MonitoringTargetNS:   "test-namespace",
		MonitoringTargetName: "test-dc-name",
		Logger:               testr.New(t),
		ServiceMonitorName:   "test-servicemonitor",
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-k8ssandracluster"},
	}
	ownerCassDC := test.NewCassandraDatacenter("test-cassdc", "test-namespace")
	existingSM, err := cfg.NewCassServiceMonitor(true)
	require.NoError(t, err, "couldn't create new ServiceMonitor for CassDC")
	require.NoError(t, controllerutil.SetControllerReference(&ownerCassDC, existingSM, fakeClient.Scheme()))
	require.NoError(t, fakeClient.Create(ctx, existingSM))

	desiredSM, err := cfg.NewCassServiceMonitor(false)
	require.NoError(t, err, "couldn't create new ServiceMonitor for CassDC")
	require.NoError(t, cfg.UpdateResources(ctx, fakeClient, &ownerCassDC, desiredSM))

	updatedSM := &promapi.ServiceMonitor{}
	require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Namespace: cfg.MonitoringTargetNS, Name: cfg.ServiceMonitorName}, updatedSM))
	assert.Equal(t, desiredSM.Annotations[k8ssandraapi.ResourceHashAnnotation], updatedSM.Annotations[k8ssandraapi.ResourceHashAnnotation])
	assertNonBlockingOwnerReference(t, updatedSM, ownerCassDC.Name)
}

// assertNonBlockingOwnerReference checks the ServiceMonitor has a single non-controller owner reference to the CassandraDatacenter.
func assertNonBlockingOwnerReference(t *testing.T, sm *promapi.ServiceMonitor, ownerName string) {
	t.Helper()
	require.Len(t, sm.OwnerReferences, 1)
	owner := sm.OwnerReferences[0]
	assert.Equal(t, "CassandraDatacenter", owner.Kind)
	assert.Equal(t, ownerName, owner.Name)
	assert.Nil(t, owner.Controller)
	assert.Nil(t, owner.BlockOwnerDeletion)
}

// Test_PrometheusResourcer_Cleanup_CassDC tests that the Cleanup method on PrometheusResourcer correctly
// cleans up all resources it creates using its UpdateResources method.
func Test_PrometheusResourcer_Cleanup_CassDC(t *testing.T) {
	fakeClient, err := test.NewFakeClient()
	if err != nil {
		assert.Fail(t, "could not create fake client", err)
	}
	ctx := context.Background()
	testLogger := testr.New(t)
	// Create ServiceMonitor in the fakeClient
	cfg := PrometheusResourcer{
		MonitoringTargetNS:   "test-namespace",
		MonitoringTargetName: "test-dc-name",
		Logger:               testLogger,
		ServiceMonitorName:   "test-servicemonitor",
		CommonLabels:         map[string]string{k8ssandraapi.K8ssandraClusterNameLabel: "test-klustername"},
	}
	testSM, err := cfg.NewCassServiceMonitor(true)
	if err != nil {
		assert.Fail(t, "could not create service monitor", err)
	}
	err = fakeClient.Create(ctx, testSM)
	assert.NoError(t, err, "could not create resource as expected")
	// Ensure that the ServiceMonitor was created
	createdSM := &promapi.ServiceMonitor{}
	if err := fakeClient.Get(ctx, types.NamespacedName{Namespace: cfg.MonitoringTargetNS, Name: cfg.ServiceMonitorName}, createdSM); err != nil {
		assert.Fail(t, "could not get expected ServiceMonitor", err)
	}
	assert.Equal(t, cfg.ServiceMonitorName, createdSM.Name)
	// Clean up the ServiceMonitor and ensure it has been deleted
	err = cfg.CleanupResources(ctx, fakeClient)
	assert.NoError(t, err, "could not cleanup resources as expected")
	if err := fakeClient.Get(ctx, types.NamespacedName{Namespace: cfg.MonitoringTargetNS, Name: cfg.ServiceMonitorName}, createdSM); err != nil {
		assert.IsType(t, &errors.StatusError{}, err)
		return
	} else {
		assert.Fail(t, "We still found a resource that had not been cleaned up.")
	}
}
