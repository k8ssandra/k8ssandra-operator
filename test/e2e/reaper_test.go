package e2e

import (
	"context"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func createSingleReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("check k8ssandra cluster status updated for CassandraDatacenter")
	assert.Eventually(t, func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		}
		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		return found && kdcStatus.Cassandra != nil && cassandraDatacenterReady(kdcStatus.Cassandra)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	reaperKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}
	withReaper := f.NewWithReaper(ctx, reaperKey)

	t.Log("check Reaper status updated to ready")
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return reaper.Status.IsReady()
	}), polling.reaperReady.timeout, polling.reaperReady.interval)

	t.Log("check k8ssandra cluster status updated for Reaper")
	assert.Eventually(t, func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		}
		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		return found &&
			kdcStatus.Cassandra != nil &&
			cassandraDatacenterReady(kdcStatus.Cassandra) &&
			kdcStatus.Reaper != nil &&
			kdcStatus.Reaper.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	t.Log("check that if Reaper is deleted directly it gets re-created")
	reaper := &reaperapi.Reaper{}
	err = f.Client.Get(ctx, reaperKey.NamespacedName, reaper)
	require.NoError(t, err, "failed to get Reaper in namespace %s", namespace)
	err = f.Client.Delete(ctx, reaper)
	require.NoError(t, err, "failed to delete Reaper in namespace %s", namespace)

	t.Log("check that Reaper is re-created and ready")
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return reaper.Status.IsReady()
	}), polling.reaperReady.timeout, polling.reaperReady.interval, "timed out waiting for Reaper test-dc1-reaper to become ready")

	t.Log("delete Reaper in k8ssandracluster CRD")
	err = f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)
	patch := client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	reaperTemplate := k8ssandra.Spec.Reaper
	k8ssandra.Spec.Reaper = nil
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster in namespace %s", namespace)

	t.Log("check Reaper deleted")
	require.Eventually(t, func() bool {
		reaper := &reaperapi.Reaper{}
		err := f.Client.Get(ctx, reaperKey.NamespacedName, reaper)
		return err != nil && errors.IsNotFound(err)
	}, polling.reaperReady.timeout, polling.reaperReady.interval)

	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("check Reaper status deleted in k8ssandracluster resource")
	require.Eventually(t, func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		}
		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		return found && kdcStatus.Reaper == nil
	}, polling.reaperReady.timeout, polling.reaperReady.interval)

	t.Log("re-create Reaper in k8ssandracluster resource")
	err = f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)
	patch = client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	k8ssandra.Spec.Reaper = reaperTemplate.DeepCopy()
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster in namespace %s", namespace)

	t.Log("check Reaper re-created")
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return true
	}), polling.reaperReady.timeout, polling.reaperReady.interval)

	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("check Reaper status updated to ready")
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return reaper.Status.IsReady()
	}), polling.reaperReady.timeout, polling.reaperReady.interval)

	t.Log("check k8ssandra cluster status updated for Reaper")
	require.Eventually(t, func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		}
		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		return found &&
			kdcStatus.Cassandra != nil &&
			cassandraDatacenterReady(kdcStatus.Cassandra) &&
			kdcStatus.Reaper != nil &&
			kdcStatus.Reaper.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval)
}

func createMultiReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	t.Log("TODO")
}
