package e2e

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func createSingleReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	reaperKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)

	t.Log("check reaper keyspace created")
	f.CheckKeyspaceExists(t, ctx, "kind-k8ssandra-0", namespace, "test", "test-dc1-default-sts-0", "reaper_db")

	testDeleteReaperManually(t, f, ctx, kcKey, dcKey, reaperKey)
	testRemoveReaperFromK8ssandraCluster(t, f, ctx, kcKey, dcKey, reaperKey)

	t.Log("deploying Reaper ingress routes in kind-k8ssandra-0")
	f.DeployReaperIngresses(t, ctx, "kind-k8ssandra-0", 0, namespace, "test-dc1-reaper-service")
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)

	testReaperApi(t, ctx, 0, "reaper_db")
}

func createMultiReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	cqlSecretKey := types.NamespacedName{Namespace: namespace, Name: "reaper-cql-secret"}
	jmxSecretKey := types.NamespacedName{Namespace: namespace, Name: "reaper-jmx-secret"}

	checkSecretExists(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: cqlSecretKey})
	checkSecretExists(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: cqlSecretKey})
	checkSecretExists(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: jmxSecretKey})
	checkSecretExists(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: jmxSecretKey})

	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	reaper1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}
	reaper2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-reaper"}}

	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)

	checkReaperReady(t, f, ctx, reaper1Key)
	checkReaperReady(t, f, ctx, reaper2Key)

	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc1Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc2Key)

	t.Log("check reaper keyspace created")
	f.CheckKeyspaceExists(t, ctx, "kind-k8ssandra-0", namespace, "test", "test-dc1-default-sts-0", "reaper_ks")
	f.CheckKeyspaceExists(t, ctx, "kind-k8ssandra-1", namespace, "test", "test-dc2-default-sts-0", "reaper_ks")

	t.Log("deploying Reaper ingress routes in kind-k8ssandra-0 and kind-k8ssandra-1")
	f.DeployReaperIngresses(t, ctx, "kind-k8ssandra-0", 0, namespace, "test-dc1-reaper-service")
	f.DeployReaperIngresses(t, ctx, "kind-k8ssandra-1", 1, namespace, "test-dc2-reaper-service")
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-1", namespace)

	testReaperApi(t, ctx, 0, "reaper_ks")
	testReaperApi(t, ctx, 1, "reaper_ks")
}

func checkSecretExists(t *testing.T, f *framework.E2eFramework, ctx context.Context, secretKey framework.ClusterKey) {
	secret := &corev1.Secret{}
	require.Eventually(t, func() bool {
		return f.Get(ctx, secretKey, secret) == nil
	}, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval)
	assert.True(t, utils.IsManagedBy(secret, "test"), "secret is not managed by k8c")
}

func checkReaperReady(t *testing.T, f *framework.E2eFramework, ctx context.Context, reaperKey framework.ClusterKey) {
	t.Log("check Reaper status updated to ready")
	withReaper := f.NewWithReaper(ctx, reaperKey)
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return reaper.Status.Progress == reaperapi.ReaperProgressRunning && reaper.Status.IsReady()
	}), polling.reaperReady.timeout, polling.reaperReady.interval)
}

func checkReaperK8cStatusReady(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dcKey framework.ClusterKey,
) {
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
			kdcStatus.Reaper.Progress == reaperapi.ReaperProgressRunning &&
			kdcStatus.Reaper.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")
}

func testDeleteReaperManually(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dcKey framework.ClusterKey,
	reaperKey framework.ClusterKey,
) {
	t.Log("check that if Reaper is deleted directly it gets re-created")
	reaper := &reaperapi.Reaper{}
	err := f.Client.Get(ctx, reaperKey.NamespacedName, reaper)
	require.NoError(t, err, "failed to get Reaper in namespace %s", reaperKey.Namespace)
	err = f.Client.Delete(ctx, reaper)
	require.NoError(t, err, "failed to delete Reaper in namespace %s", reaperKey.Namespace)
	checkDatacenterReady(t, ctx, dcKey, f)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)
}

func testRemoveReaperFromK8ssandraCluster(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dcKey framework.ClusterKey,
	reaperKey framework.ClusterKey,
) {
	t.Log("delete Reaper in k8ssandracluster CRD")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", kcKey.Namespace)
	patch := client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	reaperTemplate := k8ssandra.Spec.Reaper
	k8ssandra.Spec.Reaper = nil
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster in namespace %s", kcKey.Namespace)

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
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", kcKey.Namespace)
	patch = client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	k8ssandra.Spec.Reaper = reaperTemplate.DeepCopy()
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster in namespace %s", kcKey.Namespace)

	t.Log("check Reaper re-created")
	withReaper := f.NewWithReaper(ctx, reaperKey)
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return true
	}), polling.reaperReady.timeout, polling.reaperReady.interval)

	checkDatacenterReady(t, ctx, dcKey, f)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)
}

func testReaperApi(t *testing.T, ctx context.Context, k8sContextIdx int, keyspace string) {
	t.Logf("Testing Reaper API in context kind-k8ssandra-%v...", k8sContextIdx)
	var reaperURL, _ = url.Parse(fmt.Sprintf("http://reaper.127.0.0.1.nip.io:3%d080", k8sContextIdx))
	var reaperClient = reaperclient.NewClient(reaperURL)
	checkClusterIsRegisteredInReaper(t, ctx, "test", reaperClient)
	repairId := triggerRepair(t, ctx, "test", keyspace, reaperClient)
	t.Log("Waiting for one segment to be repaired and canceling run")
	waitForOneSegmentToBeDone(t, ctx, repairId, reaperClient)
	err := reaperClient.AbortRepairRun(ctx, repairId)
	require.NoErrorf(t, err, "Failed to abort repair run %s: %s", repairId, err)
}

func checkClusterIsRegisteredInReaper(t *testing.T, ctx context.Context, clusterName string, reaperClient reaperclient.Client) {
	require.Eventually(t, func() bool {
		_, err := reaperClient.GetCluster(ctx, clusterName)
		return err == nil
	}, polling.reaperReady.timeout, polling.reaperReady.interval, "Cluster wasn't properly registered in Reaper")
}

func triggerRepair(t *testing.T, ctx context.Context, clusterName, keyspace string, reaperClient reaperclient.Client) uuid.UUID {
	t.Log("Starting a repair")
	options := &reaperclient.RepairRunCreateOptions{SegmentCountPerNode: 5}
	repairId, err := reaperClient.CreateRepairRun(ctx, clusterName, keyspace, "k8ssandra", options)
	require.NoErrorf(t, err, "Failed to create repair run: %s", err)
	// Start the previously created repair run
	err = reaperClient.StartRepairRun(ctx, repairId)
	require.NoErrorf(t, err, "Failed to start repair run %s: %s", repairId, err)
	return repairId
}

func waitForOneSegmentToBeDone(t *testing.T, ctx context.Context, repairId uuid.UUID, reaperClient reaperclient.Client) {
	require.Eventually(t, func() bool {
		if segments, err := reaperClient.RepairRunSegments(ctx, repairId); err == nil {
			for _, segment := range segments {
				if segment.State == reaperclient.RepairSegmentStateDone {
					return true
				}
			}
		}
		return false
	}, polling.reaperReady.timeout, polling.reaperReady.interval, "No repair segment was fully processed within timeout")
}
