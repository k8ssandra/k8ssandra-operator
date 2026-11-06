package medusa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// recordingClientFactory captures the address and the TLS secret passed to the factory.
type recordingClientFactory struct {
	address string
	secret  *corev1.Secret
}

func (f *recordingClientFactory) NewClient(address string) (medusa.Client, error) {
	f.address = address
	return nil, nil
}

func (f *recordingClientFactory) NewClientWithTLS(address string, secret *corev1.Secret) (medusa.Client, error) {
	f.address = address
	f.secret = secret
	return nil, nil
}

func TestNewClientAddress(t *testing.T) {
	const (
		namespace  = "test-ns"
		secretName = "medusa-client-secret"
	)

	tests := []struct {
		name            string
		podIP           string
		grpcPort        int32
		tls             bool
		expectedAddress string
	}{
		{
			name:            "IPv4 with default port",
			podIP:           "192.0.2.10",
			expectedAddress: "192.0.2.10:50051",
		},
		{
			name:            "IPv6 with default port",
			podIP:           "2001:db8:ae4:ee06:1310::3",
			expectedAddress: "[2001:db8:ae4:ee06:1310::3]:50051",
		},
		{
			name:            "IPv6 with port from medusa container",
			podIP:           "2001:db8::3",
			grpcPort:        50052,
			expectedAddress: "[2001:db8::3]:50052",
		},
		{
			name:            "IPv6 with TLS client secret",
			podIP:           "2001:db8::3",
			tls:             true,
			expectedAddress: "[2001:db8::3]:50051",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cassdc := &cassdcapi.CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: namespace},
			}
			k8sClient := fake.NewClientBuilder().Build()
			if tt.tls {
				metav1.SetMetaDataAnnotation(&cassdc.ObjectMeta, medusa.MedusaClientSecretNameAnnotation, secretName)
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}}
				k8sClient = fake.NewClientBuilder().WithObjects(secret).Build()
			}

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: namespace},
				Status:     corev1.PodStatus{PodIP: tt.podIP},
			}
			if tt.grpcPort != 0 {
				pod.Spec.Containers = []corev1.Container{{
					Name:  "medusa",
					Ports: []corev1.ContainerPort{{Name: "grpc", ContainerPort: tt.grpcPort}},
				}}
			}

			factory := &recordingClientFactory{}
			_, err := newClient(t.Context(), k8sClient, cassdc, pod, factory)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedAddress, factory.address)
			if tt.tls {
				require.NotNil(t, factory.secret, "expected the TLS client path to be used")
				assert.Equal(t, secretName, factory.secret.Name)
			} else {
				assert.Nil(t, factory.secret, "expected the plaintext client path to be used")
			}
		})
	}
}
