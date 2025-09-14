package k8ssandra

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	cassdcFinalizer = "finalizer.cassandra.datastax.com"
)

func deleteDcWithUserKeyspacesFails(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3, "dc2": 3}
	updatedReplication := map[string]int{"dc1": 3}

	// We need a version of the map with string values because GetKeyspaceReplication returns
	// a map[string]string.
	replicationStr := map[string]string{"class": cassandra.NetworkTopology, "dc1": "3", "dc2": "3"}

	userKeyspaces := []string{"ks1", "ks2"}

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return(userKeyspaces, nil)
	mockMgmtApi.On(testutils.GetSchemaVersions).Return(map[string][]string{"fake": {"test"}}, nil)

	for _, ks := range userKeyspaces {
		mockMgmtApi.On(testutils.GetKeyspaceReplication, ks).Return(replicationStr, nil)
	}

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDatacenterFinalizer(ctx, t, f, dc2Key)

	kcKey := utils.GetKey(kc)

	require.Eventually(func() bool {
		err := f.Client.Get(ctx, kcKey, kc)
		if err != nil {
			return false
		}
		t.Log("remove dc2 from k8ssandraCluster spec")
		kc.Spec.Cassandra.Datacenters = kc.Spec.Cassandra.Datacenters[:1]
		return f.Client.Update(ctx, kc) == nil
	}, timeout, interval, "failed to remove dc2 from K8ssandraCluster spec")

	t.Log("verify that dc2 removal generates an error")
	require.Eventually(func() bool {
		err := f.Client.Get(ctx, kcKey, kc)
		if err != nil {
			return false
		}
		return strings.Contains(kc.Status.Error, "cannot decommission DC dc2")
	}, timeout, interval, "expected error on dc2 removal not found")

}

func deleteDcWithUserKeyspacesSucceeds(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	require := require.New(t)

	replication := map[string]int{"dc1": 3, "dc2": 3}
	updatedReplication := map[string]int{"dc1": 3}

	// We need a version of the map with string values because GetKeyspaceReplication returns
	// a map[string]string.
	replicationStr := map[string]string{"class": cassandra.NetworkTopology, "dc1": "3"}

	userKeyspaces := []string{"ks1", "ks2"}

	mockMgmtApi := testutils.NewFakeManagementApiFacade()
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_auth", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_distributed", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", replication).Return(nil)
	mockMgmtApi.On(testutils.EnsureKeyspaceReplication, "system_traces", updatedReplication).Return(nil)
	mockMgmtApi.On(testutils.ListKeyspaces, "").Return(userKeyspaces, nil)
	mockMgmtApi.On(testutils.GetSchemaVersions).Return(map[string][]string{"fake": {"test"}}, nil)

	for _, ks := range userKeyspaces {
		mockMgmtApi.On(testutils.GetKeyspaceReplication, ks).Return(replicationStr, nil)
		mockMgmtApi.On(testutils.AlterKeyspace, ks, updatedReplication).Return(nil)
	}

	adapter := func(ctx context.Context, datacenter *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) (cassandra.ManagementApiFacade, error) {
		return mockMgmtApi, nil
	}
	managementApiFactory.SetAdapter(adapter)

	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: kc.Namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}

	addDatacenterFinalizer(ctx, t, f, dc2Key)

	kcKey := utils.GetKey(kc)

	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster")

	t.Log("remove dc2 from k8ssandraCluster spec")
	kc.Spec.Cassandra.Datacenters = kc.Spec.Cassandra.Datacenters[:1]
	err = f.Client.Update(ctx, kc)
	require.NoError(err, "failed to remove dc2 from K8ssandraCluster spec")

	assertDecommissionAnnotationAdded(ctx, t, f, dc2Key)

	t.Log("verify that dc2 removal generates no error")
	require.Eventually(func() bool {
		err := f.Client.Get(ctx, kcKey, kc)
		if err != nil {
			return false
		}
		return kc.Status.Error == "None"
	}, timeout, interval, "unexpected error on dc2 removal found")

	// Make sure the status isn't updated too soon
	assertDatacenterInClusterStatus(ctx, t, f, kcKey, dc2Key)

	finishDatacenterDecommission(ctx, t, f, dc2Key)

	f.AssertObjectDoesNotExist(ctx, t, dc2Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)

	assertDatacenterRemovedFromClusterStatus(ctx, t, f, kcKey, dc2Key)
}

func assertDecommissionAnnotationAdded(ctx context.Context, t *testing.T, f *framework.Framework, dcKey framework.ClusterKey) {
	t.Logf("check that decommission annotation added to dc (%s)", dcKey.Name)
	assert.Eventually(t, func() bool {
		dc := &cassdcapi.CassandraDatacenter{}
		err := f.Get(ctx, dcKey, dc)
		if err != nil {
			return false
		}
		return annotations.HasAnnotationWithValue(dc, cassdcapi.DecommissionOnDeleteAnnotation, "true")
	}, timeout, interval, "timed out waiting for decommission annotation to be added to dc (%s)", dcKey.Name)
}

func assertDatacenterInClusterStatus(ctx context.Context, t *testing.T, f *framework.Framework, kcKey client.ObjectKey, dcKey framework.ClusterKey) {
	t.Logf("check that %s is in the K8ssandraCluster status", dcKey.Name)
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(t, err, "failed to get K8ssandraCluster")
	_, found := kc.Status.Datacenters[dcKey.Name]
	assert.True(t, found, "expected to find dc (%s) in K8ssandraCluster status", dcKey.Name)
}

func assertDatacenterRemovedFromClusterStatus(ctx context.Context, t *testing.T, f *framework.Framework, kcKey client.ObjectKey, dcKey framework.ClusterKey) {
	t.Logf("check that %s is remove from K8ssandraCluster status", dcKey.Name)
	assert.Eventually(t, func() bool {
		kc := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, kcKey, kc)
		if err != nil {
			return false
		}
		_, found := kc.Status.Datacenters[dcKey.Name]
		return !found
	}, timeout, interval, "timed out waiting for dc (%s) to be removed from K8ssandraCluster status", dcKey.Name)
}

func addDatacenterFinalizer(ctx context.Context, t *testing.T, f *framework.Framework, dcKey framework.ClusterKey) {
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(t, err, "failed to get dc (%s)", dcKey.Name)

	controllerutil.AddFinalizer(dc, cassdcFinalizer)
	err = f.Update(ctx, dcKey, dc)
	require.NoError(t, err, "failed to add finalizer to dc (%s)")
}

func finishDatacenterDecommission(ctx context.Context, t *testing.T, f *framework.Framework, dcKey framework.ClusterKey) {
	t.Logf("simulate cass-operator completing the decommission and deletion of dc2")
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(t, err, "failed to get dc (%s)", dcKey.Name)
	controllerutil.RemoveFinalizer(dc, cassdcFinalizer)
	err = f.Update(ctx, dcKey, dc)
	require.NoError(t, err, "failed to remove finalizer from dc (%s)", dcKey.Name)
}
