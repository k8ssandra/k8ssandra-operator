package utils

import (
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
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

func HasLabelWithValue(component Labeled, labelKey string, labelValue string) bool {
	return GetLabel(component, labelKey) == labelValue
}

// SetManagedBy sets the required labels for making a component managed by K8ssandra.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func SetManagedBy(component Labeled, klusterKey client.ObjectKey) {
	AddLabel(component, api.ManagedByLabel, api.NameLabelValue)
	AddLabel(component, api.K8ssandraClusterNameLabel, klusterKey.Name)
	AddLabel(component, api.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// IsManagedBy checks whether the given component is managed by K8ssandra, and belongs to the K8ssandraCluster resource
// specified by klusterKey which specifies the namespace and name of the K8ssandraCluster.
func IsManagedBy(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, api.ManagedByLabel, api.NameLabelValue) &&
		HasLabelWithValue(component, api.K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, api.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// IsCreatedByK8ssandraController returns true if this component was created by the k8ssandra-cluster controller, and
// belongs to the K8ssandraCluster resource specified by klusterKey. klusterKey referns to the namespace and
// name of the K8ssandraCluster.
func IsCreatedByK8ssandraController(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, api.CreatedByLabel, api.CreatedByLabelValueK8ssandraClusterController) &&
		HasLabelWithValue(component, api.K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, api.K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// CreatedByK8ssandraControllerLabels returns the labels used to identify a component created by the k8ssandra-cluster
// controller, and belonging to the K8ssandraCluster resource specified by klusterKey, which is namespace and name
// of the K8ssandraCluster.
func CreatedByK8ssandraControllerLabels(klusterKey client.ObjectKey) map[string]string {
	return map[string]string{
		api.CreatedByLabel:                 api.CreatedByLabelValueK8ssandraClusterController,
		api.K8ssandraClusterNameLabel:      klusterKey.Name,
		api.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	}
}
