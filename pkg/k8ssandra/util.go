package k8ssandra

import (
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"k8s.io/utils/strings/slices"
)

func GetDatacenterForDecommission(kc *api.K8ssandraCluster) (string, string) {
	dcNames := make([]string, 0)
	for _, dc := range kc.Spec.Cassandra.Datacenters {
		dcNames = append(dcNames, dc.Meta.Name)
	}

	// First look for a status that already has started decommission
	for dcName, status := range kc.Status.Datacenters {
		if !slices.Contains(dcNames, dcName) {
			if status.DecommissionProgress != api.DecommNone {
				return dcName, dcNameOverride(kc.Status.Datacenters[dcName].Cassandra.DatacenterName)
			}
		}
	}

	// No decommissions are in progress. Pick the first one we find.
	for dcName := range kc.Status.Datacenters {
		if !slices.Contains(dcNames, dcName) {
			return dcName, dcNameOverride(kc.Status.Datacenters[dcName].Cassandra.DatacenterName)
		}
	}

	return "", ""
}

func dcNameOverride(datacenterName *string) string {
	if datacenterName != nil {
		return *datacenterName
	}
	return ""
}
