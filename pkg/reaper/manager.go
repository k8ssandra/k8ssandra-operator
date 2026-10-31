package reaper

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/tlsconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
)

func createHTTPClientWithTLS(serverName string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName: serverName,
				MinVersion: tls.VersionTLS12,
			},
		},
		Timeout: 10 * time.Second,
	}
}

func CreateHTTPClientWithMutualTLS(tlsCert, tlsKey, caCert []byte, serverName string) (*http.Client, error) {
	tlsConfig, err := tlsconfig.NewMutualTLSClientConfig(caCert, tlsCert, tlsKey, serverName)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 10 * time.Second,
	}, nil
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
	reaperServiceName := GetServiceName(kc.Spec.Reaper.ReaperRef.Name)
	reaperSvc := fmt.Sprintf("%s.%s", reaperServiceName, namespace)

	if reaperSpec.Encryption != nil {
		useTLS = true
		httpClientOption, err := r.httpClientOption(ctx, namespace, reaperSpec.Encryption, reaperServiceName)
		if err != nil {
			return fmt.Errorf("failed to create HTTP client option: %w", err)
		}
		opts = append(opts, httpClientOption)
	}

	return r.connect(ctx, reaperSvc, username, password, useTLS, opts...)
}

func (r *restReaperManager) Connect(ctx context.Context, reaper *api.Reaper, username, password string) error {
	var opts []reaperclient.ClientCreateOption
	useTLS := false

	reaperSpec := reaper.Spec
	reaperServiceName := GetServiceName(reaper.Name)
	reaperSvc := fmt.Sprintf("%s.%s", reaperServiceName, reaper.Namespace)

	if reaperSpec.Encryption != nil {
		useTLS = true
		httpClientOption, err := r.httpClientOption(ctx, reaper.Namespace, reaperSpec.Encryption, reaperServiceName)
		if err != nil {
			return fmt.Errorf("failed to create HTTP client option: %w", err)
		}
		opts = append(opts, httpClientOption)
	}

	return r.connect(ctx, reaperSvc, username, password, useTLS, opts...)
}

func (r *restReaperManager) httpClientOption(ctx context.Context, namespace string, encryption *api.ReaperEncryption, serverName string) (reaperclient.ClientCreateOption, error) {
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
		httpClient, err := CreateHTTPClientWithMutualTLS(tlsCert, tlsKey, caCert, serverName)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client with mutual TLS: %w", err)
		}
		return reaperclient.WithHttpClient(httpClient), nil
	} else {
		// Client uses TLS without mutual authentication
		httpClient := createHTTPClientWithTLS(serverName)
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
