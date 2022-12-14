package reaper

import (
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

var commonLabels = map[string]string{
	k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
	k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
	k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
	k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
}

func createResourceLabels(kc *k8ssandraapi.K8ssandraCluster) map[string]string {
	labels := map[string]string{
		k8ssandraapi.K8ssandraClusterNameLabel:      kc.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: kc.Namespace,
		k8ssandraapi.CreatedByLabel:                 k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
	}
	return utils.MergeMap(labels, commonLabels)
}

func createServiceAndDeploymentLabels(r *reaperapi.Reaper) map[string]string {
	labels := map[string]string{
		reaperapi.ReaperLabel:       r.Name,
		k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueReaperController,
	}

	kcName, nameFound := r.Labels[k8ssandraapi.K8ssandraClusterNameLabel]
	kcNamespace, namespaceFound := r.Labels[k8ssandraapi.K8ssandraClusterNamespaceLabel]

	if nameFound && namespaceFound {
		labels[k8ssandraapi.K8ssandraClusterNameLabel] = kcName
		labels[k8ssandraapi.K8ssandraClusterNamespaceLabel] = kcNamespace
	}

	return utils.MergeMap(labels, commonLabels)
}

func createDeploymentLabels(r *reaperapi.Reaper) map[string]string {
	labels := createServiceAndDeploymentLabels(r)
	if meta := r.Spec.ResourceMeta; meta != nil && meta.OrchestrationTags != nil {
		return utils.MergeMap(meta.OrchestrationTags.Labels, labels)
	}
	return labels
}

func createPodLabels(r *reaperapi.Reaper) map[string]string {
	labels := createServiceAndDeploymentLabels(r)
	if meta := r.Spec.ResourceMeta; meta != nil && r.Spec.ResourceMeta.ChildTags != nil {
		return utils.MergeMap(meta.ChildTags.Labels, labels)
	}
	return labels
}
