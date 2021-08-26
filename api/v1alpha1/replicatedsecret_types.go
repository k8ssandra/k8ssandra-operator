/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReplicatedSecretSpec defines the desired state of ReplicatedSecret
type ReplicatedSecretSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

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
	// Namespace string `json:"namespace,omitempty"`

	// K8sContextName defines the target cluster name as set in the ClientConfig. If left empty, current cluster is assumed
	// +optional
	K8sContextName string `json:"k8sContextName,omitempty"`

	// TODO Add label selector for clusters (from ClientConfigs)
	// Selector defines which clusters are targeted.
	// +optional
	// Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// ReplicatedSecretStatus defines the observed state of ReplicatedSecret
type ReplicatedSecretStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Conditions []ReplicationCondition `json:"conditions,omitempty"`
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

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ReplicatedSecret is the Schema for the replicatedsecrets API
type ReplicatedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicatedSecretSpec   `json:"spec,omitempty"`
	Status ReplicatedSecretStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReplicatedSecretList contains a list of ReplicatedSecret
type ReplicatedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicatedSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReplicatedSecret{}, &ReplicatedSecretList{})
}
