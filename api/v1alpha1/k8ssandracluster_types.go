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
	cassdcv1beta1 "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// K8ssandraClusterSpec defines the desired state of K8ssandraCluster
type K8ssandraClusterSpec struct {
	Cassandra *Cassandra `json:"cassandra,omitempty"`
}

type Cassandra struct {
	Datacenters []CassandraDatacenterTemplateSpec `json:"datacenters,omitempty"`
}

type CassandraDatacenterTemplateSpec struct {
	Meta EmbeddedObjectMeta `json:"metadata,omitempty"`

	Spec cassdcv1beta1.CassandraDatacenterSpec `json:"spec,omitempty"`
}

// K8ssandraClusterStatus defines the observed state of K8ssandraCluster
type K8ssandraClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// K8ssandraCluster is the Schema for the k8ssandraclusters API
type K8ssandraCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   K8ssandraClusterSpec   `json:"spec,omitempty"`
	Status K8ssandraClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// K8ssandraClusterList contains a list of K8ssandraCluster
type K8ssandraClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []K8ssandraCluster `json:"items"`
}

type EmbeddedObjectMeta struct {
	Namespace string `json:"namespace,omitempty"`

	Name string `json:"name,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty"`
}

func init() {
	SchemeBuilder.Register(&K8ssandraCluster{}, &K8ssandraClusterList{})
}
