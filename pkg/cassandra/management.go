package cassandra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/operator/pkg/httphelper"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type ManagementApiFacade interface {
	CreateKeyspace(
		ctx context.Context,
		dc *cassdcapi.CassandraDatacenter,
		remoteClient client.Client,
		keyspaceName string,
		replication map[string]int,
		logger logr.Logger,
	) error
}

type defaultManagementApiFacade struct {
}

func NewManagementApiFacade() ManagementApiFacade {
	return &defaultManagementApiFacade{}
}

func (r *defaultManagementApiFacade) CreateKeyspace(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client, keyspaceName string, replication map[string]int, logger logr.Logger) error {
	logger.Info(fmt.Sprintf("Creating keyspace %s with %v", keyspaceName, replication))
	if managementApiClient, err := r.createManagementApiClient(ctx, dc, remoteClient, logger); err != nil {
		return err
	} else if pod, err := r.pickDatacenterPod(ctx, dc, remoteClient, logger); err != nil {
		return err
	} else if err = managementApiClient.CreateKeyspace(pod, "data_endpoint_auth", r.createReplicationConfig(replication)); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create keyspace %s", keyspaceName))
		return err
	}
	return nil
}

func (r *defaultManagementApiFacade) createManagementApiClient(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client, logger logr.Logger) (*httphelper.NodeMgmtClient, error) {
	if httpClient, err := httphelper.BuildManagementApiHttpClient(dc, remoteClient, ctx); err != nil {
		logger.Error(err, "Failed to create management-api client")
		return nil, err
	} else if protocol, err := httphelper.GetManagementApiProtocol(dc); err != nil {
		logger.Error(err, "Failed to get management-api protocol")
		return nil, err
	} else {
		return &httphelper.NodeMgmtClient{
			Client:   httpClient,
			Log:      logger,
			Protocol: protocol,
		}, nil
	}
}

func (r *defaultManagementApiFacade) pickDatacenterPod(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client, logger logr.Logger) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	labels := client.MatchingLabels{cassdcapi.DatacenterLabel: dc.Name}
	if err := remoteClient.List(ctx, podList, labels); err != nil {
		logger.Error(err, "Failed to list datacenter pods")
		return nil, err
	} else if len(podList.Items) == 0 {
		err = fmt.Errorf("no pods found in datacenter %v", dc.Name)
		logger.Error(err, err.Error())
		return nil, err
	} else {
		return &podList.Items[0], nil
	}
}

func (r *defaultManagementApiFacade) createReplicationConfig(replication map[string]int) []map[string]string {
	replicationConfig := make([]map[string]string, 0, len(replication))
	for dcName, dcRf := range replication {
		replicationConfig = append(replicationConfig, map[string]string{
			"dc_name":            dcName,
			"replication_factor": strconv.Itoa(dcRf),
		})
	}
	return replicationConfig
}
