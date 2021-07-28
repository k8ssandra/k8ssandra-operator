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
	"encoding/json"
	cassdcv1beta1 "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// K8ssandraClusterSpec defines the desired state of K8ssandraCluster
type K8ssandraClusterSpec struct {
	K8sContextsSecret string `json:"k8sContextsSecret,omitempty"`

	Cassandra *Cassandra `json:"cassandra,omitempty"`
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

type Cassandra struct {
	Cluster string `json:"cluster,omitempty"`

	Datacenters []CassandraDatacenterTemplateSpec `json:"datacenters,omitempty"`
}

// +kubebuilder:pruning:PreserveUnknownFields

type CassandraDatacenterTemplateSpec struct {
	Meta EmbeddedObjectMeta `json:"metadata,omitempty"`

	K8sContext string `json:"k8sContext,omitempty"`

	// TODO Determine which fields from CassandraDatacenterSpec should be copied here.
	// This is only a subset set of the fields. More fields do need to be copied. Some are
	// unnecessary though. Some belong at the cluster level. I have created
	// https://github.com/k8ssandra/k8ssandra-operator/issues/9 to sort it out.

	// +kubebuilder:validation:Minimum=1
	Size int32 `json:"size"`

	ServerVersion string `json:"serverVersion"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Config json.RawMessage `json:"config,omitempty"`

	// Kubernetes resource requests and limits, per pod
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	Racks []cassdcv1beta1.Rack `json:"racks,omitempty"`

	StorageConfig cassdcv1beta1.StorageConfig `json:"storageConfig"`

	// Stargate defines the desired deployment characteristics for Stargate. Leave nil to skip
	// deploying Stargate in this datacenter.
	// +optional
	Stargate *StargateTemplate `json:"stargate,omitempty"`
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
