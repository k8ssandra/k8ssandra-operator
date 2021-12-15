package reaper

import (
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

var commonLabels = map[string]string{
	utils.NameLabel:      utils.NameLabelValue,
	utils.PartOfLabel:    utils.PartOfLabelValue,
	utils.ComponentLabel: utils.ComponentLabelValueReaper,
	utils.ManagedByLabel: utils.NameLabelValue,
}

func createResourceLabels(kc *k8ssandraapi.K8ssandraCluster) map[string]string {
	labels := map[string]string{
		utils.K8ssandraClusterNameLabel:      kc.Name,
		utils.K8ssandraClusterNamespaceLabel: kc.Namespace,
		utils.CreatedByLabel:                 utils.CreatedByLabelValueK8ssandraClusterController,
	}
	return utils.MergeMap(labels, commonLabels)
}

func createServiceAndDeploymentLabels(r *reaperapi.Reaper) map[string]string {
	labels := map[string]string{
		reaperapi.ReaperLabel: r.Name,
		utils.CreatedByLabel:  utils.CreatedByLabelValueReaperController,
	}

	kcName, nameFound := r.Labels[utils.K8ssandraClusterNameLabel]
	kcNamespace, namespaceFound := r.Labels[utils.K8ssandraClusterNamespaceLabel]

	if nameFound && namespaceFound {
		labels[utils.K8ssandraClusterNameLabel] = kcName
		labels[utils.K8ssandraClusterNamespaceLabel] = kcNamespace
	}

	return utils.MergeMap(labels, commonLabels)
}
