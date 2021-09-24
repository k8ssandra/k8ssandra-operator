package cassandra

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagementApiFactory creates request-scoped instances of ManagementApiFacade. This component exists
// mostly to allow tests to provide mocks for the Management API client.
type ManagementApiFactory interface {

	// NewManagementApiFacade returns a new ManagementApiFacade that will connect to the Management API of nodes in
	// the given datacenter. The k8sClient is used to fetch pods in that datacenter.
	NewManagementApiFacade(
		ctx context.Context,
		dc *cassdcapi.CassandraDatacenter,
		k8sClient client.Client,
		logger logr.Logger,
	) (ManagementApiFacade, error)
}

func NewManagementApiFactory() ManagementApiFactory {
	return &defaultManagementApiFactory{}
}

type defaultManagementApiFactory struct {
}

func (d defaultManagementApiFactory) NewManagementApiFacade(
	ctx context.Context,
	dc *cassdcapi.CassandraDatacenter,
	k8sClient client.Client,
	logger logr.Logger,
) (ManagementApiFacade, error) {
	if httpClient, err := httphelper.BuildManagementApiHttpClient(dc, k8sClient, ctx); err != nil {
		return nil, err
	} else if protocol, err := httphelper.GetManagementApiProtocol(dc); err != nil {
		return nil, err
	} else {
		nodeMgmtClient := &httphelper.NodeMgmtClient{
			Client:   httpClient,
			Log:      logger,
			Protocol: protocol,
		}
		return &defaultManagementApiFacade{
			ctx:            ctx,
			dc:             dc,
			nodeMgmtClient: nodeMgmtClient,
			k8sClient:      k8sClient,
			logger:         logger,
		}, nil
	}
}

// ManagementApiFacade is a component mirroring methods available on httphelper.NodeMgmtClient.
type ManagementApiFacade interface {

	// CreateKeyspaceIfNotExists calls the management API "/ops/keyspace/create" endpoint to create a new keyspace if it
	// does not exist yet. Calling this method on an existing keyspace is a no-op.
	CreateKeyspaceIfNotExists(
		keyspaceName string,
		replication map[string]int,
	) error

	ListKeyspaces(
		keyspaceName string,
	) ([]string, error)

	AlterKeyspace(
		keyspaceName string,
		replicationSettings map[string]int) error
}

type defaultManagementApiFacade struct {
	ctx            context.Context
	dc             *cassdcapi.CassandraDatacenter
	nodeMgmtClient *httphelper.NodeMgmtClient
	k8sClient      client.Client
	logger         logr.Logger
}

func (r *defaultManagementApiFacade) CreateKeyspaceIfNotExists(
	keyspaceName string,
	replication map[string]int,
) error {
	if pods, err := r.fetchDatacenterPods(); err != nil {
		r.logger.Error(err, "Failed to fetch datacenter pods")
		return err
	} else {
		for _, pod := range pods {
			if err := r.nodeMgmtClient.CreateKeyspace(&pod, keyspaceName, r.createReplicationConfig(replication)); err != nil {
				r.logger.Error(err, fmt.Sprintf("Failed to CALL create keyspace %s on pod %v", keyspaceName, pod.Name))
			} else {
				return nil
			}
		}
		return fmt.Errorf("CALL create keyspace %s failed on all datacenter %v pods", keyspaceName, r.dc.Name)
	}
}

func (r *defaultManagementApiFacade) fetchDatacenterPods() ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	labels := client.MatchingLabels{cassdcapi.DatacenterLabel: r.dc.Name}
	if err := r.k8sClient.List(r.ctx, podList, labels); err != nil {
		return nil, err
	} else {
		pods := r.filterPods(podList.Items, func(pod corev1.Pod) bool {
			status := r.getCassandraContainerStatus(pod)
			return status != nil && status.Ready
		})
		if len(pods) == 0 {
			err = fmt.Errorf("no pods in READY state found in datacenter %v", r.dc.Name)
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

func (r *defaultManagementApiFacade) ListKeyspaces(
	keyspaceName string,
) ([]string, error) {
	if pods, err := r.fetchDatacenterPods(); err != nil {
		r.logger.Error(err, "Failed to fetch datacenter pods")
		return []string{}, err
	} else {
		for _, pod := range pods {
			if keyspaces, err := r.nodeMgmtClient.GetKeyspace(&pod, keyspaceName); err != nil {
				r.logger.Error(err, fmt.Sprintf("Failed to CALL list keyspaces %s on pod %v", keyspaceName, pod.Name))
			} else {
				return keyspaces, nil
			}
		}
		return []string{}, fmt.Errorf("CALL list keyspaces %s failed on all datacenter %v pods", keyspaceName, r.dc.Name)
	}
}

func (r *defaultManagementApiFacade) AlterKeyspace(
	keyspaceName string,
	replicationSettings map[string]int,
) error {
	if pods, err := r.fetchDatacenterPods(); err != nil {
		r.logger.Error(err, "Failed to fetch datacenter pods")
		return err
	} else {
		for _, pod := range pods {
			if err := r.nodeMgmtClient.AlterKeyspace(&pod, keyspaceName, r.createReplicationConfig(replicationSettings)); err != nil {
				r.logger.Error(err, fmt.Sprintf("Failed to CALL alter keyspace %s on pod %v", keyspaceName, pod.Name))
			} else {
				r.logger.Info(fmt.Sprintf("Successfully altered keyspace %s replication", keyspaceName))
				return nil
			}
		}
		return fmt.Errorf("CALL alter keyspaces %s failed on all datacenter %v pods", keyspaceName, r.dc.Name)
	}
}
