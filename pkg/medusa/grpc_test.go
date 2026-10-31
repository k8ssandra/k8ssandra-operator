package medusa

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/require"
)

func TestTransportCredentialsRequireServerName(t *testing.T) {
	factory := &DefaultFactory{}
	secret := &corev1.Secret{}

	_, err := factory.transportCredentials(secret, "")

	require.EqualError(t, err, "TLS server name must not be empty")
}
