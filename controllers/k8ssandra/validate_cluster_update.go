package k8ssandra

import (
	"fmt"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

func validateK8ssandraCluster(kc api.K8ssandraCluster) error {
	oldDCs := make([]string, 0)
	for dcName, _ := range kc.Status.Datacenters {
		oldDCs = append(oldDCs, dcName)
	}
	newDcs := make([]string, 0)
	for _, dc := range kc.Spec.Cassandra.Datacenters {
		newDcs = append(newDcs, dc.Meta.Name)
	}
	wasAdded := false
	wasRemoved := false
	for _, dc := range kc.Spec.Cassandra.Datacenters {
		if !utils.SliceContains(oldDCs, dc.Meta.Name) {
			wasAdded = true
			break
		}
	}
	for dcName, _ := range kc.Status.Datacenters {
		if !utils.SliceContains(newDcs, dcName) {
			wasRemoved = true
			break
		}
	}
	if wasAdded && wasRemoved {
		return fmt.Errorf("cannot add and remove datacenters at the same time")
	}
	return nil
}
