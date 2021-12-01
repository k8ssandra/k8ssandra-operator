package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// findSeeds queries for pods labeled as seeds. It does this for each DC, across all
// clusters.
func (r *K8ssandraClusterReconciler) findSeeds(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) ([]corev1.Pod, error) {
	pods := make([]corev1.Pod, 0)

	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
		if err != nil {
			logger.Error(err, "Failed to get remote client", "K8sContext", dcTemplate.K8sContext)
			return nil, err
		}

		namespace := kc.Namespace
		if dcTemplate.Meta.Namespace != "" {
			namespace = dcTemplate.Meta.Namespace
		}

		list := &corev1.PodList{}
		selector := map[string]string{
			cassdcapi.ClusterLabel:    kc.Spec.Cassandra.Cluster,
			cassdcapi.DatacenterLabel: dcTemplate.Meta.Name,
			cassdcapi.SeedNodeLabel:   "true",
		}

		dcKey := client.ObjectKey{Namespace: namespace, Name: dcTemplate.Meta.Name}

		if err := remoteClient.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
			logger.Error(err, "Failed to get seed pods", "K8sContext", dcTemplate.K8sContext, "DC", dcKey)
			return nil, err
		}

		pods = append(pods, list.Items...)
	}

	return pods, nil
}

// newEndpoints returns an Endpoints object who is named after the additional seeds service
// of dc.
func newEndpoints(dc *cassdcapi.CassandraDatacenter, seeds []corev1.Pod) *corev1.Endpoints {
	ep := corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   dc.Namespace,
			Name:        dc.GetAdditionalSeedsServiceName(),
			Labels:      dc.GetDatacenterLabels(),
			Annotations: map[string]string{},
		},
	}

	addresses := make([]corev1.EndpointAddress, 0, len(seeds))
	for _, seed := range seeds {
		addresses = append(addresses, corev1.EndpointAddress{
			IP: seed.Status.PodIP,
		})
	}

	ep.Subsets = []corev1.EndpointSubset{
		{
			Addresses: addresses,
		},
	}

	ep.Annotations[api.ResourceHashAnnotation] = utils.DeepHashString(ep)

	return &ep
}
