package k8ssandra

import (
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	ktaskapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

// TestDatacentersByTemplate verifies that the reconciled datacenters, which are sorted by upgrade priority,
// are paired with their templates by name and not by index.
func TestDatacentersByTemplate(t *testing.T) {
	kc := &api.K8ssandraCluster{
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{Meta: api.EmbeddedObjectMeta{Name: "cassandra"}},
					{Meta: api.EmbeddedObjectMeta{Name: "analytics"}},
				},
			},
		},
	}
	// actualDcs are in priority order: analytics first
	dcs := []*cassdcapi.CassandraDatacenter{
		{ObjectMeta: metav1.ObjectMeta{Name: "analytics"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cassandra"}},
	}

	ordered, err := datacentersByTemplate(kc, dcs)
	require.NoError(t, err)
	require.Len(t, ordered, 2)
	require.Equal(t, "cassandra", ordered[0].Name)
	require.Equal(t, "analytics", ordered[1].Name)

	_, err = datacentersByTemplate(kc, dcs[:1])
	require.Error(t, err)
}

// TestMidPassWritesPreserveInMemoryStatus verifies that writes to the main K8ssandraCluster resource during a
// reconcile do not replace the in-memory status with the stored one.
func TestMidPassWritesPreserveInMemoryStatus(t *testing.T) {
	s := newUnitTestScheme(t)
	ctx := t.Context()

	newKc := func() *api.K8ssandraCluster {
		return &api.K8ssandraCluster{
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
				Datacenters: map[string]api.K8ssandraStatus{"dc1": {ContextName: "old"}},
			},
		}
	}

	setup := func(t *testing.T, kc *api.K8ssandraCluster) (*K8ssandraClusterReconciler, *api.K8ssandraCluster) {
		c := fake.NewClientBuilder().WithScheme(s).WithObjects(kc).WithStatusSubresource(&api.K8ssandraCluster{}).Build()
		r := &K8ssandraClusterReconciler{
			ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second},
			Client:           c,
			Scheme:           s,
		}
		actual := &api.K8ssandraCluster{}
		require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(kc), actual))
		// This reconcile's in-memory status change
		actual.Status.Datacenters["dc1"] = api.K8ssandraStatus{ContextName: "fresh"}
		return r, actual
	}

	t.Run("checkRebuildAnnotation", func(t *testing.T) {
		r, kc := setup(t, newKc())
		res := r.checkRebuildAnnotation(ctx, kc, "dc2")
		require.False(t, res.IsError())
		require.Equal(t, "dc2", kc.Annotations[api.RebuildDcAnnotation])
		require.Equal(t, "fresh", kc.Status.Datacenters["dc1"].ContextName)
	})

	t.Run("removeRebuildDcAnnotation", func(t *testing.T) {
		kc := newKc()
		kc.Annotations = map[string]string{api.RebuildDcAnnotation: "dc2"}
		r, kc := setup(t, kc)
		res := r.removeRebuildDcAnnotation(kc, ctx)
		require.False(t, res.IsError())
		require.NotContains(t, kc.Annotations, api.RebuildDcAnnotation)
		require.Equal(t, "fresh", kc.Status.Datacenters["dc1"].ContextName)
	})

	t.Run("updateStatus", func(t *testing.T) {
		kc := newKc()
		kc.Annotations = map[string]string{api.AutomatedUpdateAnnotation: string(api.AllowUpdateOnce)}
		r, kc := setup(t, kc)
		res := updateStatus(ctx, r.Client, kc)
		require.False(t, res.IsError())

		stored := &api.K8ssandraCluster{}
		require.NoError(t, r.Get(ctx, client.ObjectKeyFromObject(kc), stored))
		require.NotContains(t, stored.Annotations, api.AutomatedUpdateAnnotation)
		require.Equal(t, "fresh", stored.Status.Datacenters["dc1"].ContextName)
	})
}

// TestDatacenterForUpdatePreservesForeignMetadata verifies that updating a CassandraDatacenter keeps finalizers and
// annotations set by others, while annotations that k8ssandra-operator set and no longer wants are removed.
func TestDatacenterForUpdatePreservesForeignMetadata(t *testing.T) {
	desired := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "dc1",
			Finalizers:  []string{"k8ssandracluster.k8ssandra.io/finalizer"},
			Annotations: map[string]string{"k8ssandra.io/resource-hash": "new", "user-template": "v2"},
		},
	}

	t.Run("tracked", func(t *testing.T) {
		actual := &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "dc1",
				ResourceVersion: "42",
				Finalizers:      []string{"finalizer.cassandra.datastax.com", "k8ssandracluster.k8ssandra.io/finalizer"},
				Annotations: map[string]string{
					"k8ssandra.io/resource-hash": "old",
					"user-template":              "v1",
					"removed-from-template":      "x",

					"cassandra.datastax.com/skip-local-decommission-check": "true",
					ownedAnnotationsAnnotation:                             "k8ssandra.io/resource-hash,removed-from-template,user-template",
				},
			},
		}

		updated := datacenterForUpdate(actual, desired)
		require.Equal(t, "42", updated.ResourceVersion)
		require.ElementsMatch(t, []string{"finalizer.cassandra.datastax.com", "k8ssandracluster.k8ssandra.io/finalizer"}, updated.Finalizers)
		require.Equal(t, map[string]string{
			"k8ssandra.io/resource-hash": "new",
			"user-template":              "v2",
			"cassandra.datastax.com/skip-local-decommission-check": "true",
			ownedAnnotationsAnnotation:                             "k8ssandra.io/resource-hash,user-template",
		}, updated.Annotations)
		// desired must not be modified
		require.NotContains(t, desired.Annotations, ownedAnnotationsAnnotation)
	})

	t.Run("not tracked yet", func(t *testing.T) {
		actual := &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "dc1",
				Finalizers:  []string{"finalizer.cassandra.datastax.com"},
				Annotations: map[string]string{"k8ssandra.io/resource-hash": "old", "unknown-origin": "x"},
			},
		}

		updated := datacenterForUpdate(actual, desired)
		require.ElementsMatch(t, []string{"finalizer.cassandra.datastax.com", "k8ssandracluster.k8ssandra.io/finalizer"}, updated.Finalizers)
		require.NotContains(t, updated.Annotations, "unknown-origin")
		require.Equal(t, "k8ssandra.io/resource-hash,user-template", updated.Annotations[ownedAnnotationsAnnotation])
	})
}

// TestRefreshTaskRunning verifies that an unfinished refresh task is found even if its name was generated from
// older generations.
func TestRefreshTaskRunning(t *testing.T) {
	s := newUnitTestScheme(t)
	require.NoError(t, ktaskapi.AddToScheme(s))
	ctx := t.Context()

	kc := &api.K8ssandraCluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns", Generation: 3}}
	task := &ktaskapi.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{Name: "refresh-2-1", Namespace: "ns", Labels: labels.WatchedByK8ssandraClusterLabels(client.ObjectKeyFromObject(kc))},
		Spec:       ktaskapi.K8ssandraTaskSpec{Cluster: corev1.ObjectReference{Name: kc.Name}},
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(task).WithStatusSubresource(task).Build()
	r := &K8ssandraClusterReconciler{Client: c, Scheme: s}

	running, err := r.refreshTaskRunning(ctx, kc)
	require.NoError(t, err)
	require.Equal(t, task.Name, running)

	// An invalid task never completes, so it must not block later refreshes.
	task.SetCondition(ktaskapi.JobInvalid, metav1.ConditionTrue)
	require.NoError(t, c.Status().Update(ctx, task))
	running, err = r.refreshTaskRunning(ctx, kc)
	require.NoError(t, err)
	require.Empty(t, running)

	task.SetCondition(ktaskapi.JobInvalid, metav1.ConditionFalse)
	task.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	require.NoError(t, c.Status().Update(ctx, task))
	running, err = r.refreshTaskRunning(ctx, kc)
	require.NoError(t, err)
	require.Empty(t, running)
}
