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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// var _ ReplicatedResource = &ReplicatedSecretSpec{}

// type ReplicatedResource interface {
// 	*ReplicatedResourceSpec
// }

// ReplicatedSecretSpec defines the desired state of ReplicatedSecret
type ReplicatedSecretSpec struct {
	*ReplicatedResourceSpec `json:",inline"`
	// // Selector defines which secrets are replicated. If left empty, all the secrets are replicated
	// // +optional
	// Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// // TargetContexts indicates the target clusters to which the secrets are replicated to. If empty, no clusters are targeted
	// // +optional
	// ReplicationTargets []ReplicationTarget `json:"replicationTargets,omitempty"`
}

// func (r *ReplicatedSecretSpec) ReplicationTargets() []ReplicationTarget {
// 	return r.ReplicationTargets
// }

// func (r *ReplicatedSecretSpec) Selector() *metav1.LabelSelector {
// 	return r.Selector
// }

// ReplicatedSecretStatus defines the observed state of ReplicatedSecret
type ReplicatedSecretStatus struct {
	// +optional
	Conditions []ReplicationCondition `json:"conditions,omitempty"`
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
