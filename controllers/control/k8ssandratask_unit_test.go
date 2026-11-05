package control

import (
	"context"
	"testing"
	"time"

	cassapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	k8capi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestMatchDcsKeepsListedOrder(t *testing.T) {
	kc := newCluster("ns", newDc("dc1", ""), newDc("dc2", ""), newDc("dc3", ""))

	dcs, unknown := matchDcs(kc, []string{"dc3", "dc1", "dc4"})
	require.Equal(t, []string{"dc4"}, unknown)
	require.Len(t, dcs, 2)
	require.Equal(t, "dc3", dcs[0].Meta.Name)
	require.Equal(t, "dc1", dcs[1].Meta.Name)

	_, err := filterDcs(kc, []string{"dc3", "dc4"})
	require.Error(t, err)
}

// A K8ssandraTask that references a datacenter that was removed from the K8ssandraCluster must still be deletable.
func TestDeleteK8ssandraTaskWithRemovedDatacenter(t *testing.T) {
	ctx := context.Background()
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, api.AddToScheme(s))
	require.NoError(t, k8capi.AddToScheme(s))
	require.NoError(t, cassapi.AddToScheme(s))

	kc := newCluster("ns", newDc("dc1", ""))
	kc.UID = "kc-uid"
	now := metav1.Now()
	kTask := &api.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         "ns",
			Name:              "refresh-2-1",
			Finalizers:        []string{k8ssandraTaskFinalizer},
			DeletionTimestamp: &now,
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: k8capi.GroupVersion.String(), Kind: "K8ssandraCluster", Name: kc.Name, UID: kc.UID}},
		},
		Spec: api.K8ssandraTaskSpec{
			Cluster:     corev1.ObjectReference{Name: kc.Name},
			Datacenters: []string{"dc1", "removed-dc"},
		},
	}
	cTask := &cassapi.CassandraTask{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: cassandraTaskName(kTask, "dc1")}}

	c := fake.NewClientBuilder().WithScheme(s).WithObjects(kc, kTask, cTask).Build()
	clientCache := clientcache.New(c, c, s)
	r := &K8ssandraTaskReconciler{
		ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second},
		Client:           c,
		Scheme:           s,
		ClientCache:      clientCache,
	}

	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(kTask)})
	require.NoError(t, err)

	err = c.Get(ctx, client.ObjectKeyFromObject(cTask), &cassapi.CassandraTask{})
	require.True(t, k8serrors.IsNotFound(err), "the CassandraTask of the remaining DC should be deleted")
	err = c.Get(ctx, client.ObjectKeyFromObject(kTask), &api.K8ssandraTask{})
	require.True(t, k8serrors.IsNotFound(err), "the K8ssandraTask finalizer should be removed")
}
