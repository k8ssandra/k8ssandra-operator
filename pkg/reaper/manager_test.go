package reaper

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"testing"
	"time"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestCreateHTTPClientWithMutualTLS(t *testing.T) {
	secret := newReaperTLSSecret(t, "reaper-client")

	httpClient, err := CreateHTTPClientWithMutualTLS(
		secret.Data["tls.crt"],
		secret.Data["tls.key"],
		secret.Data["ca.crt"],
		"test-reaper-service",
	)

	require.NoError(t, err)
	require.Equal(t, 10*time.Second, httpClient.Timeout)
	transport, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	require.Equal(t, "test-reaper-service", transport.TLSClientConfig.ServerName)
	require.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
	require.Len(t, transport.TLSClientConfig.Certificates, 1)
	require.NotNil(t, transport.TLSClientConfig.RootCAs)
}

func TestHTTPClientOptionTLSOnly(t *testing.T) {
	manager := &restReaperManager{}

	option, err := manager.httpClientOption(
		context.Background(),
		"service-test",
		&reaperapi.ReaperEncryption{},
		"test-reaper-service",
	)

	require.NoError(t, err)
	require.NotNil(t, option)
}

func TestHTTPClientOptionMutualTLS(t *testing.T) {
	const secretName = "reaper-client"

	tests := []struct {
		name          string
		secret        *corev1.Secret
		expectedError string
	}{
		{
			name:          "secret not found",
			expectedError: "failed to get client certificate secret reaper-client",
		},
		{
			name: "missing tls certificate",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "service-test"},
				Data:       map[string][]byte{},
			},
			expectedError: "tls.crt not found in secret reaper-client",
		},
		{
			name: "missing tls key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "service-test"},
				Data: map[string][]byte{
					"tls.crt": []byte("certificate"),
				},
			},
			expectedError: "tls.key not found in secret reaper-client",
		},
		{
			name: "missing ca certificate",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "service-test"},
				Data: map[string][]byte{
					"tls.crt": []byte("certificate"),
					"tls.key": []byte("key"),
				},
			},
			expectedError: "ca.crt not found in secret reaper-client",
		},
		{
			name: "invalid certificate data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "service-test"},
				Data: map[string][]byte{
					"tls.crt": []byte("certificate"),
					"tls.key": []byte("key"),
					"ca.crt":  []byte("ca"),
				},
			},
			expectedError: "failed to create HTTP client with mutual TLS: failed to parse CA certificate",
		},
		{
			name:   "valid certificate data",
			secret: newReaperTLSSecret(t, secretName),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder()
			if tt.secret != nil {
				clientBuilder = clientBuilder.WithObjects(tt.secret)
			}
			manager := &restReaperManager{k8sClient: clientBuilder.Build()}

			option, err := manager.httpClientOption(
				context.Background(),
				"service-test",
				&reaperapi.ReaperEncryption{ClientCertName: secretName},
				"test-reaper-service",
			)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				require.Nil(t, option)
			} else {
				require.NoError(t, err)
				require.NotNil(t, option)
			}
		})
	}
}

func TestConnectWithTLS(t *testing.T) {
	manager := &restReaperManager{}
	reaper := newTestReaper()
	reaper.Spec.Encryption = &reaperapi.ReaperEncryption{ServerCertName: "server-certificate"}

	err := manager.Connect(context.Background(), reaper, "", "")

	require.NoError(t, err)
	require.NotNil(t, manager.reaperClient)
}

func TestConnectWithReaperRefAndTLS(t *testing.T) {
	manager := &restReaperManager{}
	kc := &k8ssandraapi.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "service-test"},
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Reaper: &reaperapi.ReaperClusterTemplate{
				ReaperTemplate: reaperapi.ReaperTemplate{
					Encryption: &reaperapi.ReaperEncryption{ServerCertName: "server-certificate"},
				},
				ReaperRef: corev1.ObjectReference{Name: "shared-reaper"},
			},
		},
	}

	err := manager.ConnectWithReaperRef(context.Background(), kc, "", "")

	require.NoError(t, err)
	require.NotNil(t, manager.reaperClient)
}

func TestConnectWrapsTLSOptionErrors(t *testing.T) {
	manager := &restReaperManager{k8sClient: fake.NewClientBuilder().Build()}
	reaper := newTestReaper()
	reaper.Spec.Encryption = &reaperapi.ReaperEncryption{
		ServerCertName: "server-certificate",
		ClientCertName: "missing-client-certificate",
	}

	err := manager.Connect(context.Background(), reaper, "", "")

	require.ErrorContains(t, err, "failed to create HTTP client option: failed to get client certificate secret missing-client-certificate")
}

func TestConnectWithReaperRefWrapsTLSOptionErrors(t *testing.T) {
	manager := &restReaperManager{k8sClient: fake.NewClientBuilder().Build()}
	kc := &k8ssandraapi.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "service-test"},
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Reaper: &reaperapi.ReaperClusterTemplate{
				ReaperTemplate: reaperapi.ReaperTemplate{
					Encryption: &reaperapi.ReaperEncryption{
						ServerCertName: "server-certificate",
						ClientCertName: "missing-client-certificate",
					},
				},
				ReaperRef: corev1.ObjectReference{Name: "shared-reaper"},
			},
		},
	}

	err := manager.ConnectWithReaperRef(context.Background(), kc, "", "")

	require.ErrorContains(t, err, "failed to create HTTP client option: failed to get client certificate secret missing-client-certificate")
}

func newReaperTLSSecret(t *testing.T, name string) *corev1.Secret {
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
		Subject:      pkix.Name{CommonName: "reaper-client"},
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "service-test"},
		Data: map[string][]byte{
			"ca.crt":  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
			"tls.crt": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}),
			"tls.key": pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyDER}),
		},
	}
}
