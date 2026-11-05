package k8ssandra

import (
	"context"
	"testing"
	"time"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newUnitTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, api.AddToScheme(s))
	require.NoError(t, cassdcapi.AddToScheme(s))
	require.NoError(t, cassctlapi.AddToScheme(s))
	return s
}

// TestReconcileDcRebuildRequeueDelay verifies that rebuild polling uses the configured
// DefaultDelay rather than a 15ns delay, which caused a hot reconcile loop.
func TestReconcileDcRebuildRequeueDelay(t *testing.T) {
	s := newUnitTestScheme(t)
	ctx := context.Background()
	logger := testlogr.NewTestLogger(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}},
					{Meta: api.EmbeddedObjectMeta{Name: "dc2"}},
				},
			},
		},
		Status: api.K8ssandraClusterStatus{
			Datacenters: map[string]api.K8ssandraStatus{
				"dc1": {Cassandra: &cassdcapi.CassandraDatacenterStatus{Conditions: []cassdcapi.DatacenterCondition{
					{Type: cassdcapi.DatacenterReady, Status: "True"},
				}}},
			},
		},
	}
	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc2", Namespace: "ns"},
		Spec:       cassdcapi.CassandraDatacenterSpec{Size: 3},
	}

	remoteClient := fake.NewClientBuilder().WithScheme(s).Build()
	r := &K8ssandraClusterReconciler{
		ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: 7 * time.Second},
		Client:           remoteClient,
		Scheme:           s,
	}

	// First pass creates the task
	res, err := r.reconcileDcRebuild(ctx, kc, dc, remoteClient, logger).Output()
	require.NoError(t, err)
	require.Equal(t, 7*time.Second, res.RequeueAfter)

	task := &cassctlapi.CassandraTask{}
	require.NoError(t, remoteClient.Get(ctx, client.ObjectKey{Namespace: "ns", Name: "dc2-rebuild"}, task))

	// Second pass waits for the unfinished task
	res, err = r.reconcileDcRebuild(ctx, kc, dc, remoteClient, logger).Output()
	require.NoError(t, err)
	require.Equal(t, 7*time.Second, res.RequeueAfter)
}
