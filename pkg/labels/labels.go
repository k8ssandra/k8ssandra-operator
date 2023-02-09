package labels

import (
	k8ssandrataskapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
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

// LabelSet represents a set of labels keys and values that are applied to a K8ssandra component.
type LabelSet map[string]string

func newLabelSet(m map[string]string) *LabelSet {
	l := LabelSet(m)
	return &l
}

// AddTo adds the labels to the given component.
func (l *LabelSet) AddTo(component Labeled) {
	for key, value := range *l {
		AddLabel(component, key, value)
	}
}

// IsPresent checks whether the labels are present on the given component.
func (l *LabelSet) IsPresent(component Labeled) bool {
	for key, value := range *l {
		if !HasLabelWithValue(component, key, value) {
			return false
		}
	}
	return true
}

// MapOf builds a map with the labels from all the given sets.
func MapOf(ls ...*LabelSet) map[string]string {
	m := make(map[string]string)
	for _, l := range ls {
		for key, value := range *l {
			m[key] = value
		}
	}
	return m
}

// CassandraCommon is a set of common Kubernetes labels, for resources belonging to the "Cassandra" component.
var CassandraCommon = newLabelSet(map[string]string{
	k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
	k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
	k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueCassandra,
})

// WatchedByK8ssandraCluster returns the labels for making a component watched by a K8ssandraCluster. In other words, a
// modification of the component will trigger a reconciliation loop in K8ssandraClusterReconciler.
func WatchedByK8ssandraCluster(klusterKey client.ObjectKey) *LabelSet {
	return newLabelSet(map[string]string{
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterKey.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	})
}

// WatchedByK8ssandraTask returns the labels for making a component watched by a K8ssandraTask. In other words, a
// modification of the component will trigger a reconciliation loop in K8ssandraTaskReconciler.
func WatchedByK8ssandraTask(kTask *k8ssandrataskapi.K8ssandraTask) *LabelSet {
	return newLabelSet(map[string]string{
		k8ssandrataskapi.K8ssandraTaskNameLabel:      kTask.Name,
		k8ssandrataskapi.K8ssandraTaskNamespaceLabel: kTask.Namespace,
	})
}

// ReplicatedBy returns the labels that make a Secret selectable by a ReplicatedSecret for a K8ssandraCluster.
// This is a superset of WatchedByK8ssandraCluster.
func ReplicatedBy(klusterKey client.ObjectKey) *LabelSet {
	return newLabelSet(map[string]string{
		k8ssandraapi.ReplicatedByLabel:              k8ssandraapi.ReplicatedByLabelValue,
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterKey.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	})
}

// CleanedUpByK8ssandraCluster returns the labels that mark a component to be cleaned up when a K8ssandraCluster gets
// deleted.
// This is a superset of WatchedByK8ssandraCluster.
func CleanedUpByK8ssandraCluster(klusterKey client.ObjectKey) *LabelSet {
	return newLabelSet(map[string]string{
		k8ssandraapi.CreatedByLabel:                 k8ssandraapi.CreatedByLabelValueK8ssandraClusterController,
		k8ssandraapi.K8ssandraClusterNameLabel:      klusterKey.Name,
		k8ssandraapi.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
	})
}

// IsOwnedByK8ssandraController returns true if this component was created by the k8ssandra-cluster controller.
func IsOwnedByK8ssandraController(component Labeled) bool {
	// This is equivalent to CleanedUpByK8ssandraCluster(<any cluster>).IsPresent(component).
	// Which is kind of a hack because this only gets called for Stargate resources, and we know the controller
	// always sets these labels for those.
	return HasLabelWithValue(component, k8ssandraapi.CreatedByLabel, k8ssandraapi.CreatedByLabelValueK8ssandraClusterController) &&
		HasLabel(component, k8ssandraapi.K8ssandraClusterNameLabel) &&
		HasLabel(component, k8ssandraapi.K8ssandraClusterNamespaceLabel)
}

// StargateCommon is a set of common Kubernetes labels, for resources belonging to the "Stargate" component.
var StargateCommon = newLabelSet(map[string]string{
	k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
	k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
	k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueStargate,
	// Unlike K8ssandraCluster, there's no particular semantic meaning for Stargate, except in
	// ManagedByStargateServiceMonitor.
	k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueStargateController,
})

// StargateName returns the label used to identify components as parts of a given Stargate resource.
// In most case, this label is purely informational, but it is also used to match Deployments, as well as in the
// selector for the Stargate Service.
func StargateName(stargateName string) *LabelSet {
	return newLabelSet(map[string]string{
		stargateapi.StargateLabel: stargateName,
	})
}

// ManagedByStargateDeployment returns the label used in the Stargate deployment's selector.
func ManagedByStargateDeployment(deploymentName string) *LabelSet {
	return newLabelSet(map[string]string{
		stargateapi.StargateDeploymentLabel: deploymentName,
	})
}

// ManagedByStargateServiceMonitor returns the labels used in the Stargate ServiceMonitor's selector.
func ManagedByStargateServiceMonitor(stargateName string) *LabelSet {
	return newLabelSet(map[string]string{
		stargateapi.StargateLabel:   stargateName,
		k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueStargateController,
	})
}
