package k8ssandra

import (
	"context"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDatacenterForDecommission(kc *api.K8ssandraCluster) string {
	dcNames := make([]string, 0)
	for _, dc := range kc.Spec.Cassandra.Datacenters {
		dcNames = append(dcNames, dc.Meta.Name)
	}

	// First look for a status that already has started decommission
	for dcName, status := range kc.Status.Datacenters {
		if !slices.Contains(dcNames, dcName) {
			if status.DecommissionProgress != api.DecommNone {
				return dcName
			}
		}
	}

	// No decommissions are in progress. Pick the first one we find.
	for dcName, _ := range kc.Status.Datacenters {
		if !slices.Contains(dcNames, dcName) {
			return dcName
		}
	}

	return ""
}

func GetK8ssandraClusterFromCassDc(ctx context.Context, cassdc *cassdcapi.CassandraDatacenter, client client.Client, logger logr.Logger) *api.K8ssandraCluster {
	kcName := cassdc.Spec.ClusterName
	if kcName == "" {
		logger.Info("CassandraDatacenter %s/%s does not have a cluster name", cassdc.Namespace, cassdc.Name)
		return nil
	}

	kcNamespace := cassdc.Labels[api.K8ssandraClusterNamespaceLabel]
	if kcNamespace == "" {
		logger.Info("CassandraDatacenter %s/%s does not have a k8ssandra cluster namespace label", cassdc.Namespace, cassdc.Name)
		return nil
	}

	// Read the K8ssandraCluster object from the API server
	kc := &api.K8ssandraCluster{}
	err := client.Get(ctx, types.NamespacedName{Namespace: kcNamespace, Name: kcName}, kc)
	if err != nil {
		logger.Error(err, "Failed to get K8ssandraCluster")
		return nil
	}

	return kc
}
