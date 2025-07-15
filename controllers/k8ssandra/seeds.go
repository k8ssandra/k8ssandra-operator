package k8ssandra

import (
	"context"
	"fmt"
	"net"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// findSeeds queries for pods labeled as seeds. It does this for each DC, across all
// clusters.
func (r *K8ssandraClusterReconciler) findSeeds(ctx context.Context, kc *api.K8ssandraCluster, cassClusterName string, logger logr.Logger) ([]corev1.Pod, error) {
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
			cassdcapi.ClusterLabel:    cassdcapi.CleanLabelValue(cassClusterName),
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

func (r *K8ssandraClusterReconciler) reconcileSeedsEndpoints(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	seeds []corev1.Pod,
	additionalSeeds []string,
	remoteClient client.Client,
	logger logr.Logger) result.ReconcileResult {
	logger.Info("Reconciling seeds")

	// Additional seed nodes should never be part of the current datacenter
	filteredSeeds := make([]string, 0)
	for _, seed := range seeds {
		if seed.Labels[cassdcapi.DatacenterLabel] != dc.Name {
			filteredSeeds = append(filteredSeeds, seed.Status.PodIP)
		}
	}
	filteredSeeds = append(filteredSeeds, additionalSeeds...)

	prefixName := fmt.Sprintf("%s-%s", kc.Name, dc.GetAdditionalSeedsServiceName())

	ipv4Addresses := make([]string, 0)
	ipv6Addresses := make([]string, 0)

	for _, additionalSeed := range filteredSeeds {
		ip := net.ParseIP(additionalSeed)
		if ip != nil {
			if ip.To4() != nil {
				ipv4Addresses = append(ipv4Addresses, additionalSeed)
			} else {
				ipv6Addresses = append(ipv6Addresses, additionalSeed)
			}
		}
		// In cass-operator, we support FQDN addresses also, but in k8ssandra-operator this feature is omitted on purpose.
		// The feature is not well supported in practise and we should reduce our maintenance burden by avoiding it
	}

	endpointSlices := make([]*discoveryv1.EndpointSlice, 0)

	ipv4Slice := reconciliation.CreateEndpointSlice(dc, prefixName, discoveryv1.AddressTypeIPv4, ipv4Addresses)
	endpointSlices = append(endpointSlices, ipv4Slice)

	ipv6Slice := reconciliation.CreateEndpointSlice(dc, prefixName, discoveryv1.AddressTypeIPv6, ipv6Addresses)
	endpointSlices = append(endpointSlices, ipv6Slice)

	// Can't set owner reference for EndpointSlice as they can be in different namespace than K8ssandraCluster

	if err := reconciliation.ReconcileEndpointSlices(ctx, remoteClient, logger, endpointSlices); err != nil {
		logger.Error(err, "Failed to reconcile additional seed endpoint slices")
		return result.Error(err)
	}

	// TODO Add code to cleanup old endpoints (or their slices as they are mirrored)

	return result.Continue()
}
