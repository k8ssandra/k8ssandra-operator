package utils

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	NameLabel      = "app.kubernetes.io/name"
	NameLabelValue = "k8ssandra-operator"

	InstanceLabel  = "app.kubernetes.io/instance"
	VersionLabel   = "app.kubernetes.io/version"
	ManagedByLabel = "app.kubernetes.io/managed-by"

	ComponentLabel               = "app.kubernetes.io/component"
	ComponentLabelValueCassandra = "cassandra"
	ComponentLabelValueStargate  = "stargate"
	ComponentLabelValueReaper    = "reaper"

	CreatedByLabel                                = "app.kubernetes.io/created-by"
	CreatedByLabelValueK8ssandraClusterController = "k8ssandracluster-controller"
	CreatedByLabelValueStargateController         = "stargate-controller"
	CreatedByLabelValueReaperController           = "reaper-controller"

	PartOfLabel      = "app.kubernetes.io/part-of"
	PartOfLabelValue = "k8ssandra"

	K8ssandraClusterNameLabel      = "k8ssandra.io/cluster-name"
	K8ssandraClusterNamespaceLabel = "k8ssandra.io/cluster-namespace"
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
	AddLabel(component, ManagedByLabel, NameLabelValue)
	AddLabel(component, K8ssandraClusterNameLabel, klusterKey.Name)
	AddLabel(component, K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// IsManagedBy checks whether the given component is managed by K8ssandra, and belongs to the K8ssandraCluster resource
// specified by klusterKey which specifies the namespace and name of the K8ssandraCluster.
func IsManagedBy(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, ManagedByLabel, NameLabelValue) &&
		HasLabelWithValue(component, K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// ManagedByLabels returns the labels used to identify a component managed by K8ssandra.
// klusterKey specifies the namespace and name of the K8ssandraCluster.
func ManagedByLabels(klusterKey client.ObjectKey) map[string]string {
	return map[string]string{
		ManagedByLabel:                 NameLabelValue,
		K8ssandraClusterNameLabel:      klusterKey.Name,
		K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	}
}

// IsCreatedByK8ssandraController returns true if this component was created by the k8ssandra-cluster controller, and
// belongs to the K8ssandraCluster resource specified by klusterKey. klusterKey referns to the namespace and
// name of the K8ssandraCluster.
func IsCreatedByK8ssandraController(component Labeled, klusterKey client.ObjectKey) bool {
	return HasLabelWithValue(component, CreatedByLabel, CreatedByLabelValueK8ssandraClusterController) &&
		HasLabelWithValue(component, K8ssandraClusterNameLabel, klusterKey.Name) &&
		HasLabelWithValue(component, K8ssandraClusterNamespaceLabel, klusterKey.Namespace)
}

// CreatedByK8ssandraControllerLabels returns the labels used to identify a component created by the k8ssandra-cluster
// controller, and belonging to the K8ssandraCluster resource specified by klusterKey, which is namespace and name
// of the K8ssandraCluster.
func CreatedByK8ssandraControllerLabels(klusterKey client.ObjectKey) map[string]string {
	return map[string]string{
		CreatedByLabel:                 CreatedByLabelValueK8ssandraClusterController,
		K8ssandraClusterNameLabel:      klusterKey.Name,
		K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	}
}
