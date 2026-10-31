package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewMutualTLSClientConfig(t *testing.T) {
	caCertPEM, caCert, caKey := newCertificateAuthority(t)
	clientCertPEM, clientKeyPEM := newSignedCertificate(t, caCert, caKey, certificateOptions{
		commonName: "client",
		usage:      []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	serverCertPEM, _ := newSignedCertificate(t, caCert, caKey, certificateOptions{
		commonName: "server",
		dnsNames:   []string{"service.namespace.svc"},
		usage:      []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	serverCertBlock, _ := pem.Decode(serverCertPEM)
	require.NotNil(t, serverCertBlock)
	serverCertificate, err := x509.ParseCertificate(serverCertBlock.Bytes)
	require.NoError(t, err)

	t.Run("configures strict peer verification", func(t *testing.T) {
		tlsConfig, err := NewMutualTLSClientConfig(caCertPEM, clientCertPEM, clientKeyPEM, "service.namespace.svc")

		require.NoError(t, err)
		require.False(t, tlsConfig.InsecureSkipVerify)
		require.Equal(t, "service.namespace.svc", tlsConfig.ServerName)
		require.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
		require.Len(t, tlsConfig.Certificates, 1)
		require.NotNil(t, tlsConfig.RootCAs)

		_, err = serverCertificate.Verify(x509.VerifyOptions{
			DNSName: "service.namespace.svc",
			Roots:   tlsConfig.RootCAs,
		})
		require.NoError(t, err)
	})

	t.Run("rejects a server certificate for a different name", func(t *testing.T) {
		tlsConfig, err := NewMutualTLSClientConfig(caCertPEM, clientCertPEM, clientKeyPEM, "other.namespace.svc")
		require.NoError(t, err)

		_, err = serverCertificate.Verify(x509.VerifyOptions{
			DNSName: tlsConfig.ServerName,
			Roots:   tlsConfig.RootCAs,
		})
		require.Error(t, err)
	})

	t.Run("rejects an empty server name", func(t *testing.T) {
		_, err := NewMutualTLSClientConfig(caCertPEM, clientCertPEM, clientKeyPEM, " ")

		require.EqualError(t, err, "TLS server name must not be empty")
	})

	t.Run("rejects invalid CA data", func(t *testing.T) {
		_, err := NewMutualTLSClientConfig([]byte("invalid"), clientCertPEM, clientKeyPEM, "service.namespace.svc")

		require.EqualError(t, err, "failed to parse CA certificate")
	})

	t.Run("rejects an invalid client key pair", func(t *testing.T) {
		_, err := NewMutualTLSClientConfig(caCertPEM, clientCertPEM, []byte("invalid"), "service.namespace.svc")

		require.ErrorContains(t, err, "failed to load client certificate")
	})
}

type certificateOptions struct {
	commonName string
	dnsNames   []string
	usage      []x509.ExtKeyUsage
}

func newCertificateAuthority(t *testing.T) ([]byte, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	certificate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	require.NoError(t, err)

	certificate, err = x509.ParseCertificate(der)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), certificate, key
}

func newSignedCertificate(
	t *testing.T,
	caCertificate *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	options certificateOptions,
) ([]byte, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: options.commonName},
		DNSNames:     options.dnsNames,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  options.usage,
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, caCertificate, &key.PublicKey, caKey)
	require.NoError(t, err)

	keyBytes, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
}
