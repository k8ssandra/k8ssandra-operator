package k8ssandra

import (
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/require"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoadAllPodsEndpointsIsNamespaceScoped(t *testing.T) {
	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc1",
			Namespace: "target",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{ClusterName: "cluster1"},
	}

	endpointSlice := func(namespace, address string) *discoveryv1.EndpointSlice {
		labels := dc.GetDatacenterLabels()
		labels[discoveryv1.LabelServiceName] = dc.GetAllPodsServiceName()
		return &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dc.GetAllPodsServiceName(),
				Namespace: namespace,
				Labels:    labels,
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{address}}},
		}
	}

	remoteClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(
			endpointSlice(dc.Namespace, "10.0.1.1"),
			endpointSlice("other", "10.0.2.1"),
		).
		Build()

	reconciler := &K8ssandraClusterReconciler{}
	endpoints, err := reconciler.loadAllPodsEndpoints(t.Context(), dc, remoteClient)
	require.NoError(t, err)
	require.Len(t, endpoints, 1)
	require.Equal(t, "10.0.1.1", endpoints[0].Endpoints[0].Addresses[0])
}

// TestReconcileContactPointsServiceEndpoints verifies that the remote slices are mirrored into one local slice per
// address type, with one Endpoint per remote endpoint, and that the cleanup of the legacy Endpoints object does not
// fail when it does not exist.
func TestReconcileContactPointsServiceEndpoints(t *testing.T) {
	s := newUnitTestScheme(t)
	require.NoError(t, discoveryv1.AddToScheme(s))
	kc := &api.K8ssandraCluster{ObjectMeta: metav1.ObjectMeta{Name: "kc", Namespace: "ns"}}
	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "ns"},
		Spec:       cassdcapi.CassandraDatacenterSpec{ClusterName: "cluster1"},
	}

	remoteSlice := func(name string, addresses ...string) discoveryv1.EndpointSlice {
		endpoints := make([]discoveryv1.Endpoint, 0, len(addresses))
		for _, a := range addresses {
			endpoints = append(endpoints, discoveryv1.Endpoint{Addresses: []string{a}})
		}
		return discoveryv1.EndpointSlice{
			ObjectMeta:  metav1.ObjectMeta{Name: name, Namespace: dc.Namespace},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints:   endpoints,
		}
	}

	localClient := fake.NewClientBuilder().WithScheme(s).Build()
	r := &K8ssandraClusterReconciler{Client: localClient, Scheme: s}
	err := r.reconcileContactPointsServiceEndpoints(t.Context(), testlogr.NewTestLogger(t), kc, dc, []discoveryv1.EndpointSlice{
		remoteSlice("slice-a", "10.0.0.1", "10.0.0.2"),
		remoteSlice("slice-b", "10.0.0.3"),
	})
	require.NoError(t, err)

	slices := &discoveryv1.EndpointSliceList{}
	require.NoError(t, localClient.List(t.Context(), slices, client.InNamespace(kc.Namespace)))
	require.Len(t, slices.Items, 1)
	addresses := []string{}
	for _, e := range slices.Items[0].Endpoints {
		require.Len(t, e.Addresses, 1)
		addresses = append(addresses, e.Addresses[0])
	}
	require.ElementsMatch(t, []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, addresses)
}
