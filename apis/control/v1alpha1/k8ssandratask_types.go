/*
Copyright 2022.

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
	"github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// K8ssandraTaskSpec defines the desired state of K8ssandraTask
type K8ssandraTaskSpec struct {

	// Which K8ssandraCluster this task is operating on.
	Cluster corev1.ObjectReference `json:"cluster,omitempty"`

	// The names of the targeted datacenters. If omitted, will default to all DCs in spec order.
	Datacenters []string `json:"datacenters,omitempty"`

	// TODO replace with CassandraTaskTemplate (once k8ssandra/cass-operator#458 merged)
	Template v1alpha1.CassandraTask `json:"template,omitempty"`
}

// K8ssandraTaskStatus defines the observed state of K8ssandraTask
type K8ssandraTaskStatus struct {

	// The overall progress across all datacenters.
	Global v1alpha1.CassandraTaskStatus `json:"global,omitempty"`

	// The individual progress of the CassandraTask in each datacenter.
	Datacenters map[string]v1alpha1.CassandraTaskStatus `json:"datacenters,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// K8ssandraTask is the Schema for the k8ssandratasks API
type K8ssandraTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   K8ssandraTaskSpec   `json:"spec,omitempty"`
	Status K8ssandraTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// K8ssandraTaskList contains a list of K8ssandraTask
type K8ssandraTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []K8ssandraTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&K8ssandraTask{}, &K8ssandraTaskList{})
}
