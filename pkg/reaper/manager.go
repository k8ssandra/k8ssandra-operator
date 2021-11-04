package reaper

import (
	"context"
	"fmt"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/reaper-client-go/reaper"
	reapergo "github.com/k8ssandra/reaper-client-go/reaper"
)

type Manager interface {
	Connect(reaper *api.Reaper) error
	AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error
	VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error)
}

func NewManager() Manager {
	return &restReaperManager{}
}

type restReaperManager struct {
	reaperClient reaper.ReaperClient
}

func (r *restReaperManager) Connect(reaper *api.Reaper) error {
	// Include the namespace in case Reaper is deployed in a different namespace than
	// the CassandraDatacenter.
	reaperSvc := GetServiceName(reaper.Name) + "." + reaper.Namespace

	reaperClient, err := reapergo.NewReaperClient(fmt.Sprintf("http://%s:8080", reaperSvc))
	if err != nil {
		return err
	}
	r.reaperClient = reaperClient
	return nil
}

func (r *restReaperManager) AddClusterToReaper(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) error {
	return r.reaperClient.AddCluster(ctx, cassdc.Spec.ClusterName, cassdc.GetSeedServiceName())
}

func (r *restReaperManager) VerifyClusterIsConfigured(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter) (bool, error) {
	_, err := r.reaperClient.GetCluster(ctx, cassdc.Spec.ClusterName)
	if err != nil {
		if err == reaper.CassandraClusterNotFound {
			// We didn't have issues verifying the existence, but the cluster isn't there
			return false, nil
		}
		return false, err
	}
	return true, nil
}
