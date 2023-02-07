package labels

import (
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

// SetManagedBy sets the required labels for making a component managed by K8ssandra.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func SetManagedBy(component Labeled, klusterKey client.ObjectKey) {
	AddLabel(component, k8ssandraapi.ManagedByLabel, k8ssandraapi.NameLabelValue)
	AddLabel(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name)
	AddLabel(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// IsManagedBy checks whether the given component is managed by K8ssandra, and belongs to the K8ssandraCluster resource
// specified by klusterKey which specifies the namespace and name of the K8ssandraCluster.
func IsManagedBy(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, k8ssandraapi.ManagedByLabel, k8ssandraapi.NameLabelValue) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// ManagedByLabels returns the labels used to identify a component managed by K8ssandra.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func ManagedByLabels(klusterKey client.ObjectKey) map[string]string {
	return map[string]string{
		k8ssandraapi.ManagedByLabel:                 k8ssandraapi.NameLabelValue,
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

// IsPartOf returns true if this component was created by the k8ssandra-cluster controller, and belongs to the
// K8ssandraCluster resource specified by klusterKey. klusterKey refers to the namespace and name of the
// K8ssandraCluster.
func IsPartOf(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, k8ssandraapi.CreatedByLabel, k8ssandraapi.CreatedByLabelValueK8ssandraClusterController) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, k8ssandraapi.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// PartOfLabels returns the labels used to identify a component created by the k8ssandra-cluster controller, and
// belonging to the K8ssandraCluster resource specified by klusterKey, which is namespace and name of the
// K8ssandraCluster.
func PartOfLabels(klusterKey client.ObjectKey) map[string]string {
	// TODO add k8ssandraapi.PartOfLabel entry here, already done for telemetry objects elsewhere
	return map[string]string{
		k8ssandraapi.CreatedByLabel:                 k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterKey.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	}
}

// IsOwnedByK8ssandraController returns true if this component was created by the k8ssandra-cluster controller.
func IsOwnedByK8ssandraController(component Labeled) bool {
	return HasLabelWithValue(component, k8ssandraapi.CreatedByLabel, k8ssandraapi.CreatedByLabelValueK8ssandraClusterController) &&
		HasLabel(component, k8ssandraapi.K8ssandraClusterNameLabel) &&
		HasLabel(component, k8ssandraapi.K8ssandraClusterNamespaceLabel)
}
