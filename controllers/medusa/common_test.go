package medusa

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// recordingClientFactory captures the address passed to NewClient for verification.
type recordingClientFactory struct {
	lastAddress string
}

func (f *recordingClientFactory) NewClient(address string) (medusa.Client, error) {
	f.lastAddress = address
	return nil, nil
}

func (f *recordingClientFactory) NewClientWithTLS(address string, secret *corev1.Secret) (medusa.Client, error) {
	f.lastAddress = address
	return nil, nil
}

func TestNewClientAddressFormatting(t *testing.T) {
	tests := []struct {
		name            string
		podIP           string
		expectedAddress string
	}{
		{
			name:            "IPv4 address",
			podIP:           "192.0.2.10", // RFC 5737 documentation range
			expectedAddress: "192.0.2.10:50051",
		},
		{
			name:            "IPv6 address",
			podIP:           "2001:db8:ae4:ee06:1310::3", // RFC 3849 documentation range
			expectedAddress: "[2001:db8:ae4:ee06:1310::3]:50051",
		},
		{
			name:            "IPv6 loopback",
			podIP:           "::1",
			expectedAddress: "[::1]:50051",
		},
	}

	cassdc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc1",
			Namespace: "test-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &recordingClientFactory{}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Status: corev1.PodStatus{
					PodIP: tt.podIP,
				},
			}

			_, _ = newClient(context.Background(), nil, cassdc, pod, factory)

			require.NotEmpty(t, factory.lastAddress)
			assert.Equal(t, tt.expectedAddress, factory.lastAddress)
		})
	}
}
