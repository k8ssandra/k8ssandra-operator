package k8ssandra

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	kerrors "github.com/k8ssandra/k8ssandra-operator/pkg/errors"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// addDc tests scenarios that involve adding a new CassandraDatacenter to an existing
// K8ssandraCluster.
func addDc(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	t.Run("WithUserKeyspaces", addDcTest(ctx, f, withUserKeyspaces, true))
	t.Run("WithStargateAndReaper", addDcTest(ctx, f, withStargateAndReaper, true))
	t.Run("FailSystemKeyspaceUpdate", addDcTest(ctx, f, failSystemKeyspaceUpdate, true))
	t.Run("FailUserKeyspaceUpdate", addDcTest(ctx, f, failUserKeyspaceUpdate, true))
	t.Run("SchemaDisagreementOnSystemKeyspaces", addDcTest(ctx, f, schemaDisagreementOnSystemKeyspaces, true))
	t.Run("SchemaDisagreementOnStargate", addDcTest(ctx, f, schemaDisagreementOnStargate, true))
	t.Run("ConfigureSrcDcForRebuild", addDcTest(ctx, f, configureSrcDcForRebuild, false))
	t.Run("DeleteDcWithUserKeyspaces", addDcTest(ctx, f, deleteDcWithUserKeyspaces, false))
	t.Run("DeleteDcWithStargateAndReaper", addDcTest(ctx, f, deleteDcWithStargateAndReaper, false))
}

type addDcTestFunc func(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster)

func addDcSetupForSingleDc(ctx context.Context, t *testing.T, f *framework.Framework, namespace string) *api.K8ssandraCluster {
	require := require.New(t)
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "add-dc-test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "4.0.1",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
							},
						},
					},
				},
			},
		},
	}
	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}

	createSuperuserSecret(ctx, t, f, kcKey, kc.Name)

	createReplicatedSecret(ctx, t, f, kcKey, f.DataPlaneContexts[0])
	setReplicationStatusDone(ctx, t, f, kcKey)

	createCassandraDatacenter(ctx, t, f, kc, 0)

	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}

	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dc1Key, dc)
	require.NoError(err)

	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	err = f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	t.Log("wait for the CassandraInitialized condition to be set")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, kcKey, kc)
		if err != nil {
			return false
		}
		initialized := kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionTrue
		return initialized && len(kc.Status.Datacenters) > 0
	}, timeout, interval, "timed out waiting for CassandraInitialized condition check")

	return kc
}

func addDcSetupForMultiDc(ctx context.Context, t *testing.T, f *framework.Framework, namespace string) *api.K8ssandraCluster {
	require := require.New(t)
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "add-dc-test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "4.0.1",
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       3,
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       3,
					},
				},
			},
		},
	}
	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}

	createSuperuserSecret(ctx, t, f, kcKey, kc.Name)

	createReplicatedSecret(ctx, t, f, kcKey, f.DataPlaneContexts[0], f.DataPlaneContexts[1])
	setReplicationStatusDone(ctx, t, f, kcKey)

	createCassandraDatacenter(ctx, t, f, kc, 0)

	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dc1Key, dc)
	require.NoError(err)

	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	createCassandraDatacenter(ctx, t, f, kc, 1)

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
	dc = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc)
	require.NoError(err)

	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	err = f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	t.Log("wait for the CassandraInitialized condition to be set")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, kcKey, kc)
		if err != nil {
			return false
		}
		initialized := kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionTrue
		return initialized && len(kc.Status.Datacenters) > 1
	}, timeout, interval, "timed out waiting for CassandraInitialized condition check")

	return kc
}

func addDcTest(ctx context.Context, f *framework.Framework, test addDcTestFunc, single bool) func(*testing.T) {
	return func(t *testing.T) {
		managementApiFactory.SetT(t)
		managementApiFactory.UseDefaultAdapter()

		namespace := framework.CleanupForKubernetes(rand.String(9))
		if err := f.CreateNamespace(namespace); err != nil {
			t.Fatalf("failed to create namespace %s: %v", namespace, err)
		}
		var kc *api.K8ssandraCluster
		if single {
			kc = addDcSetupForSingleDc(ctx, t, f, namespace)
		} else {
			kc = addDcSetupForMultiDc(ctx, t, f, namespace)
		}

		test(ctx, t, f, kc)

		if err := f.DeleteK8ssandraCluster(ctx, utils.GetKey(kc), timeout, interval); err != nil {
			t.Fatalf("failed to delete k8ssandracluster: %v", err)
		}
	}
}

// withUserKeyspaces tests adding a DC to a cluster that has user-defined keyspaces. This
// is a happy path test.
func withUserKeyspaces(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3}
	updatedReplication := map[string]int{"dc1": 3, "dc2": 3}
	// We need a version of the map with string values because GetKeyspaceReplication returns
	// a map[string]string.
	updatedReplicationStr := map[string]string{"class": cassandra.NetworkTopology, "dc1": "3", "dc2": "3"}

	userKeyspaces := []string{"ks1", "ks2"}

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return(userKeyspaces, nil)

	for _, ks := range userKeyspaces {
		mockMgmtApi.On(testutils.EnsureKeyspaceReplication, ks, updatedReplication).Return(nil)
		mockMgmtApi.On(testutils.GetKeyspaceReplication, ks).Return(updatedReplicationStr, nil)
	}

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDcToCluster(ctx, t, f, kc, dc2Key)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval, "failed to verify dc2 was created")

	t.Log("update dc2 status to ready")
	err := f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	verifyReplicationOfSystemKeyspacesUpdated(t, mockMgmtApi, replication, updatedReplication)

	for _, ks := range userKeyspaces {
		verifyReplicationOfKeyspaceUpdated(t, mockMgmtApi, ks, updatedReplication)
	}

	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}

	verifyRebuildTaskCreated(ctx, t, f, dc2Key, dc1Key)
}

// schemaDisagreementOnSystemKeyspaces verifies that the rebuild task is not created when the
// EnsureKeyspaceReplication call fails with the updated replication. The failure is intended to simulate a schema
// disagreement.
func schemaDisagreementOnSystemKeyspaces(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3}
	updatedReplication := map[string]int{"dc1": 3, "dc2": 3}

	schemaDisagreementErr := kerrors.NewSchemaDisagreementError("system keyspace check failed")

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(schemaDisagreementErr)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(schemaDisagreementErr)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(schemaDisagreementErr)

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDcToCluster(ctx, t, f, kc, dc2Key)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval, "failed to verify dc2 was created")

	t.Log("update dc2 status to ready")
	err := f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	taskKey := framework.ClusterKey{
		NamespacedName: types.NamespacedName{
			Namespace: dc2Key.Namespace,
			Name:      dc2Key.Name + "-rebuild",
		},
		K8sContext: dc2Key.K8sContext,
	}
	f.AssertObjectDoesNotExist(ctx, t, taskKey, &cassctlapi.CassandraTask{}, timeout, interval)
}

// configureSrcDcForRebuild tests adding a DC to a cluster and setting the
// api.RebuildSourceDcAnnotation annotation. The test verifies that the rebuild task is
// configured with the specified source dc.
func configureSrcDcForRebuild(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3, "dc2": 3}
	updatedReplication := map[string]int{"dc1": 3, "dc2": 3, "dc3": 3}

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return([]string{}, nil)

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	dc3Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc3"}, K8sContext: f.DataPlaneContexts[2]}

	kcKey := utils.GetKey(kc)
	kc = &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster")

	kc.Annotations[api.RebuildSourceDcAnnotation] = "dc2"
	err = f.Client.Update(ctx, kc)
	require.NoError(err, "failed to add %s annotation to K8ssandraCluster", api.RebuildSourceDcAnnotation)

	addDcToCluster(ctx, t, f, kc, dc3Key)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc3 was created")
	require.Eventually(f.DatacenterExists(ctx, dc3Key), timeout, interval, "failed to verify dc2 was created")

	t.Log("update dc3 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc3Key)
	require.NoError(err, "failed to set dc3 status ready")

	verifyReplicationOfSystemKeyspacesUpdated(t, mockMgmtApi, replication, updatedReplication)

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	verifyRebuildTaskCreated(ctx, t, f, dc3Key, dc2Key)

	rebuildTaskKey := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[2],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      "dc3-rebuild",
		},
	}
	setRebuildTaskFinished(ctx, t, f, rebuildTaskKey, dc3Key)
}

// withStargateAndReaper tests adding a DC to a cluster that also has Stargate and Reaper
// deployed. There are internal keyspaces for both Stargate and Reaper that this test
// verifies get updated. They are internal in that they are created and have their
// replication managed by the operator like Cassandra's system keyspaces. The test also
// verifies that Stargate and Reaper are deployed in the new DC after the rebuild finishes.
func withStargateAndReaper(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3}
	updatedReplication := map[string]int{"dc1": 3, "dc2": 3}

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, stargate.AuthKeyspace, replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, stargate.AuthKeyspace, updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, reaperapi.DefaultKeyspace, replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, reaperapi.DefaultKeyspace, updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.ListTables, stargate.AuthKeyspace).Return([]string{stargate.AuthTable}, nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return([]string{}, nil)

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	addStargateAndReaperToCluster(ctx, t, f, kc)

	sg1Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[0],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc1-stargate",
		},
	}

	t.Log("check that stargate sg1 is created")
	require.Eventually(f.StargateExists(ctx, sg1Key), timeout, interval)

	t.Logf("update stargate sg1 status to ready")
	err := f.SetStargateStatusReady(ctx, sg1Key)
	require.NoError(err, "failed to patch stargate status")

	reaper1Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[0],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc1-reaper",
		},
	}

	t.Log("check that reaper reaper1 is created")
	require.Eventually(f.ReaperExists(ctx, reaper1Key), timeout, interval)

	t.Logf("update reaper reaper1 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper1Key)
	require.NoError(err, "failed to patch reaper status")

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDcToCluster(ctx, t, f, kc, dc2Key)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval, "failed to verify dc2 was created")

	t.Log("update dc2 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	verifyReplicationOfInternalKeyspacesUpdated(t, mockMgmtApi, replication, updatedReplication)

	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}

	verifyRebuildTaskCreated(ctx, t, f, dc2Key, dc1Key)

	rebuildTaskKey := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      "dc2-rebuild",
		},
	}
	setRebuildTaskFinished(ctx, t, f, rebuildTaskKey, dc2Key)

	sg2Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc2-stargate"},
	}

	t.Log("check that stargate sg2 is created")
	require.Eventually(f.StargateExists(ctx, sg2Key), timeout, interval, "failed to verify stargate sg2 created")

	t.Logf("update stargate sg2 status to ready")
	err = f.SetStargateStatusReady(ctx, sg2Key)
	require.NoError(err, "failed to patch stargate status")

	reaper2Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc2-reaper",
		},
	}

	t.Log("check that reaper reaper2 is created")
	require.Eventually(f.ReaperExists(ctx, reaper2Key), timeout, interval, "failed to verify reaper reaper2 created")
}

// schemaDisagreementOnStargate tests the scenario in which updating the replication of
// the Stargate auth keyspace fails. It verifies that the none rebuild task, Stargate, nor
// Reaper objects for DC2 are created.
func schemaDisagreementOnStargate(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3}
	updatedReplication := map[string]int{"dc1": 3, "dc2": 3}

	schemaDisagreementErr := kerrors.NewSchemaDisagreementError("system keyspace check failed")

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, stargate.AuthKeyspace, replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, stargate.AuthKeyspace, updatedReplication).Return(schemaDisagreementErr)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, reaperapi.DefaultKeyspace, replication).Return(nil)
	mockMgmtApi.On(testutils.ListTables, stargate.AuthKeyspace).Return([]string{stargate.AuthTable}, nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return([]string{}, nil)

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	addStargateAndReaperToCluster(ctx, t, f, kc)

	sg1Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[0],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc1-stargate",
		},
	}

	t.Log("check that stargate sg1 is created")
	require.Eventually(f.StargateExists(ctx, sg1Key), timeout, interval)

	t.Logf("update stargate sg1 status to ready")
	err := f.SetStargateStatusReady(ctx, sg1Key)
	require.NoError(err, "failed to patch stargate status")

	reaper1Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[0],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc1-reaper",
		},
	}

	t.Log("check that reaper reaper1 is created")
	require.Eventually(f.ReaperExists(ctx, reaper1Key), timeout, interval)

	t.Logf("update reaper reaper1 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper1Key)
	require.NoError(err, "failed to patch reaper status")

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDcToCluster(ctx, t, f, kc, dc2Key)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval, "failed to verify dc2 was created")

	t.Log("update dc2 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	rebuildTaskKey := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      "dc2-rebuild",
		},
	}
	f.AssertObjectDoesNotExist(ctx, t, rebuildTaskKey, &cassctlapi.CassandraTask{}, timeout, interval)

	sg2Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc2-stargate"},
	}
	f.AssertObjectDoesNotExist(ctx, t, sg2Key, &stargateapi.Stargate{}, timeout, interval)

	reaper2Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: kc.Namespace,
			Name:      kc.Name + "-dc2-reaper",
		},
	}
	f.AssertObjectDoesNotExist(ctx, t, reaper2Key, &reaperapi.Reaper{}, timeout, interval)
}

// failSystemKeyspaceUpdate tests adding a DC to an existing cluster and verifying the
// behavior when updating replication of system keyspaces fails.
func failSystemKeyspaceUpdate(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3}
	updatedReplication := map[string]int{"dc1": 3, "dc2": 3}

	replicationCheckErr := fmt.Errorf("failed to check replication")

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(replicationCheckErr)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(replicationCheckErr)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(replicationCheckErr)

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDcToCluster(ctx, t, f, kc, dc2Key)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval, "failed to verify dc2 was created")

	t.Log("update dc2 status to ready")
	err := f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	verifyRebuildTaskNotCreated(ctx, t, f, kc.Namespace, dc2Key.Name)
}

// failUserKeyspaceUpdate tests adding a DC to an existing cluster and verifying behavior
// when updating replication of user-defined keyspaces fails.
func failUserKeyspaceUpdate(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3}
	updatedReplication := map[string]int{"dc1": 3, "dc2": 3}

	// We need a version of the map with string values because GetKeyspaceReplication returns
	// a map[string]string.
	updatedReplicationStr := map[string]string{"class": cassandra.NetworkTopology, "dc1": "3", "dc2": "3"}

	userKeyspaces := []string{"ks1", "ks2"}

	replicationCheckErr := fmt.Errorf("failed to check replication")

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return(userKeyspaces, nil)

	for _, ks := range userKeyspaces {
		mockMgmtApi.On(testutils.GetKeyspaceReplication, ks).Return(updatedReplicationStr, nil)
		mockMgmtApi.On(testutils.EnsureKeyspaceReplication, ks, updatedReplication).Return(replicationCheckErr)
	}

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	kcKey := utils.GetKey(kc)
	namespace := kcKey.Namespace

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDcToCluster(ctx, t, f, kc, dc2Key)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval, "failed to verify dc2 was created")

	t.Log("update dc2 status to ready")
	err := f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	verifyReplicationOfSystemKeyspacesUpdated(t, mockMgmtApi, replication, updatedReplication)

	verifyRebuildTaskNotCreated(ctx, t, f, namespace, dc2Key.Name)
}

func addStargateAndReaperToCluster(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	t.Log("add Stargate and Reaper")

	key := utils.GetKey(kc)
	err := f.Client.Get(ctx, key, kc)
	require.NoError(t, err, "failed to get K8ssandraCluster")

	kc.Spec.Stargate = &stargateapi.StargateClusterTemplate{
		Size: 1,
	}
	kc.Spec.Reaper = &reaperapi.ReaperClusterTemplate{}

	err = f.Client.Update(ctx, kc)
	require.NoError(t, err, "failed to add Stargate and Reaper")
}

func addDcToCluster(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster, dcKey framework.ClusterKey) {
	t.Logf("add %s to cluster", dcKey.Name)

	key := utils.GetKey(kc)
	err := f.Client.Get(ctx, key, kc)
	require.NoError(t, err, "failed to get K8ssandraCluster")

	kc.Spec.Cassandra.Datacenters = append(kc.Spec.Cassandra.Datacenters, api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name:      dcKey.Name,
			Namespace: dcKey.Namespace,
		},
		K8sContext: dcKey.K8sContext,
		Size:       3,
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "4.0.1",
			StorageConfig: &cassdcapi.StorageConfig{
				CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &defaultStorageClass,
				},
			},
		},
	})
	annotations.AddAnnotation(kc, api.DcReplicationAnnotation, fmt.Sprintf(`{"%s": {"ks1": 3, "ks2": 3}}`, dcKey.Name))

	err = f.Client.Update(ctx, kc)
	require.NoError(t, err, "failed to add dc to K8ssandraCluster")
}

func verifyReplicationOfSystemKeyspacesUpdated(t *testing.T, mockMgmtApi *testutils.FakeManagementApiFacade, replication, updatedReplication map[string]int) {
	require.Eventually(t, func() bool {
		for _, ks := range api.SystemKeyspaces {
			if mockMgmtApi.GetFirstCall(testutils.EnsureKeyspaceReplication, ks, updatedReplication) < 0 {
				return false
			}

		}
		return true
	}, timeout, interval, "Failed to verify system keyspaces replication updated")

	for _, ks := range api.SystemKeyspaces {
		lastCallOriginalReplication := mockMgmtApi.GetLastCall(testutils.EnsureKeyspaceReplication, ks, replication)
		firstCallUpdatedReplication := mockMgmtApi.GetFirstCall(testutils.EnsureKeyspaceReplication, ks, updatedReplication)
		assert.True(t, firstCallUpdatedReplication > lastCallOriginalReplication)
	}
}

func verifyReplicationOfInternalKeyspacesUpdated(t *testing.T, mockMgmtApi *testutils.FakeManagementApiFacade, replication, updatedReplication map[string]int) {
	internalKeyspaces := append(api.SystemKeyspaces, stargate.AuthKeyspace, reaperapi.DefaultKeyspace)

	require.Eventually(t, func() bool {
		for _, ks := range internalKeyspaces {
			if mockMgmtApi.GetFirstCall(testutils.EnsureKeyspaceReplication, ks, updatedReplication) < 0 {
				t.Logf("failed to find updated replication call for keyspace %s with replication %v", ks, updatedReplication)
				return false
			}

		}
		return true
	}, timeout*3, time.Second*1, "Failed to verify internal keyspaces replication updated")

	for _, ks := range internalKeyspaces {
		lastCallOriginalReplication := mockMgmtApi.GetLastCall(testutils.EnsureKeyspaceReplication, ks, replication)
		firstCallUpdatedReplication := mockMgmtApi.GetFirstCall(testutils.EnsureKeyspaceReplication, ks, updatedReplication)
		msg := fmt.Sprintf("replication update check failed for keyspace %s: lastCallOriginal(%d), firstCallUpdated(%d)", ks, lastCallOriginalReplication, firstCallUpdatedReplication)
		assert.True(t, firstCallUpdatedReplication > lastCallOriginalReplication, msg)
	}
}

func verifyReplicationOfKeyspaceUpdated(t *testing.T, mockMgmtApi *testutils.FakeManagementApiFacade, keyspace string, replication map[string]int) {
	require.Eventually(t, func() bool {
		return mockMgmtApi.GetFirstCall(testutils.EnsureKeyspaceReplication, keyspace, replication) > -1
	}, timeout, interval, fmt.Sprintf("failed to verify replication for keyspace %s updated", keyspace))
}

func verifyKeyspaceReplicationAltered(t *testing.T, mockMgmtApi *testutils.FakeManagementApiFacade, keyspace string, replication map[string]int) {
	require.Eventually(t, func() bool {
		return mockMgmtApi.GetFirstCall(testutils.AlterKeyspace, keyspace, replication) > -1
	}, timeout, interval, fmt.Sprintf("failed to verify replication for keyspace %s updated", keyspace))
}

func verifyRebuildTaskCreated(ctx context.Context, t *testing.T, f *framework.Framework, targetDcKey, srcDcKey framework.ClusterKey) {
	t.Log("check that rebuild task was created")
	require := require.New(t)
	task := &cassctlapi.CassandraTask{}
	taskKey := framework.ClusterKey{
		NamespacedName: types.NamespacedName{
			Namespace: targetDcKey.Namespace,
			Name:      targetDcKey.Name + "-rebuild",
		},
		K8sContext: targetDcKey.K8sContext,
	}

	require.Eventually(func() bool {
		err := f.Get(ctx, taskKey, task)
		return err == nil
	}, timeout, interval, "failed to get rebuild task")

	require.Equal(corev1.ObjectReference{Namespace: targetDcKey.Namespace, Name: targetDcKey.Name}, task.Spec.Datacenter)

	require.Contains(task.Labels, rebuildNodesLabel, "rebuild task missing %s label", rebuildNodesLabel)
	nodeCount, err := strconv.Atoi(task.Labels[rebuildNodesLabel])
	require.NoError(err, "invalid value for %s label", rebuildNodesLabel)

	dc := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, targetDcKey, dc)
	require.NoError(err, "failed to get CassandraDatacenter for rebuild")

	require.Equal(int(dc.Spec.Size), nodeCount, "%s labels has wrong value", rebuildNodesLabel)

	expectedJobs := []cassctlapi.CassandraJob{
		{
			Name:      targetDcKey.Name + "-rebuild",
			Command:   "rebuild",
			Arguments: map[string]string{"source_datacenter": srcDcKey.Name},
		},
	}
	require.Equal(expectedJobs, task.Spec.Jobs)
}

func setRebuildTaskFinished(ctx context.Context, t *testing.T, f *framework.Framework, taskKey framework.ClusterKey, dcKey framework.ClusterKey) {
	t.Log("set rebuild task to finished")

	task := &cassctlapi.CassandraTask{}
	err := f.Get(ctx, taskKey, task)
	require.NoError(t, err, "failed to get rebuild task")

	dc := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dcKey, dc)
	require.NoError(t, err, "failed to get CassandraDatacenter for rebuild")

	task.Status.Succeeded = int(dc.Spec.Size)
	err = f.UpdateStatus(ctx, taskKey, task)
	require.NoError(t, err, "failed to set rebuild task finished")
}

func verifyRebuildTaskNotCreated(ctx context.Context, t *testing.T, f *framework.Framework, namespace, dcName string) {
	t.Log("check that rebuild task is not created")

	taskKey := framework.ClusterKey{
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: dcName + "-rebuild"},
		K8sContext:     f.DataPlaneContexts[1],
	}
	require.Never(t, func() bool {
		err := f.Get(ctx, taskKey, &cassctlapi.CassandraTask{})
		return err == nil
	}, timeout, interval, "Failed to verify that the rebuild task was not created")
}

func createCassandraDatacenter(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster, dcIdx int) {
	dcTemplate := kc.Spec.Cassandra.Datacenters[dcIdx]
	dcConfig := cassandra.Coalesce(kc.Name, kc.Spec.Cassandra, &dcTemplate)
	dc, err := cassandra.NewDatacenter(utils.GetKey(kc), dcConfig)

	require.NoError(t, err, "failed to create CassandraDatacenter")

	annotations.AddHashAnnotation(dc)

	namespace := kc.Namespace
	if dcTemplate.Meta.Namespace != "" {
		namespace = dcTemplate.Meta.Namespace
	}

	dcKey := framework.ClusterKey{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      dcTemplate.Meta.Name,
		},
		K8sContext: dcTemplate.K8sContext,
	}
	err = f.Create(ctx, dcKey, dc)
	require.NoError(t, err, "failed to create cassandradatacenter")
}
