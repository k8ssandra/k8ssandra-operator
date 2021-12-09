package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func (r *K8ssandraClusterReconciler) reconcileSeedsEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, seeds []corev1.Pod, remoteClient client.Client, logger logr.Logger) result.ReconcileResult {
	logger.Info("Reconciling seeds")

	// The following if block was basically taken straight out of cass-operator. See
	// https://github.com/k8ssandra/k8ssandra-operator/issues/210 for a detailed
	// explanation of why this is being done.

	desiredEndpoints := newEndpoints(dc, seeds)
	actualEndpoints := &corev1.Endpoints{}
	endpointsKey := client.ObjectKey{Namespace: desiredEndpoints.Namespace, Name: desiredEndpoints.Name}

	if err := remoteClient.Get(ctx, endpointsKey, actualEndpoints); err == nil {
		// If we already have an Endpoints object but no seeds then it probably means all
		// Cassandra pods are down or not ready for some reason. In this case we will
		// delete the Endpoints and let it get recreated once we have seed nodes.
		if len(seeds) == 0 {
			logger.Info("Deleting endpoints")
			if err := remoteClient.Delete(ctx, actualEndpoints); err != nil {
				logger.Error(err, "Failed to delete endpoints")
				return result.Error(err)
			}
		} else {
			if !utils.CompareAnnotations(actualEndpoints, desiredEndpoints, api.ResourceHashAnnotation) {
				logger.Info("Updating endpoints", "Endpoints", endpointsKey)
				actualEndpoints := actualEndpoints.DeepCopy()
				resourceVersion := actualEndpoints.GetResourceVersion()
				desiredEndpoints.DeepCopyInto(actualEndpoints)
				actualEndpoints.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualEndpoints); err != nil {
					logger.Error(err, "Failed to update endpoints", "Endpoints", endpointsKey)
					return result.Error(err)
				}
			}
		}
	} else {
		if errors.IsNotFound(err) {
			// If we have seeds then we want to go ahead and create the Endpoints. But
			// if we don't have seeds, then we don't need to do anything for a couple
			// of reasons. First, no seeds means that cass-operator has not labeled any
			// pods as seeds which would be the case when the CassandraDatacenter is f
			// first created and no pods have reached the ready state. Secondly, you
			// cannot create an Endpoints object that has both empty Addresses and
			// empty NotReadyAddresses.
			if len(seeds) > 0 {
				logger.Info("Creating endpoints", "Endpoints", endpointsKey)
				if err = remoteClient.Create(ctx, desiredEndpoints); err != nil {
					logger.Error(err, "Failed to create endpoints", "Endpoints", endpointsKey)
					return result.Error(err)
				}
			}
		} else {
			logger.Error(err, "Failed to get endpoints", "Endpoints", endpointsKey)
			return result.Error(err)
		}
	}
	return result.Continue()
}

// newEndpoints returns an Endpoints object who is named after the additional seeds service
// of dc.
func newEndpoints(dc *cassdcapi.CassandraDatacenter, seeds []corev1.Pod) *corev1.Endpoints {
	ep := &corev1.Endpoints{
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

	utils.AddHashAnnotation(ep, api.ResourceHashAnnotation)

	return ep
}
