package secret

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaApi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newMedusaTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(api.AddToScheme(s))
	utilruntime.Must(replicationapi.AddToScheme(s))
	return s
}

// TestReconcileMedusaReplicatedSecret_Create verifies that a new ReplicatedSecret is created
// when none exists, with the correct name, namespace, replication targets, and label selector.
func TestReconcileMedusaReplicatedSecret_Create(t *testing.T) {
	ctx := context.Background()
	scheme := newMedusaTestScheme()
	ns := "test-ns"
	kcName := "mycluster"
	secretRefName := "medusa-secret"

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kcName,
			Namespace: ns,
			UID:       "some-uid",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta:       api.EmbeddedObjectMeta{Name: "dc1"},
						K8sContext: "ctx1",
					},
				},
			},
			Medusa: &medusaApi.MedusaClusterTemplate{
				StorageProperties: medusaApi.Storage{
					StorageSecretRef: corev1.LocalObjectReference{Name: secretRefName},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := ReconcileMedusaReplicatedSecret(ctx, fakeClient, scheme, kc, logr.Discard())
	require.NoError(t, err)

	// Verify the ReplicatedSecret was created with the right name.
	created := &replicationapi.ReplicatedSecret{}
	err = fakeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: kcName + "-medusa-storage-credentials"}, created)
	require.NoError(t, err)

	// Should have two replication targets: one for dc1, one for the control-plane.
	assert.Len(t, created.Spec.ReplicationTargets, 2)

	// First target is for the DC.
	assert.Equal(t, "ctx1", created.Spec.ReplicationTargets[0].K8sContextName)
	assert.Equal(t, ns, created.Spec.ReplicationTargets[0].Namespace)
	assert.Equal(t, kc.SanitizedName()+"-", created.Spec.ReplicationTargets[0].TargetPrefix)
	assert.Equal(t, []string{medusaApi.MedusaStorageSecretIdentifierLabel}, created.Spec.ReplicationTargets[0].DropLabels)

	// Second target is the control-plane (empty context, same namespace).
	assert.Equal(t, "", created.Spec.ReplicationTargets[1].K8sContextName)
	assert.Equal(t, ns, created.Spec.ReplicationTargets[1].Namespace)
	assert.Equal(t, kc.SanitizedName()+"-", created.Spec.ReplicationTargets[1].TargetPrefix)
	assert.Equal(t, []string{medusaApi.MedusaStorageSecretIdentifierLabel}, created.Spec.ReplicationTargets[1].DropLabels)

	// Label selector must use the storage secret hash.
	expectedHash := utils.HashNameNamespace(secretRefName, ns)
	require.NotNil(t, created.Spec.Selector)
	assert.Equal(t, expectedHash, created.Spec.Selector.MatchLabels[medusaApi.MedusaStorageSecretIdentifierLabel])
}

// TestReconcileMedusaReplicatedSecret_MultiDC verifies that all DCs appear as replication
// targets, using the dc-level namespace when set and falling back to the kc namespace.
func TestReconcileMedusaReplicatedSecret_MultiDC(t *testing.T) {
	ctx := context.Background()
	scheme := newMedusaTestScheme()
	ns := "test-ns"
	kcName := "multidc"

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kcName,
			Namespace: ns,
			UID:       "some-uid",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta:       api.EmbeddedObjectMeta{Name: "dc1", Namespace: "dc1-ns"},
						K8sContext: "ctx1",
					},
					{
						// No namespace override — should fall back to kc.Namespace.
						Meta:       api.EmbeddedObjectMeta{Name: "dc2"},
						K8sContext: "ctx2",
					},
				},
			},
			Medusa: &medusaApi.MedusaClusterTemplate{
				StorageProperties: medusaApi.Storage{
					StorageSecretRef: corev1.LocalObjectReference{Name: "some-secret"},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := ReconcileMedusaReplicatedSecret(ctx, fakeClient, scheme, kc, logr.Discard())
	require.NoError(t, err)

	created := &replicationapi.ReplicatedSecret{}
	err = fakeClient.Get(ctx, types.NamespacedName{Namespace: ns, Name: kcName + "-medusa-storage-credentials"}, created)
	require.NoError(t, err)

	// 2 DCs + 1 control-plane = 3 targets total.
	assert.Len(t, created.Spec.ReplicationTargets, 3)

	// DC1 uses its own namespace override.
	assert.Equal(t, "ctx1", created.Spec.ReplicationTargets[0].K8sContextName)
	assert.Equal(t, "dc1-ns", created.Spec.ReplicationTargets[0].Namespace)

	// DC2 falls back to kc namespace.
	assert.Equal(t, "ctx2", created.Spec.ReplicationTargets[1].K8sContextName)
	assert.Equal(t, ns, created.Spec.ReplicationTargets[1].Namespace)

	// Control-plane entry is last.
	assert.Equal(t, "", created.Spec.ReplicationTargets[2].K8sContextName)
	assert.Equal(t, ns, created.Spec.ReplicationTargets[2].Namespace)
}

// TestReconcileMedusaReplicatedSecret_Update verifies that when the ReplicatedSecret already
// exists with a stale selector it is updated, and that its finalizers are preserved.
func TestReconcileMedusaReplicatedSecret_Update(t *testing.T) {
	ctx := context.Background()
	scheme := newMedusaTestScheme()
	ns := "test-ns"
	kcName := "updatecluster"

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kcName,
			Namespace: ns,
			UID:       "some-uid",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta:       api.EmbeddedObjectMeta{Name: "dc1"},
						K8sContext: "ctx1",
					},
				},
			},
			Medusa: &medusaApi.MedusaClusterTemplate{
				StorageProperties: medusaApi.Storage{
					StorageSecretRef: corev1.LocalObjectReference{Name: "medusa-secret"},
				},
			},
		},
	}

	// Pre-create a ReplicatedSecret with a stale selector and a finalizer that must survive.
	existing := &replicationapi.ReplicatedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       kcName + "-medusa-storage-credentials",
			Namespace:  ns,
			Finalizers: []string{"some-finalizer"},
		},
		Spec: replicationapi.ReplicatedSecretSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"stale": "label"},
			},
			ReplicationTargets: []replicationapi.ReplicationTarget{},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()

	err := ReconcileMedusaReplicatedSecret(ctx, fakeClient, scheme, kc, logr.Discard())
	require.NoError(t, err)

	updated := &replicationapi.ReplicatedSecret{}
	err = fakeClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: kcName + "-medusa-storage-credentials"}, updated)
	require.NoError(t, err)

	// Targets must now reflect the desired state (DC + control-plane).
	assert.Len(t, updated.Spec.ReplicationTargets, 2)

	// Finalizers must be preserved across the update.
	assert.Equal(t, []string{"some-finalizer"}, updated.Finalizers)
}

// TestReconcileMedusaReplicatedSecret_NoOpWhenUpToDate verifies that when the
// ReplicatedSecret already matches the desired state, no update is issued.
func TestReconcileMedusaReplicatedSecret_NoOpWhenUpToDate(t *testing.T) {
	ctx := context.Background()
	scheme := newMedusaTestScheme()
	ns := "test-ns"
	kcName := "noop-cluster"
	secretRefName := "medusa-secret"

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kcName,
			Namespace: ns,
			UID:       "some-uid",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta:       api.EmbeddedObjectMeta{Name: "dc1"},
						K8sContext: "ctx1",
					},
				},
			},
			Medusa: &medusaApi.MedusaClusterTemplate{
				StorageProperties: medusaApi.Storage{
					StorageSecretRef: corev1.LocalObjectReference{Name: secretRefName},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// First call creates the ReplicatedSecret.
	err := ReconcileMedusaReplicatedSecret(ctx, fakeClient, scheme, kc, logr.Discard())
	require.NoError(t, err)

	first := &replicationapi.ReplicatedSecret{}
	err = fakeClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: kcName + "-medusa-storage-credentials"}, first)
	require.NoError(t, err)
	rvAfterCreate := first.ResourceVersion

	// Second call with identical state — should be a no-op.
	err = ReconcileMedusaReplicatedSecret(ctx, fakeClient, scheme, kc, logr.Discard())
	require.NoError(t, err)

	second := &replicationapi.ReplicatedSecret{}
	err = fakeClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: kcName + "-medusa-storage-credentials"}, second)
	require.NoError(t, err)

	// ResourceVersion must be unchanged — no update was issued.
	assert.Equal(t, rvAfterCreate, second.ResourceVersion)
}

// TestGenerateMedusaReplicatedSecret verifies the shape of the generated object.
func TestGenerateMedusaReplicatedSecret(t *testing.T) {
	kcKey := client.ObjectKey{Namespace: "ns", Name: "kc"}
	targets := []replicationapi.ReplicationTarget{
		{K8sContextName: "ctx1", Namespace: "ns"},
	}
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"key": "val"},
	}

	repSec := generateMedusaReplicatedSecret(kcKey, targets, selector)

	assert.Equal(t, "kc-medusa-storage-credentials", repSec.Name)
	assert.Equal(t, "ns", repSec.Namespace)
	assert.Equal(t, selector, repSec.Spec.Selector)
	assert.Equal(t, targets, repSec.Spec.ReplicationTargets)
}
