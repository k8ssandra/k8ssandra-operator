package reaper

import (
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var commonLabels = map[string]string{
	k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
	k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
	k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
	k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
}

func createResourceLabels(kc *k8ssandraapi.K8ssandraCluster) map[string]string {
	return utils.MergeMap(
		commonLabels,
		k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}))
}

func getConstantLabels(r *reaperapi.Reaper) map[string]string {
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

	return utils.MergeMap(commonLabels, labels)
}

func createServiceAndDeploymentLabels(r *reaperapi.Reaper) map[string]string {
	labels := getConstantLabels(r)

	if meta := r.Spec.ResourceMeta; meta != nil {
		return utils.MergeMap(meta.CommonLabels, labels)
	}

	return labels
}

func createPodLabels(r *reaperapi.Reaper) map[string]string {
	labels := createServiceAndDeploymentLabels(r)
	if meta := r.Spec.ResourceMeta; meta != nil {
		return utils.MergeMap(labels, meta.Pods.Labels)
	}
	return labels
}
