package reaper

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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
	reaperSvc := fmt.Sprintf("%s.%s", GetServiceName(kc.Spec.Reaper.ReaperRef.Name), namespace)
	return r.connect(ctx, reaperSvc, username, password)
}

func (r *restReaperManager) Connect(ctx context.Context, reaper *api.Reaper, username, password string) error {
	reaperSvc := fmt.Sprintf("%s.%s", GetServiceName(reaper.Name), reaper.Namespace)
	return r.connect(ctx, reaperSvc, username, password)
}

func (r *restReaperManager) connect(ctx context.Context, reaperSvc, username, password string) error {
	u, err := url.Parse(fmt.Sprintf("http://%s:8080", reaperSvc))
	if err != nil {
		return err
	}
	r.reaperClient = reaperclient.NewClient(u)
	if username != "" && password != "" {
		if err := r.reaperClient.Login(ctx, username, password); err != nil {
			return err
		}
	}
	return nil
}

func cleanReaperName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func (r *restReaperManager) AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error {
	namespacedServiceName := cassdc.GetSeedServiceName() + "." + cassdc.Namespace
	return r.reaperClient.AddCluster(ctx, cleanReaperName(cassdc.Spec.ClusterName), namespacedServiceName)
}

func (r *restReaperManager) VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error) {
	clusters, err := r.reaperClient.GetClusterNames(ctx)
	if err != nil {
		return false, err
	}
	return utils.SliceContains(clusters, cleanReaperName(cassdc.Spec.ClusterName)), nil
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
