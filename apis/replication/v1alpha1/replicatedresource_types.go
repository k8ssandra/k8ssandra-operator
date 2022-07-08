package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// type ReplicatedResource interface {
// 	ReplicationTargets() []ReplicationTarget
// 	Selector() *metav1.LabelSelector
// }

type ReplicatedResourceSpec struct {
	// TODO Add Kind + ApiVersion?

	// Selector defines which secrets are replicated. If left empty, all the secrets are replicated
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// TargetContexts indicates the target clusters to which the secrets are replicated to. If empty, no clusters are targeted
	// +optional
	ReplicationTargets []ReplicationTarget `json:"replicationTargets,omitempty"`
}

type ReplicationTarget struct {
	// TODO Implement at some point
	// Namespace to replicate the data to in the target cluster. If left empty, current namespace is used.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// K8sContextName defines the target cluster name as set in the ClientConfig. If left empty, current cluster is assumed
	// +optional
	K8sContextName string `json:"k8sContextName,omitempty"`

	// TODO Add label selector for clusters (from ClientConfigs)
	// Selector defines which clusters are targeted.
	// +optional
	// Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

type ReplicationConditionType string

const (
	ReplicationDone ReplicationConditionType = "Replication"
)

type ReplicationCondition struct {
	// Cluster
	Cluster string `json:"cluster"`

	// Type of condition
	Type ReplicationConditionType `json:"type"`

	// Status of the replication to target cluster
	Status corev1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transited from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}
