package medusa

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
)

func GetCassandraDatacenterPods(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter, r client.Reader, logger logr.Logger) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	labels := client.MatchingLabels(cassdc.GetDatacenterLabels())
	if err := r.List(ctx, podList, labels, client.InNamespace(cassdc.Namespace)); err != nil {
		logger.Error(err, "failed to get pods for cassandradatacenter", "CassandraDatacenter", cassdc.DatacenterName())
		return nil, err
	}

	if podList.Items != nil {
		return podList.Items, nil
	}
	return nil, errors.New("podList came with nil Items field")
}
