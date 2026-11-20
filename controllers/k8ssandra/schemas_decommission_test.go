package k8ssandra

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCheckSchemasDecommissionDcWithOverrideAlreadyGone covers removing a DC that had a
// datacenterName override after its CassandraDatacenter was deleted. The DC topologies and the
// "CassandraDatacenter already gone" flow are covered by the envtests in remove_dc_test.go. This case
// stays a unit test because the envtest multi-DC setup has no DC with a datacenterName override.
//
// cass-operator records the object name (dc2), not the override (real-dc2), in
// status.datacenterName, so the override can't be recovered once the object is gone.
func TestCheckSchemasDecommissionDcWithOverrideAlreadyGone(t *testing.T) {
	require.NoError(t, api.AddToScheme(scheme.Scheme))
	require.NoError(t, cassdcapi.AddToScheme(scheme.Scheme))

	t.Run("KeyspaceStillReplicatesToOverrideName", func(t *testing.T) {
		kc, r := newDecommissionReconciler(t, nil)
		res := checkSchemasWithReplication(t, r, kc, map[string]string{"dc1": "3", "real-dc2": "3"})

		require.True(t, res.Completed())
		_, err := res.Output()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot decommission DC dc2: keyspace ks1 still has replicas on DC real-dc2, which is not part of the cluster")
		assert.Equal(t, api.DecommUpdatingReplication, kc.Status.Datacenters["dc2"].DecommissionProgress)
	})

	t.Run("OnlyKnownDcsLeft", func(t *testing.T) {
		kc, r := newDecommissionReconciler(t, []string{"external-dc"})
		res := checkSchemasWithReplication(t, r, kc, map[string]string{"dc1": "3", "external-dc": "3"})

		require.True(t, res.Completed())
		out, err := res.Output()
		require.NoError(t, err)
		assert.Equal(t, time.Second, out.RequeueAfter)
		assert.Equal(t, api.DecommDeleting, kc.Status.Datacenters["dc2"].DecommissionProgress)
	})
}

// newDecommissionReconciler returns a cluster where dc2 is being removed and its
// CassandraDatacenter no longer exists.
func newDecommissionReconciler(t *testing.T, externalDcs []string) (*api.K8ssandraCluster, *K8ssandraClusterReconciler) {
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "kc", Namespace: "ns"},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				ServerType: api.ServerDistributionCassandra,
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "4.1.0",
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1", Namespace: "ns"}},
				},
			},
			ExternalDatacenters: externalDcs,
		},
		Status: api.K8ssandraClusterStatus{
			Datacenters: map[string]api.K8ssandraStatus{
				"dc1": {},
				"dc2": {
					DecommissionProgress: api.DecommUpdatingReplication,
					Cassandra:            &cassdcapi.CassandraDatacenterStatus{DatacenterName: ptr.To("dc2")},
				},
			},
		},
	}

	localClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(kc.DeepCopy()).Build()

	// Read the kc back so that it carries the resourceVersion that versionCheck needs.
	stored := &api.K8ssandraCluster{}
	require.NoError(t, localClient.Get(t.Context(), client.ObjectKeyFromObject(kc), stored))
	stored.Status = kc.Status

	factory := &testutils.FakeManagementApiFactory{}
	factory.SetT(t)

	return stored, &K8ssandraClusterReconciler{
		ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second, LongDelay: time.Minute},
		Client:           localClient,
		Scheme:           scheme.Scheme,
		ClientCache:      clientcache.New(localClient, localClient, scheme.Scheme),
		ManagementApi:    factory,
	}
}

// checkSchemasWithReplication runs checkSchemas for dc1 with a single user keyspace "ks1" that has
// the given replication.
func checkSchemasWithReplication(t *testing.T, r *K8ssandraClusterReconciler, kc *api.K8ssandraCluster, ksReplication map[string]string) result.ReconcileResult {
	r.ManagementApi.(*testutils.FakeManagementApiFactory).SetAdapter(func(context.Context, *cassdcapi.CassandraDatacenter, client.Client, logr.Logger) (cassandra.ManagementApiFacade, error) {
		m := new(mocks.ManagementApiFacade)
		m.On(testutils.EnsureKeyspaceReplication, mock.Anything, mock.Anything).Return(nil)
		m.On(testutils.ListKeyspaces, "").Return([]string{"system_auth", "ks1"}, nil)
		replication := map[string]string{"class": cassandra.NetworkTopology}
		for k, v := range ksReplication {
			replication[k] = v
		}
		m.On(testutils.GetKeyspaceReplication, "ks1").Return(replication, nil)
		return m, nil
	})

	loopDc := &cassdcapi.CassandraDatacenter{ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "ns"}}
	var res result.ReconcileResult
	require.NotPanics(t, func() {
		res = r.checkSchemas(t.Context(), kc, loopDc, r.Client, testlogr.NewTestLogger(t))
	})
	return res
}
