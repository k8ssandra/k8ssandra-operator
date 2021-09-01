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

func (r *defaultManagementApiFacade) CreateKeyspace(
	ctx context.Context,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	keyspaceName string,
	replication map[string]int,
	logger logr.Logger,
) error {
	if managementApiClient, err := r.createManagementApiClient(ctx, dc, remoteClient, logger); err != nil {
		logger.Error(err, "Failed to create management-api client")
		return err
	} else if pods, err := r.fetchDatacenterPods(ctx, dc, remoteClient); err != nil {
		logger.Error(err, "Failed to fetch datacenter pods")
		return err
	} else {
		for _, pod := range pods {
			if err := managementApiClient.CreateKeyspace(&pod, keyspaceName, r.createReplicationConfig(replication)); err != nil {
				logger.Error(err, fmt.Sprintf("Failed to CALL create keyspace %s on pod %v", keyspaceName, pod.Name))
			} else {
				return nil
			}
		}
		return fmt.Errorf("CALL create keyspace %s failed on all datacenter %v pods", keyspaceName, dc.Name)
	}
}

func (r *defaultManagementApiFacade) createManagementApiClient(
	ctx context.Context,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger,
) (*httphelper.NodeMgmtClient, error) {
	if httpClient, err := httphelper.BuildManagementApiHttpClient(dc, remoteClient, ctx); err != nil {
		return nil, err
	} else if protocol, err := httphelper.GetManagementApiProtocol(dc); err != nil {
		return nil, err
	} else {
		return &httphelper.NodeMgmtClient{
			Client:   httpClient,
			Log:      logger,
			Protocol: protocol,
		}, nil
	}
}

func (r *defaultManagementApiFacade) fetchDatacenterPods(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	labels := client.MatchingLabels{cassdcapi.DatacenterLabel: dc.Name}
	if err := remoteClient.List(ctx, podList, labels); err != nil {
		return nil, err
	} else {
		pods := r.filterPods(podList.Items, func(pod corev1.Pod) bool {
			status := r.getCassandraContainerStatus(pod)
			return status != nil && status.Ready
		})
		if len(pods) == 0 {
			err = fmt.Errorf("no pods in READY state found in datacenter %v", dc.Name)
			return nil, err
		}
		return pods, nil
	}
}

func (r *defaultManagementApiFacade) filterPods(pods []corev1.Pod, filter func(corev1.Pod) bool) []corev1.Pod {
	if len(pods) == 0 {
		return pods
	}
	filtered := make([]corev1.Pod, 0)
	for _, pod := range pods {
		if filter(pod) {
			filtered = append(pods, pod)
		}
	}
	return filtered
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

func (r *defaultManagementApiFacade) getCassandraContainerStatus(pod corev1.Pod) *corev1.ContainerStatus {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "cassandra" {
			return &status
		}
	}
	return nil
}
