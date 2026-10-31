package reaper

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateHTTPClientWithTLS(t *testing.T) {
	httpClient := createHTTPClientWithTLS("test-reaper-service")

	transport, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	require.Equal(t, "test-reaper-service", transport.TLSClientConfig.ServerName)
	require.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
	require.NotZero(t, httpClient.Timeout)
}

func TestCreateHTTPClientWithMutualTLSRequiresServerName(t *testing.T) {
	_, err := CreateHTTPClientWithMutualTLS(nil, nil, nil, "")

	require.EqualError(t, err, "TLS server name must not be empty")
}
