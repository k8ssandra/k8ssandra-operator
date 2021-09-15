package cassandra

import (
	"context"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RemoteSeedsResolver interface {
	ResolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error)
}

type defaultSeedsResolver struct {
}

func NewRemoteSeedsResolver() RemoteSeedsResolver {
	return &defaultSeedsResolver{}
}

func (r *defaultSeedsResolver) ResolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error) {
	podList := &corev1.PodList{}
	labels := client.MatchingLabels{cassdcapi.DatacenterLabel: dc.Name}

	err := remoteClient.List(ctx, podList, labels)
	if err != nil {
		return nil, err
	}

	endpoints := make([]string, 0, 3)

	for _, pod := range podList.Items {
		endpoints = append(endpoints, pod.Status.PodIP)
		if len(endpoints) > 2 {
			break
		}
	}

	return endpoints, nil
}
