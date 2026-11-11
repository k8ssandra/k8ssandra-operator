package medusa

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func TestTransportCredentialsRequireServerName(t *testing.T) {
	factory := &DefaultFactory{}
	secret := &corev1.Secret{}

	_, err := factory.transportCredentials(secret, "")

	require.EqualError(t, err, "TLS server name must not be empty")
}

func TestTransportCredentials(t *testing.T) {
	factory := &DefaultFactory{}
	secret := newMedusaTLSSecret(t, "medusa-client")

	transportCredentials, err := factory.transportCredentials(secret, "medusa-service")

	require.NoError(t, err)
	require.NotNil(t, transportCredentials)
}

func TestTransportCredentialsRejectInvalidCA(t *testing.T) {
	factory := &DefaultFactory{}
	secret := newMedusaTLSSecret(t, "medusa-client")
	secret.Data["ca.crt"] = []byte("invalid")

	_, err := factory.transportCredentials(secret, "medusa-service")

	require.EqualError(t, err, "failed to parse CA certificate")
}

func TestNewClient(t *testing.T) {
	factory := &DefaultFactory{}

	client, err := factory.NewClient("passthrough:///unused")

	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close())
}

func TestNewClientWithTLS(t *testing.T) {
	factory := &DefaultFactory{}
	secret := newMedusaTLSSecret(t, "medusa-client")

	client, err := factory.NewClientWithTLS("passthrough:///unused", secret, "medusa-service")

	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.Close())
}

func TestNewClientWithTLSWrapsCredentialError(t *testing.T) {
	factory := &DefaultFactory{}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "medusa-client"},
		Data:       map[string][]byte{},
	}

	_, err := factory.NewClientWithTLS("passthrough:///unused", secret, "medusa-service")

	require.EqualError(t, err, "failed to create transport credentials from secret medusa-client: failed to parse CA certificate")
}

func newMedusaTLSSecret(t *testing.T, name string) *corev1.Secret {
	t.Helper()

	server := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	certificate := server.TLS.Certificates[0]
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	require.NoError(t, err)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			"ca.crt":  certificatePEM,
			"tls.crt": certificatePEM,
			"tls.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		},
	}
}
