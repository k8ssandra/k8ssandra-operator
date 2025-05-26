package labels

import (
	"github.com/adutra/goalesce"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Labeled interface {
	GetLabels() map[string]string
	SetLabels(labels map[string]string)
}

func AddLabel(obj Labeled, labelKey string, labelValue string) {
	m := obj.GetLabels()
	if m == nil {
		m = map[string]string{}
	}
	m[labelKey] = labelValue
	obj.SetLabels(m)
}

func GetLabel(component Labeled, labelKey string) string {
	m := component.GetLabels()
	return m[labelKey]
}

func HasLabel(component Labeled, labelKey string) bool {
	return GetLabel(component, labelKey) != ""
}

func HasLabelWithValue(component Labeled, labelKey string, labelValue string) bool {
	return GetLabel(component, labelKey) == labelValue
}

// SetWatchedByK8ssandraCluster sets the required labels for making a component watched by the K8ssandraCluster, i.e. a
// modification of the component will trigger a reconciliation loop in K8ssandraClusterReconciler.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func SetWatchedByK8ssandraCluster(component Labeled, klusterKey client.ObjectKey) {
	AddLabel(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name)
	AddLabel(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// IsWatchedByK8ssandraCluster checks whether the given component is watched by a K8ssandraCluster.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func IsWatchedByK8ssandraCluster(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// WatchedByK8ssandraClusterLabels returns the labels used to make a component watched by a K8ssandraCluster.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func WatchedByK8ssandraClusterLabels(klusterKey client.ObjectKey) map[string]string {
	return map[string]string{
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterKey.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	}
}

// SetReplicatedBy sets the required labels that make a Secret selectable by a ReplicatedSecret for a given
// K8ssandraCluster.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func SetReplicatedBy(component Labeled, klusterKey client.ObjectKey) {
	AddLabel(component, k8ssandraapi.ReplicatedByLabel, k8ssandraapi.ReplicatedByLabelValue)
	AddLabel(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name)
	AddLabel(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// IsReplicatedBy checks whether the given component (which in practice will be Secret) is selectable by a
// ReplicatedSecret for the given K8ssandraCluster.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func IsReplicatedBy(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, k8ssandraapi.ReplicatedByLabel, k8ssandraapi.ReplicatedByLabelValue) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// ReplicatedByLabels returns the labels used to make a Secret selectable by a ReplicatedSecret for the given
// K8ssandraCluster.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func ReplicatedByLabels(klusterKey client.ObjectKey) map[string]string {
	return map[string]string{
		k8ssandraapi.ReplicatedByLabel:              k8ssandraapi.ReplicatedByLabelValue,
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterKey.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	}
}

// IsCleanedUpBy returns whether this component is labelled to be cleaned up by the k8ssandra-cluster controller when
// the given K8ssandraCluster is deleted.
func IsCleanedUpBy(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, k8ssandraapi.CleanedUpByLabel, k8ssandraapi.CleanedUpByLabelValue) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// CleanedUpByLabels returns the labels used to indicate that a component should be cleaned up by the k8ssandra-cluster
// controller when the given K8ssandraCluster is deleted.
// (This is only used for cross-context references, when ownerReferences cannot be used).
func CleanedUpByLabels(klusterKey client.ObjectKey) map[string]string {
	return map[string]string{
		k8ssandraapi.CleanedUpByLabel:               k8ssandraapi.CleanedUpByLabelValue,
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterKey.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	}
}

// IsOwnedByK8ssandraController returns true if this component was created by the k8ssandra-cluster controller.
func IsOwnedByK8ssandraController(component Labeled) bool {
	return HasLabelWithValue(component, k8ssandraapi.CleanedUpByLabel, k8ssandraapi.CleanedUpByLabelValue) &&
		HasLabel(component, k8ssandraapi.K8ssandraClusterNameLabel) &&
		HasLabel(component, k8ssandraapi.K8ssandraClusterNamespaceLabel)
}

func AddCommonLabels(component Labeled, k8c *k8ssandraapi.K8ssandraCluster) {
	if k8c.Spec.Cassandra != nil && k8c.Spec.Cassandra.Meta.CommonLabels != nil {
		component.SetLabels(goalesce.MustDeepMerge(component.GetLabels(), k8c.Spec.Cassandra.Meta.CommonLabels))
	}
}
