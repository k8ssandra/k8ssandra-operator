package utils

import (
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
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
// Important: k8cName is the name of the K8ssandraCluster resource managing the component, not the name of the Cassandra
// cluster. IOW, it should be k8c.Name, NOT k8c.Spec.Cassandra.Cluster!
func SetManagedBy(component Labeled, k8cName string) {
	AddLabel(component, api.ManagedByLabel, api.NameLabelValue)
	AddLabel(component, api.K8ssandraClusterLabel, k8cName)
}

// IsManagedBy checks whether the given component is managed by K8ssandra, and belongs to the K8ssandraCluster resource
// specified by k8cName.
// Important: k8cName is the name of the K8ssandraCluster resource managing the component, not the name of the Cassandra
// cluster. IOW, it should be k8c.Name, NOT k8c.Spec.Cassandra.Cluster!
func IsManagedBy(component Labeled, k8cName string) bool {
	return HasLabelWithValue(component, api.ManagedByLabel, api.NameLabelValue) &&
		HasLabelWithValue(component, api.K8ssandraClusterLabel, k8cName)
}

// ManagedByLabels returns the labels used to identify a component managed by K8ssandra.
// Important: k8cName is the name of the K8ssandraCluster resource managing the component, not the name of the Cassandra
// cluster. IOW, it should be k8c.Name, NOT k8c.Spec.Cassandra.Cluster!
func ManagedByLabels(k8cName string) map[string]string {
	return map[string]string{
		api.ManagedByLabel:        api.NameLabelValue,
		api.K8ssandraClusterLabel: k8cName,
	}
}

// IsCreatedByK8ssandraController returns true if this component was created by the k8ssandra-cluster controller, and
// belongs to the K8ssandraCluster resource specified by k8cName.
// Important: k8cName is the name of the K8ssandraCluster resource managing the component, not the name of the Cassandra
// cluster. IOW, it should be k8c.Name, NOT k8c.Spec.Cassandra.Cluster!
func IsCreatedByK8ssandraController(component Labeled, k8cName string) bool {
	return HasLabelWithValue(component, api.CreatedByLabel, api.CreatedByLabelValueK8ssandraClusterController) &&
		HasLabelWithValue(component, api.K8ssandraClusterLabel, k8cName)
}

// CreatedByK8ssandraControllerLabels returns the labels used to identify a component created by the k8ssandra-cluster
// controller, and belonging to the K8ssandraCluster resource specified by k8cName.
// Important: k8cName is the name of the K8ssandraCluster resource managing the component, not the name of the Cassandra
// cluster. IOW, it should be k8c.Name, NOT k8c.Spec.Cassandra.Cluster!
func CreatedByK8ssandraControllerLabels(k8cName string) map[string]string {
	return map[string]string{
		api.CreatedByLabel:        api.CreatedByLabelValueK8ssandraClusterController,
		api.K8ssandraClusterLabel: k8cName,
	}
}
