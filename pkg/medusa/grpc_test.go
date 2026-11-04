package medusa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

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

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCertificate, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "medusa-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCertificate, &clientKey.PublicKey, caKey)
	require.NoError(t, err)
	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	require.NoError(t, err)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			"ca.crt":  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
			"tls.crt": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}),
			"tls.key": pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyDER}),
		},
	}
}
