package k8ssandra

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// stopDc tests scenarios that involve stopping a CassandraDatacenter.
func stopDc(t *testing.T, ctx context.Context, f *framework.Framework, _ string) {
	t.Run("StopExistingDatacenter", stopDcTest(f, ctx, stopExistingDc))
	t.Run("AddAndStopDatacenter", stopDcTest(f, ctx, addAndStopDc))
}

type stopDcTestFunc func(t *testing.T, f *framework.Framework, ctx context.Context, kc *api.K8ssandraCluster)

func stopDcTest(f *framework.Framework, ctx context.Context, test stopDcTestFunc) func(*testing.T) {
	return func(t *testing.T) {
		namespace := rand.String(9)
		if err := f.CreateNamespace(namespace); err != nil {
			t.Fatalf("failed to create namespace %s: %v", namespace, err)
		}
		managementApiFactory.SetT(t)
		managementApiFactory.UseDefaultAdapter()
		kc := stopDcTestSetup(t, f, ctx, namespace)
		test(t, f, ctx, kc)
		err := f.DeleteK8ssandraCluster(ctx, utils.GetKey(kc), timeout, interval)
		require.NoError(t, err, "failed to delete K8ssandraCluster")
	}
}

func stopDcTestSetup(t *testing.T, f *framework.Framework, ctx context.Context, namespace string) *api.K8ssandraCluster {

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        "stop-dc-test",
			Annotations: map[string]string{api.AutomatedUpdateAnnotation: string(api.AllowUpdateAlways)},
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "4.0.1",
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{StorageClassName: &defaultStorageClass},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, K8sContext: f.DataPlaneContexts[0], Size: 3},
					{Meta: api.EmbeddedObjectMeta{Name: "dc2"}, K8sContext: f.DataPlaneContexts[1], Size: 3},
				},
			},
			Reaper: &reaperapi.ReaperClusterTemplate{},
		},
	}

	kcKey := utils.GetKey(kc)

	err := f.Client.Create(ctx, kc)
	require.NoError(t, err, "failed to create K8ssandraCluster")

	verifySuperuserSecretCreated(ctx, t, f, kc)
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
	dc2Key := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, "dc2")

	t.Log("check that dc1 was created")
	require.Eventually(t, f.DatacenterExists(ctx, dc1Key), timeout, interval)
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(t, err)
	assert.False(t, dc1.Spec.Stopped)
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(t, err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	require.Eventually(t, f.DatacenterExists(ctx, dc2Key), timeout, interval)
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(t, err)
	assert.False(t, dc2.Spec.Stopped)
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(t, err, "failed to set dc2 status ready")

	t.Log("check that dc2 was rebuilt")
	verifyRebuildTaskCreated(ctx, t, f, dc2Key, dc1Key)
	rebuildTaskKey := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, "dc2-rebuild")
	setRebuildTaskFinished(ctx, t, f, rebuildTaskKey, dc2Key)

	t.Log("wait for the CassandraInitialized condition to be set")
	require.Eventually(t, func() bool {
		err := f.Client.Get(ctx, kcKey, kc)
		require.NoError(t, err)
		return corev1.ConditionTrue == kc.Status.GetConditionStatus(api.CassandraInitialized)
	}, timeout, interval, "timed out waiting for CassandraInitialized condition check")

	// sg1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, kc.Name+"-dc1-stargate")
	// sg2Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, kc.Name+"-dc2-stargate")
	reaper1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, kc.Name+"-dc1-reaper")
	reaper2Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, kc.Name+"-dc2-reaper")

	// t.Log("check that stargate sg1 was created")
	// require.Eventually(t, f.StargateExists(ctx, sg1Key), timeout, interval)

	// t.Logf("update stargate sg1 status to ready")
	// err = f.SetStargateStatusReady(ctx, sg1Key)
	// require.NoError(t, err, "failed to patch stargate status")

	t.Log("check that reaper reaper1 was created")
	require.Eventually(t, f.ReaperExists(ctx, reaper1Key), timeout, interval)

	t.Logf("update reaper reaper1 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper1Key)
	require.NoError(t, err, "failed to patch reaper status")

	// t.Log("check that stargate sg2 is created")
	// require.Eventually(t, f.StargateExists(ctx, sg2Key), timeout, interval, "failed to verify stargate sg2 created")

	// t.Logf("update stargate sg2 status to ready")
	// err = f.SetStargateStatusReady(ctx, sg2Key)
	// require.NoError(t, err, "failed to patch stargate status")

	t.Log("check that reaper reaper2 is created")
	require.Eventually(t, f.ReaperExists(ctx, reaper2Key), timeout, interval, "failed to verify reaper reaper2 created")

	t.Logf("update reaper reaper2 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper2Key)
	require.NoError(t, err, "failed to patch reaper status")

	err = f.Client.Get(ctx, kcKey, kc)
	require.NoError(t, err)
	return kc
}

// stopDcManagementApiReset resets the mock for the Management API so that it only responds to calls to
// EnsureKeyspaceReplication that have the desired replication. Other calls with undesired replications would then
// panic. This ensures that stopping a DC does not modify the desired replication of keyspaces in the cluster.
func stopDcManagementApiReset(replication map[string]int) {
	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	// Only accept calls to EnsureKeyspaceReplication with the desired replication for system keyspaces:
	for _, keyspace := range api.SystemKeyspaces {
		mockMgmtApi.On(testutils.EnsureKeyspaceReplication, keyspace, replication).Return(nil)
	}
	// Same for Stargate and Reaper keyspaces:
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, stargate.AuthKeyspace, replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, reaperapi.DefaultKeyspace, replication).Return(nil)
	// Other stubs that we need to define:
	mockMgmtApi.On(testutils.GetSchemaVersions).Return(map[string][]string{"version1": {"host1"}, "UNREACHABLE": {"host2"}}, nil)
	mockMgmtApi.On(testutils.ListTables, stargate.AuthKeyspace).Return([]string{"token"}, nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return([]string{}, nil)
	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)
}

// stopExistingDc tests the creation of a new K8ssandraCluster containing 2 dcs, dc1 and dc2. dc1 is then stopped. It
// expects dc1 to be in stopped state, and its Stargate and Reaper resources to be deleted. It expects dc2 to remain
// deployed and ready at all times, along with its Stargate and Reaper resources.
func stopExistingDc(t *testing.T, f *framework.Framework, ctx context.Context, kc *api.K8ssandraCluster) {

	kcKey := utils.GetKey(kc)
	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, "dc1")
	// sg1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, kc.Name+"-dc1-stargate")
	// sg2Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, kc.Name+"-dc2-stargate")
	reaper1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, kc.Name+"-dc1-reaper")
	reaper2Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, kc.Name+"-dc2-reaper")

	replication := map[string]int{"dc1": 3, "dc2": 3}
	stopDcManagementApiReset(replication)

	t.Log("stop dc1")
	err := f.PatchK8ssandraCluster(ctx, kcKey, func(kc *api.K8ssandraCluster) {
		kc.Spec.Cassandra.Datacenters[0].Stopped = true
	})
	require.NoError(t, err, "failed to stop dc1")
	withDc1 := f.NewWithDatacenter(ctx, dc1Key)
	require.Eventually(t, withDc1(func(dc1 *cassdcapi.CassandraDatacenter) bool {
		return dc1.Spec.Stopped
	}), timeout, interval, "timeout waiting for dc1 to be stopped")
	err = f.SetDatacenterStatusStopped(ctx, dc1Key)
	require.NoError(t, err, "failed to set dc1 status stopped")

	t.Log("wait for the dc conditions to be met")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := f.Client.Get(ctx, kcKey, kc)
		assert.NoError(c, err)
		assert.Len(c, kc.Status.Datacenters, 2)
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc1"].Cassandra.GetConditionStatus(cassdcapi.DatacenterStopped))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc2"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
	}, timeout, interval, "timed out waiting for dc condition check")

	// t.Log("check that stargate sg1 was deleted")
	// f.AssertObjectDoesNotExist(ctx, t, sg1Key, &stargateapi.Stargate{}, timeout, interval)

	t.Log("check that reaper reaper1 was deleted")
	f.AssertObjectDoesNotExist(ctx, t, reaper1Key, &reaperapi.Reaper{}, timeout, interval)

	// t.Log("check that stargate sg2 is still present")
	// require.Eventually(t, f.StargateExists(ctx, sg2Key), timeout, interval, "failed to verify stargate sg2 created")

	t.Log("check that reaper reaper2 is still present")
	require.Eventually(t, f.ReaperExists(ctx, reaper2Key), timeout, interval, "failed to verify reaper reaper2 created")

	t.Log("start dc1")
	err = f.PatchK8ssandraCluster(ctx, kcKey, func(kc *api.K8ssandraCluster) {
		kc.Spec.Cassandra.Datacenters[0].Stopped = false
	})
	require.NoError(t, err, "failed to start dc1")
	require.Eventually(t, withDc1(func(dc1 *cassdcapi.CassandraDatacenter) bool {
		return !dc1.Spec.Stopped
	}), timeout, interval, "timeout waiting for dc1 to be started")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(t, err, "failed to set dc1 status ready")

	t.Log("wait for the dc conditions to be met")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := f.Client.Get(ctx, kcKey, kc)
		assert.NoError(c, err)
		assert.Len(c, kc.Status.Datacenters, 2)
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc1"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc2"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
	}, timeout, interval, "timed out waiting for dc condition check")

	// t.Log("check that stargate sg1 was created")
	// require.Eventually(t, f.StargateExists(ctx, sg1Key), timeout, interval)

	// t.Logf("update stargate sg1 status to ready")
	// err = f.SetStargateStatusReady(ctx, sg1Key)
	// require.NoError(t, err, "failed to patch stargate status")

	t.Log("check that reaper reaper1 was created")
	require.Eventually(t, f.ReaperExists(ctx, reaper1Key), timeout, interval)

	t.Logf("update reaper reaper1 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper1Key)
	require.NoError(t, err, "failed to patch reaper status")

	// t.Log("check that stargate sg2 is still present")
	// require.Eventually(t, f.StargateExists(ctx, sg2Key), timeout, interval, "failed to verify stargate sg2 created")

	t.Log("check that reaper reaper2 is still present")
	require.Eventually(t, f.ReaperExists(ctx, reaper2Key), timeout, interval, "failed to verify reaper reaper2 created")
}

func addAndStopDc(t *testing.T, f *framework.Framework, ctx context.Context, kc *api.K8ssandraCluster) {

	kcKey := utils.GetKey(kc)
	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, "dc1")
	dc3Key := framework.NewClusterKey(f.DataPlaneContexts[2], kc.Namespace, "dc3")
	// sg1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, kc.Name+"-dc1-stargate")
	// sg2Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, kc.Name+"-dc2-stargate")
	// sg3Key := framework.NewClusterKey(f.DataPlaneContexts[2], kc.Namespace, kc.Name+"-dc3-stargate")
	reaper1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, kc.Name+"-dc1-reaper")
	reaper2Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, kc.Name+"-dc2-reaper")
	reaper3Key := framework.NewClusterKey(f.DataPlaneContexts[2], kc.Namespace, kc.Name+"-dc3-reaper")

	t.Log("add dc3")
	err := f.PatchK8ssandraCluster(ctx, kcKey, func(kc *api.K8ssandraCluster) {
		kc.Spec.Cassandra.Datacenters = append(
			kc.Spec.Cassandra.Datacenters,
			api.CassandraDatacenterTemplate{Meta: api.EmbeddedObjectMeta{Name: "dc3"}, K8sContext: f.DataPlaneContexts[2], Size: 3},
		)
	})
	require.NoError(t, err, "failed to add dc3")

	err = f.Client.Get(ctx, kcKey, kc)
	require.NoError(t, err, "failed to get k8ssandra cluster")

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc3 was created")
	require.Eventually(t, f.DatacenterExists(ctx, dc3Key), timeout, interval, "failed to verify dc3 was created")
	err = f.SetDatacenterStatusReady(ctx, dc3Key)
	require.NoError(t, err, "failed to set dc3 status ready")

	t.Log("check that dc3 was rebuilt")
	verifyRebuildTaskCreated(ctx, t, f, dc3Key, dc1Key)
	rebuildTaskKey := framework.NewClusterKey(f.DataPlaneContexts[2], kc.Namespace, "dc3-rebuild")
	setRebuildTaskFinished(ctx, t, f, rebuildTaskKey, dc3Key)

	// t.Log("check that stargate sg3 was created")
	// require.Eventually(t, f.StargateExists(ctx, sg3Key), timeout, interval)

	// t.Logf("update stargate sg3 status to ready")
	// err = f.SetStargateStatusReady(ctx, sg3Key)
	// require.NoError(t, err, "failed to patch stargate status")

	t.Log("check that reaper reaper3 was created")
	require.Eventually(t, f.ReaperExists(ctx, reaper3Key), timeout, interval)

	t.Logf("update reaper reaper3 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper3Key)
	require.NoError(t, err, "failed to patch reaper status")

	t.Log("wait for the dc conditions to be met")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := f.Client.Get(ctx, kcKey, kc)
		assert.NoError(c, err)
		assert.Len(c, kc.Status.Datacenters, 3)
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc1"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc2"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc3"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		// assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc1"].Stargate.GetConditionStatus(stargateapi.StargateReady))
		// assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc2"].Stargate.GetConditionStatus(stargateapi.StargateReady))
		// assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc3"].Stargate.GetConditionStatus(stargateapi.StargateReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc1"].Reaper.GetConditionStatus(reaperapi.ReaperReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc2"].Reaper.GetConditionStatus(reaperapi.ReaperReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc3"].Reaper.GetConditionStatus(reaperapi.ReaperReady))
	}, timeout, interval, "timed out waiting for dc condition check")

	// From now on, only expect the updated replication including dc3 to be enforced on all keyspaces
	replication := map[string]int{"dc1": 3, "dc2": 3, "dc3": 3}
	stopDcManagementApiReset(replication)

	t.Log("stop dc3")
	err = f.PatchK8ssandraCluster(ctx, kcKey, func(kc *api.K8ssandraCluster) {
		kc.Spec.Cassandra.Datacenters[2].Stopped = true
	})
	require.NoError(t, err, "failed to stop dc3")
	withDc3 := f.NewWithDatacenter(ctx, dc3Key)
	require.Eventually(t, withDc3(func(dc3 *cassdcapi.CassandraDatacenter) bool {
		return dc3.Spec.Stopped
	}), timeout, interval, "timeout waiting for dc3 to be stopped")
	err = f.SetDatacenterStatusStopped(ctx, dc3Key)
	require.NoError(t, err, "failed to set dc3 status stopped")

	t.Log("wait for the dc conditions to be met")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := f.Client.Get(ctx, kcKey, kc)
		assert.NoError(c, err)
		assert.Len(c, kc.Status.Datacenters, 3)
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc1"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc2"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc3"].Cassandra.GetConditionStatus(cassdcapi.DatacenterStopped))
	}, timeout, interval, "timed out waiting for dc condition check")

	// t.Log("check that stargate sg1 is still present")
	// require.Eventually(t, f.StargateExists(ctx, sg1Key), timeout, interval, "failed to verify stargate sg1 created")

	t.Log("check that reaper reaper1 is still present")
	require.Eventually(t, f.ReaperExists(ctx, reaper1Key), timeout, interval, "failed to verify reaper reaper1 created")

	// t.Log("check that stargate sg2 is still present")
	// require.Eventually(t, f.StargateExists(ctx, sg2Key), timeout, interval, "failed to verify stargate sg2 created")

	t.Log("check that reaper reaper2 is still present")
	require.Eventually(t, f.ReaperExists(ctx, reaper2Key), timeout, interval, "failed to verify reaper reaper2 created")

	// t.Log("check that stargate sg3 was deleted")
	// f.AssertObjectDoesNotExist(ctx, t, sg3Key, &stargateapi.Stargate{}, timeout, interval)

	t.Log("check that reaper reaper3 was deleted")
	f.AssertObjectDoesNotExist(ctx, t, reaper3Key, &reaperapi.Reaper{}, timeout, interval)

	t.Log("start dc3")
	err = f.PatchK8ssandraCluster(ctx, kcKey, func(kc *api.K8ssandraCluster) {
		kc.Spec.Cassandra.Datacenters[2].Stopped = false
	})
	require.NoError(t, err, "failed to start dc3")
	require.Eventually(t, withDc3(func(dc3 *cassdcapi.CassandraDatacenter) bool {
		return !dc3.Spec.Stopped
	}), timeout, interval, "timeout waiting for dc3 to be stopped")
	err = f.SetDatacenterStatusReady(ctx, dc3Key)
	require.NoError(t, err, "failed to set dc3 status ready")

	t.Log("wait for the dc conditions to be met")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		err := f.Client.Get(ctx, kcKey, kc)
		assert.NoError(c, err)
		assert.Len(c, kc.Status.Datacenters, 3)
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc1"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc2"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
		assert.Equal(c, corev1.ConditionTrue, kc.Status.Datacenters["dc3"].Cassandra.GetConditionStatus(cassdcapi.DatacenterReady))
	}, timeout, interval, "timed out waiting for dc condition check")

	// t.Log("check that stargate sg1 is still present")
	// require.Eventually(t, f.StargateExists(ctx, sg1Key), timeout, interval, "failed to verify stargate sg1 created")

	t.Log("check that reaper reaper1 is still present")
	require.Eventually(t, f.ReaperExists(ctx, reaper1Key), timeout, interval, "failed to verify reaper reaper1 created")

	// t.Log("check that stargate sg2 is still present")
	// require.Eventually(t, f.StargateExists(ctx, sg2Key), timeout, interval, "failed to verify stargate sg2 created")

	t.Log("check that reaper reaper2 is still present")
	require.Eventually(t, f.ReaperExists(ctx, reaper2Key), timeout, interval, "failed to verify reaper reaper2 created")

	t.Log("check that dc3 was rebuilt")
	verifyRebuildTaskCreated(ctx, t, f, dc3Key, dc1Key)
	rebuildTaskKey = framework.NewClusterKey(f.DataPlaneContexts[2], kc.Namespace, "dc3-rebuild")
	setRebuildTaskFinished(ctx, t, f, rebuildTaskKey, dc3Key)

	// t.Log("check that stargate sg3 was created")
	// require.Eventually(t, f.StargateExists(ctx, sg3Key), timeout, interval)

	// t.Logf("update stargate sg3 status to ready")
	// err = f.SetStargateStatusReady(ctx, sg3Key)
	// require.NoError(t, err, "failed to patch stargate status")

	t.Log("check that reaper reaper3 was created")
	require.Eventually(t, f.ReaperExists(ctx, reaper3Key), timeout, interval)

	t.Logf("update reaper reaper3 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper3Key)
	require.NoError(t, err, "failed to patch reaper status")
}
