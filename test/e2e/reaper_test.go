package e2e

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createSingleReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	reaperKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)

	t.Log("check Reaper keyspace created")
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[0], namespace, "test", "test-dc1-default-sts-0", "reaper_db")

	testDeleteReaperManually(t, f, ctx, kcKey, dcKey, reaperKey)
	testRemoveReaperFromK8ssandraCluster(t, f, ctx, kcKey, dcKey, reaperKey)

	t.Log("deploying Reaper ingress routes in", f.DataPlaneContexts[0])
	f.DeployReaperIngresses(t, ctx, f.DataPlaneContexts[0], namespace, "test-dc1-reaper-service")
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.DataPlaneContexts[0])
		reaperUiSecretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-reaper-ui"}}
		username, password := retrieveCredentials(t, f, ctx, reaperUiSecretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], "test", "reaper_db", username, password)
	})
}

func createSingleReaperWithEncryption(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	reaperKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)

	t.Log("deploying Reaper ingress routes in context", f.DataPlaneContexts[0])
	f.DeployReaperIngresses(t, ctx, f.DataPlaneContexts[0], namespace, "test-dc1-reaper-service")
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.DataPlaneContexts[0])
		reaperUiSecretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-reaper-ui"}}
		username, password := retrieveCredentials(t, f, ctx, reaperUiSecretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], "test", "reaper_db", username, password)
	})
}

func createMultiReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	uiSecretKey := types.NamespacedName{Namespace: namespace, Name: "reaper-ui-secret"}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}

	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	reaper1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}
	reaper2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-reaper"}}
	stargate1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	stargate2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-stargate"}}

	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)

	t.Log("check Stargate auth keyspace created in both clusters")
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[0], namespace, "test", "test-dc1-default-sts-0", stargate.AuthKeyspace)
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[1], namespace, "test", "test-dc2-default-sts-0", stargate.AuthKeyspace)

	t.Log("check Reaper custom keyspace created in both clusters")
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[0], namespace, "test", "test-dc1-default-sts-0", "reaper_ks")
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[1], namespace, "test", "test-dc2-default-sts-0", "reaper_ks")

	checkStargateReady(t, f, ctx, stargate1Key)
	checkStargateK8cStatusReady(t, f, ctx, kcKey, dc1Key)

	checkReaperReady(t, f, ctx, reaper1Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc1Key)

	checkStargateReady(t, f, ctx, stargate2Key)
	checkStargateK8cStatusReady(t, f, ctx, kcKey, dc2Key)

	checkReaperReady(t, f, ctx, reaper2Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc2Key)

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], namespace, "test")
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("check that nodes in dc1 see nodes in dc2")
	checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, "test-dc1-default-sts-0", 2, 0, "-u", username, "-pw", password)

	t.Log("check nodes in dc2 see nodes in dc1")
	checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, "test-dc2-default-sts-0", 2, 0, "-u", username, "-pw", password)

	t.Log("deploying Stargate and Reaper ingress routes in all data plane clusters")
	f.DeployReaperIngresses(t, ctx, f.DataPlaneContexts[0], namespace, "test-dc1-reaper-service")
	f.DeployReaperIngresses(t, ctx, f.DataPlaneContexts[1], namespace, "test-dc2-reaper-service")
	f.DeployStargateIngresses(t, f.DataPlaneContexts[0], namespace, "test-dc1-stargate-service", username, password)
	f.DeployStargateIngresses(t, f.DataPlaneContexts[1], namespace, "test-dc2-stargate-service", username, password)

	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[1], namespace)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], "test", "reaper_ks", username, password)
	})
	t.Run("TestReaperApi[1]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[1], "test", "reaper_ks", username, password)
	})

	replication := map[string]int{"dc1": 1, "dc2": 1}

	t.Run("TestStargateApi[0]", func(t *testing.T) {
		testStargateApis(t, ctx, f.DataPlaneContexts[0], username, password, replication)
	})
	t.Run("TestStargateApi[1]", func(t *testing.T) {
		testStargateApis(t, ctx, f.DataPlaneContexts[1], username, password, replication)
	})
}

func createMultiReaperWithEncryption(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	uiSecretKey := types.NamespacedName{Namespace: namespace, Name: "reaper-ui-secret"}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}

	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	reaper1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}
	reaper2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-reaper"}}

	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)

	checkReaperReady(t, f, ctx, reaper1Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc1Key)

	checkReaperReady(t, f, ctx, reaper2Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc2Key)

	t.Log("deploying Stargate and Reaper ingress routes in both clusters")
	f.DeployReaperIngresses(t, ctx, f.DataPlaneContexts[0], namespace, "test-dc1-reaper-service")
	f.DeployReaperIngresses(t, ctx, f.DataPlaneContexts[1], namespace, "test-dc2-reaper-service")

	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[1], namespace)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], "test", "reaper_ks", username, password)
	})
	t.Run("TestReaperApi[1]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[1], "test", "reaper_ks", username, password)
	})
}

func createReaperAndDatacenter(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	reaperKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "reaper1"}}

	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("create Reaper keyspace")
	_, err := f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, "test", "test-dc1-rack1-sts-0",
		"CREATE KEYSPACE reaper_db WITH REPLICATION = {'class' : 'NetworkTopologyStrategy', 'dc1' : 3} ")
	require.NoError(t, err, "failed to create Reaper keyspace")

	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[0], namespace, "test", "test-dc1-rack1-sts-0", "reaper_db")

	checkReaperReady(t, f, ctx, reaperKey)

	t.Log("deploying Reaper ingress routes in context", f.DataPlaneContexts[0])
	f.DeployReaperIngresses(t, ctx, f.DataPlaneContexts[0], namespace, "reaper1-service")
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.DataPlaneContexts[0])
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "reaper-ui-secret"}}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], "test", "reaper_db", username, password)
	})
}

func checkReaperReady(t *testing.T, f *framework.E2eFramework, ctx context.Context, reaperKey framework.ClusterKey) {
	t.Logf("check that Reaper %s in cluster %s is ready", reaperKey.Name, reaperKey.K8sContext)
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

func connectReaperApi(t *testing.T, ctx context.Context, k8sContext, clusterName, username, password string) reaperclient.Client {
	t.Logf("Testing Reaper API in context %v...", k8sContext)
	var reaperURL, _ = url.Parse(fmt.Sprintf("http://%s", ingressConfigs[k8sContext].ReaperRest))
	var reaperClient = reaperclient.NewClient(reaperURL)
	if username != "" {
		t.Logf("Logging into Reaper API in context %v...", k8sContext)
		err := reaperClient.Login(ctx, username, password)
		require.NoError(t, err, "failed to login into Reaper")
	}
	checkClusterIsRegisteredInReaper(t, ctx, clusterName, reaperClient)
	return reaperClient
}

func testReaperApi(t *testing.T, ctx context.Context, k8sContext, clusterName, keyspace, username, password string) {
	reaperClient := connectReaperApi(t, ctx, k8sContext, clusterName, username, password)
	repairId := triggerRepair(t, ctx, clusterName, keyspace, reaperClient)
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
