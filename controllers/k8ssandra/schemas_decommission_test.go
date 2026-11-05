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
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCheckSchemasDecommission covers the decommission branch of checkSchemas when the DC being
// removed and the DC currently being reconciled live in different k8s clusters, and when the
// CassandraDatacenter object of the removed DC no longer exists.
func TestCheckSchemasDecommission(t *testing.T) {
	require.NoError(t, api.AddToScheme(scheme.Scheme))
	require.NoError(t, cassdcapi.AddToScheme(scheme.Scheme))

	t.Run("LocalDcRemovedWhileLoopDcIsRemote", func(t *testing.T) {
		// The removed dc2 has a datacenterName override, the replication check must use it.
		dc2 := removedDc("real-dc2")

		t.Run("NoReplicasLeft", func(t *testing.T) {
			f := newDecommissionFixture(t, nil, dc2)
			res := f.checkSchemas(t, map[string]string{"dc1": "3"})

			assertRequeued(t, res)
			assert.Equal(t, api.DecommDeleting, f.kc.Status.Datacenters["dc2"].DecommissionProgress)
		})

		t.Run("KeyspaceStillReplicatesToRemovedDc", func(t *testing.T) {
			f := newDecommissionFixture(t, nil, dc2)
			res := f.checkSchemas(t, map[string]string{"dc1": "3", "real-dc2": "3"})

			assertKeyspaceStillReplicates(t, res, "real-dc2")
			assert.Equal(t, api.DecommUpdatingReplication, f.kc.Status.Datacenters["dc2"].DecommissionProgress)
		})
	})

	t.Run("RemovedDcObjectAlreadyGone", func(t *testing.T) {
		t.Run("NameFromStatus/NoReplicasLeft", func(t *testing.T) {
			f := newDecommissionFixture(t, ptr.To("real-dc2"), nil)
			res := f.checkSchemas(t, map[string]string{"dc1": "3", "dc2": "3"})

			assertRequeued(t, res)
			assert.Equal(t, api.DecommDeleting, f.kc.Status.Datacenters["dc2"].DecommissionProgress)
		})

		t.Run("NameFromStatus/KeyspaceStillReplicatesToRemovedDc", func(t *testing.T) {
			f := newDecommissionFixture(t, ptr.To("real-dc2"), nil)
			res := f.checkSchemas(t, map[string]string{"dc1": "3", "real-dc2": "3"})

			assertKeyspaceStillReplicates(t, res, "real-dc2")
			assert.Equal(t, api.DecommUpdatingReplication, f.kc.Status.Datacenters["dc2"].DecommissionProgress)
		})

		t.Run("NoNameInStatus/KeyspaceStillReplicatesToRemovedDc", func(t *testing.T) {
			f := newDecommissionFixture(t, nil, nil)
			res := f.checkSchemas(t, map[string]string{"dc1": "3", "dc2": "3"})

			assertKeyspaceStillReplicates(t, res, "dc2")
			assert.Equal(t, api.DecommUpdatingReplication, f.kc.Status.Datacenters["dc2"].DecommissionProgress)
		})
	})
}

type decommissionFixture struct {
	kc          *api.K8ssandraCluster
	loopDc      *cassdcapi.CassandraDatacenter
	localClient client.Client
	eastClient  client.Client
	mgmtApi     *testutils.FakeManagementApiFactory
	reconciler  *K8ssandraClusterReconciler
}

// newDecommissionFixture builds a cluster where dc1 lives in the remote "east" context and dc2,
// which is being removed, lived in the control-plane cluster (ContextName ""). If removedDc is
// non-nil, it is created in the local cluster.
func newDecommissionFixture(t *testing.T, statusDcName *string, removedDc *cassdcapi.CassandraDatacenter) *decommissionFixture {
	ctx := context.Background()

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "kc", Namespace: "ns"},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				ServerType: api.ServerDistributionCassandra,
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "4.1.0",
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1", Namespace: "ns"}, K8sContext: "east"},
				},
			},
		},
		Status: api.K8ssandraClusterStatus{
			Datacenters: map[string]api.K8ssandraStatus{
				"dc1": {ContextName: "east"},
				"dc2": {
					ContextName:          "",
					DecommissionProgress: api.DecommUpdatingReplication,
					Cassandra:            &cassdcapi.CassandraDatacenterStatus{DatacenterName: statusDcName},
				},
			},
		},
	}

	localBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(kc.DeepCopy())
	if removedDc != nil {
		localBuilder = localBuilder.WithObjects(removedDc)
	}
	localClient := localBuilder.Build()
	eastClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

	// Read the kc back so that it carries the resourceVersion that versionCheck needs.
	stored := &api.K8ssandraCluster{}
	require.NoError(t, localClient.Get(ctx, client.ObjectKeyFromObject(kc), stored))
	stored.Status = kc.Status

	cc := clientcache.New(localClient, localClient, scheme.Scheme)
	cc.AddClient("east", eastClient)

	factory := &testutils.FakeManagementApiFactory{}
	factory.SetT(t)

	return &decommissionFixture{
		kc: stored,
		loopDc: &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "ns"},
		},
		localClient: localClient,
		eastClient:  eastClient,
		mgmtApi:     factory,
		reconciler: &K8ssandraClusterReconciler{
			Client:        localClient,
			Scheme:        scheme.Scheme,
			ClientCache:   cc,
			ManagementApi: factory,
			ReconcilerConfig: &config.ReconcilerConfig{
				DefaultDelay: time.Second,
				LongDelay:    time.Minute,
			},
		},
	}
}

// checkSchemas runs checkSchemas for the remote dc1 (with the east client), with a single user
// keyspace "ks1" that has the given replication.
func (f *decommissionFixture) checkSchemas(t *testing.T, ksReplication map[string]string) (res reconcileResultView) {
	f.mgmtApi.SetAdapter(func(_ context.Context, _ *cassdcapi.CassandraDatacenter, _ client.Client, _ logr.Logger) (cassandra.ManagementApiFacade, error) {
		m := new(mocks.ManagementApiFacade)
		m.On(testutils.EnsureKeyspaceReplication, mock.Anything, mock.Anything).Return(nil)
		m.On(testutils.ListKeyspaces, "").Return([]string{"system_auth", "ks1"}, nil)
		replication := map[string]string{"class": "org.apache.cassandra.locator.NetworkTopologyStrategy"}
		for k, v := range ksReplication {
			replication[k] = v
		}
		m.On(testutils.GetKeyspaceReplication, "ks1").Return(replication, nil)
		return m, nil
	})

	require.NotPanics(t, func() {
		r := f.reconciler.checkSchemas(context.Background(), f.kc, f.loopDc, f.eastClient, testlogr.NewTestLogger(t))
		res = reconcileResultView{completed: r.Completed()}
		res.result, res.err = r.Output()
	})
	return res
}

type reconcileResultView struct {
	completed bool
	result    ctrl.Result
	err       error
}

func assertRequeued(t *testing.T, res reconcileResultView) {
	t.Helper()
	require.True(t, res.completed)
	require.NoError(t, res.err)
	assert.Equal(t, time.Second, res.result.RequeueAfter)
}

func assertKeyspaceStillReplicates(t *testing.T, res reconcileResultView, cassDcName string) {
	t.Helper()
	require.True(t, res.completed)
	require.Error(t, res.err)
	assert.Contains(t, res.err.Error(), "cannot decommission DC "+cassDcName+": keyspace ks1 still has replicas on it")
}

func removedDc(datacenterName string) *cassdcapi.CassandraDatacenter {
	return &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc2",
			Namespace: "ns",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: "ns", Name: "kc"}),
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:    "kc",
			ServerType:     "cassandra",
			ServerVersion:  "4.1.0",
			DatacenterName: datacenterName,
		},
	}
}
