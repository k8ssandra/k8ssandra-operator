package reaper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"

	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
)

func createHTTPClientWithTLS() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}
}

func createHTTPClientWithMutualTLS(tlsCert, tlsKey, caCert []byte) (*http.Client, error) {
	cert, err := tls.X509KeyPair(tlsCert, tlsKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:          []tls.Certificate{cert},
				RootCAs:               caCertPool,
				InsecureSkipVerify:    true,
				VerifyPeerCertificate: buildVerifyPeerCertificateNoHostCheck(caCertPool)},
		},
	}, nil
}

// Below implementation modified from:
//
// https://go-review.googlesource.com/c/go/+/193620/5/src/crypto/tls/example_test.go#210
func buildVerifyPeerCertificateNoHostCheck(rootCAs *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	f := func(certificates [][]byte, _ [][]*x509.Certificate) error {
		certs := make([]*x509.Certificate, len(certificates))
		for i, asn1Data := range certificates {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				return err
			}
			certs[i] = cert
		}

		_, err := verifyPeerCertificateNoHostCheck(certs, rootCAs)
		return err
	}
	return f
}

func verifyPeerCertificateNoHostCheck(certificates []*x509.Certificate, rootCAs *x509.CertPool) ([][]*x509.Certificate, error) {
	opts := x509.VerifyOptions{
		Roots: rootCAs,
		// Setting the DNSName to the empty string will cause
		// Certificate.Verify() to skip hostname checking
		DNSName:       "",
		Intermediates: x509.NewCertPool(),
	}
	for _, cert := range certificates[1:] {
		opts.Intermediates.AddCert(cert)
	}
	chains, err := certificates[0].Verify(opts)
	return chains, err
}

type Manager interface {
	Connect(ctx context.Context, reaper *api.Reaper, username, password string) error
	ConnectWithReaperRef(ctx context.Context, kc *v1alpha1.K8ssandraCluster, username, password string) error
	AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error
	VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error)
	GetUiCredentials(ctx context.Context, uiUserSecretRef *corev1.LocalObjectReference, namespace string) (string, string, error)
	SetK8sClient(client.Reader)
}

func NewManager() Manager {
	return &restReaperManager{}
}

type restReaperManager struct {
	reaperClient reaperclient.Client
	k8sClient    client.Reader
}

func (r *restReaperManager) SetK8sClient(k8sClient client.Reader) {
	r.k8sClient = k8sClient
}

func (r *restReaperManager) ConnectWithReaperRef(ctx context.Context, kc *v1alpha1.K8ssandraCluster, username, password string) error {
	var namespace = kc.Spec.Reaper.ReaperRef.Namespace
	if namespace == "" {
		namespace = kc.Namespace
	}

	var opts []reaperclient.ClientCreateOption
	useTLS := false

	reaperSpec := kc.Spec.Reaper

	if reaperSpec.Encryption != nil {
		useTLS = true
		httpClientOption, err := r.httpClientOption(ctx, namespace, reaperSpec.Encryption)
		if err != nil {
			return fmt.Errorf("failed to create HTTP client option: %w", err)
		}
		opts = append(opts, httpClientOption)
	}

	reaperSvc := fmt.Sprintf("%s.%s", GetServiceName(kc.Spec.Reaper.ReaperRef.Name), namespace)
	return r.connect(ctx, reaperSvc, username, password, useTLS, opts...)
}

func (r *restReaperManager) Connect(ctx context.Context, reaper *api.Reaper, username, password string) error {
	var opts []reaperclient.ClientCreateOption
	useTLS := false

	reaperSpec := reaper.Spec

	if reaperSpec.Encryption != nil {
		useTLS = true
		httpClientOption, err := r.httpClientOption(ctx, reaper.Namespace, reaperSpec.Encryption)
		if err != nil {
			return fmt.Errorf("failed to create HTTP client option: %w", err)
		}
		opts = append(opts, httpClientOption)
	}

	reaperSvc := fmt.Sprintf("%s.%s", GetServiceName(reaper.Name), reaper.Namespace)
	return r.connect(ctx, reaperSvc, username, password, useTLS, opts...)
}

func (r *restReaperManager) httpClientOption(ctx context.Context, namespace string, encryption *api.ReaperEncryption) (reaperclient.ClientCreateOption, error) {
	if encryption.ClientCertName != "" {
		// Client must use mutual TLS
		secretKey := types.NamespacedName{
			Namespace: namespace,
			Name:      encryption.ClientCertName,
		}

		secret := &corev1.Secret{}
		if err := r.k8sClient.Get(ctx, secretKey, secret); err != nil {
			return nil, fmt.Errorf("failed to get client certificate secret %s: %w", encryption.ClientCertName, err)
		}

		tlsCert, ok := secret.Data["tls.crt"]
		if !ok {
			return nil, fmt.Errorf("tls.crt not found in secret %s", encryption.ClientCertName)
		}

		tlsKey, ok := secret.Data["tls.key"]
		if !ok {
			return nil, fmt.Errorf("tls.key not found in secret %s", encryption.ClientCertName)
		}

		caCert, ok := secret.Data["ca.crt"]
		if !ok {
			return nil, fmt.Errorf("ca.crt not found in secret %s", encryption.ClientCertName)
		}

		// Create HTTP client with mutual TLS
		httpClient, err := createHTTPClientWithMutualTLS(tlsCert, tlsKey, caCert)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client with mutual TLS: %w", err)
		}
		return reaperclient.WithHttpClient(httpClient), nil
	} else {
		// Client uses TLS without mutual authentication
		httpClient := createHTTPClientWithTLS()
		return reaperclient.WithHttpClient(httpClient), nil
	}

}

func (r *restReaperManager) connect(ctx context.Context, reaperSvc, username, password string, useTLS bool, opts ...reaperclient.ClientCreateOption) error {
	protocol := "http"
	if useTLS {
		protocol = "https"
	}
	u, err := url.Parse(fmt.Sprintf("%s://%s:8080", protocol, reaperSvc))
	if err != nil {
		return err
	}

	r.reaperClient = reaperclient.NewClient(u, opts...)

	if username != "" && password != "" {
		if err := r.reaperClient.Login(ctx, username, password); err != nil {
			return err
		}
	}
	return nil
}

func (r *restReaperManager) AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error {
	namespacedServiceName := cassdc.GetSeedServiceName() + "." + cassdc.Namespace
	return r.reaperClient.AddCluster(ctx, cassdcapi.CleanupForKubernetes(cassdc.Spec.ClusterName), namespacedServiceName)
}

func (r *restReaperManager) VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error) {
	clusters, err := r.reaperClient.GetClusterNames(ctx)
	if err != nil {
		return false, err
	}
	return utils.SliceContains(clusters, cassdcapi.CleanupForKubernetes(cassdc.Spec.ClusterName)), nil
}

func (r *restReaperManager) GetUiCredentials(ctx context.Context, uiUserSecretRef *corev1.LocalObjectReference, namespace string) (string, string, error) {
	if uiUserSecretRef == nil || uiUserSecretRef.Name == "" {
		// The UI user secret doesn't exist, meaning auth is disabled
		return "", "", nil
	}

	secretKey := types.NamespacedName{Namespace: namespace, Name: uiUserSecretRef.Name}

	secret := &corev1.Secret{}
	err := r.k8sClient.Get(ctx, secretKey, secret)
	if errors.IsNotFound(err) {
		return "", "", fmt.Errorf("reaper ui secret does not exist")
	} else if err != nil {
		return "", "", fmt.Errorf("failed to get reaper ui secret")
	} else {
		return string(secret.Data["username"]), string(secret.Data["password"]), nil
	}
}
