package reaper

import (
	"context"
	"fmt"
	"net/url"

	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
)

type Manager interface {
	Connect(ctx context.Context, reaper *api.Reaper, username, password string) error
	AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error
	VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error)
}

func NewManager() Manager {
	return &restReaperManager{}
}

type restReaperManager struct {
	reaperClient reaperclient.Client
}

func (r *restReaperManager) Connect(ctx context.Context, reaper *api.Reaper, username, password string) error {
	// Include the namespace in case Reaper is deployed in a different namespace than
	// the CassandraDatacenter.
	reaperSvc := GetServiceName(reaper.Name) + "." + reaper.Namespace
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

func (r *restReaperManager) AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error {
	return r.reaperClient.AddCluster(ctx, cassdcapi.CleanupForKubernetes(cassdc.Spec.ClusterName), cassdc.GetSeedServiceName())
}

func (r *restReaperManager) VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error) {
	clusters, err := r.reaperClient.GetClusterNames(ctx)
	if err != nil {
		return false, err
	}
	return utils.SliceContains(clusters, cassdcapi.CleanupForKubernetes(cassdc.Spec.ClusterName)), nil
}
