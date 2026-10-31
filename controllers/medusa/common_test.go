package medusa

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusapkg "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/require"
)

func TestNewClientPassesConfiguredTLSServerName(t *testing.T) {
	cassdc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc1",
			Namespace: "test",
			Annotations: map[string]string{
				medusapkg.MedusaClientSecretNameAnnotation: "medusa-client",
				medusapkg.MedusaServerNameAnnotation:       "medusa-server.test.svc",
			},
		},
	}
	pod := &corev1.Pod{Status: corev1.PodStatus{PodIP: "192.0.2.10"}}
	k8sClient := fake.NewClientBuilder().WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "medusa-client", Namespace: "test"},
	}).Build()
	factory := &recordingMedusaClientFactory{}

	_, err := newClient(context.Background(), k8sClient, cassdc, pod, factory)

	require.NoError(t, err)
	require.Equal(t, "192.0.2.10:50051", factory.address)
	require.Equal(t, "medusa-server.test.svc", factory.serverName)
	require.Equal(t, "medusa-client", factory.secret.Name)
}

type recordingMedusaClientFactory struct {
	address    string
	secret     *corev1.Secret
	serverName string
}

func (f *recordingMedusaClientFactory) NewClient(address string) (medusapkg.Client, error) {
	f.address = address
	return nil, nil
}

func (f *recordingMedusaClientFactory) NewClientWithTLS(address string, secret *corev1.Secret, serverName string) (medusapkg.Client, error) {
	f.address = address
	f.secret = secret
	f.serverName = serverName
	return nil, nil
}
