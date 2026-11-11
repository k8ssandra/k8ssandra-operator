package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
)

// NewMutualTLSClientConfig creates a client TLS configuration that verifies
// both the configured trust chain and the expected server identity.
func NewMutualTLSClientConfig(caPEM, clientCertPEM, clientKeyPEM []byte, serverName string) (*tls.Config, error) {
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		return nil, fmt.Errorf("TLS server name must not be empty")
	}

	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	clientCertificate, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{clientCertificate},
		RootCAs:      rootCAs,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
