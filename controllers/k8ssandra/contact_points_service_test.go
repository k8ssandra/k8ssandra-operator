package k8ssandra

import (
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/require"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
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
