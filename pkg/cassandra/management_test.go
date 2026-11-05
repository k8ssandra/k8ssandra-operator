package cassandra

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFetchDatacenterPodsIsNamespaceScoped(t *testing.T) {
	dc := newManagementApiTestDatacenter()
	remoteClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(
			newManagementApiTestPod(dc, dc.Namespace, "target-pod", true),
			newManagementApiTestPod(dc, "other", "other-pod", true),
		).
		Build()

	facade := &defaultManagementApiFacade{
		ctx:       context.Background(),
		dc:        dc,
		k8sClient: remoteClient,
	}
	pods, err := facade.fetchDatacenterPods()
	require.NoError(t, err)
	require.Len(t, pods, 1)
	require.Equal(t, "target-pod", pods[0].Name)
}

func TestFetchDatacenterPodsReturnsOnlyReadyPods(t *testing.T) {
	dc := newManagementApiTestDatacenter()
	remoteClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(
			newManagementApiTestPod(dc, dc.Namespace, "ready-pod", true),
			newManagementApiTestPod(dc, dc.Namespace, "unready-pod", false),
		).
		Build()

	facade := &defaultManagementApiFacade{
		ctx:       context.Background(),
		dc:        dc,
		k8sClient: remoteClient,
	}
	pods, err := facade.fetchDatacenterPods()
	require.NoError(t, err)
	require.Len(t, pods, 1)
	require.Equal(t, "ready-pod", pods[0].Name)
}

func newManagementApiTestDatacenter() *cassdcapi.CassandraDatacenter {
	return &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc1",
			Namespace: "target",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{ClusterName: "mydb"},
	}
}

func newManagementApiTestPod(dc *cassdcapi.CassandraDatacenter, namespace, name string, ready bool) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    dc.GetDatacenterLabels(),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "cassandra",
				Ready: ready,
			}},
		},
	}
}
