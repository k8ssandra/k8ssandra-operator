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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
const (
	// StargateLabel is the distinctive label for all objects created by the Stargate controller. The label value is
	// the Stargate resource name.
	StargateLabel = "k8ssandra.io/stargate"

	// StargateDeploymentLabel is a distinctive label for pods targeted by a deployment created by the Stargate
	// controller. The label value is the Deployment name.
	StargateDeploymentLabel = "k8ssandra.io/stargate-deployment"

	DefaultStargateVersion = "1.0.36"
)

// StargateTemplate defines a template for deploying Stargate.
type StargateTemplate struct {

	// StargateContainerImage is the image characteristics to use for Stargate containers. Leave nil
	// to use a default image.
	// +optional
	StargateContainerImage *ContainerImage `json:"stargateContainerImage,omitempty"`

	// ServiceAccount is the service account name to use for Stargate pods.
	// +kubebuilder:default="default"
	// +optional
	ServiceAccount *string `json:"serviceAccount,omitempty"`

	// Resources is the Kubernetes resource requests and limits to apply, per Stargate pod. Leave
	// nil to use defaults.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// HeapSize sets the JVM heap size to use for Stargate. If no Resources are specified, this
	// value will also be used to set a default memory request and limit for the Stargate pods:
	// these will be set to HeapSize x2 and x4, respectively.
	// +kubebuilder:default="256Mi"
	// +optional
	HeapSize *resource.Quantity `json:"heapSize,omitempty"`

	// LivenessProbe sets the Stargate liveness probe. Leave nil to use defaults.
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// ReadinessProbe sets the Stargate readiness probe. Leave nil to use defaults.
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`

	// NodeSelector is an optional map of label keys and values to restrict the scheduling of Stargate nodes to workers
	// with matching labels.
	// Leave nil to let the controller reuse the same node selectors used for data pods in this datacenter, if any.
	// See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations are tolerations to apply to the Stargate pods.
	// Leave nil to let the controller reuse the same tolerations used for data pods in this datacenter, if any.
	// See https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity is the affinity to apply to all the Stargate pods.
	// Leave nil to let the controller reuse the same affinity rules used for data pods in this datacenter, if any.
	// See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// AllowStargateOnDataNodes allows Stargate pods to be scheduled on a worker node already hosting data pods for this
	// datacenter. The default is false, which means that Stargate pods will be scheduled on separate worker nodes.
	// Note: if the datacenter pods have HostNetwork:true, then the Stargate pods will inherit of it, in which case it
	// is possible that Stargate nodes won't be allowed to sit on data nodes even if this property is set to true,
	// because of port conflicts on the same IP address.
	// +optional
	// +kubebuilder:default=false
	AllowStargateOnDataNodes bool `json:"allowStargateOnDataNodes,omitempty"`

	// CassandraConfigMapRef is a reference to a ConfigMap that holds Cassandra configuration.
	// The map should have a key named cassandra_yaml.
	// +optional
	CassandraConfigMapRef *corev1.LocalObjectReference `json:"cassandraConfigMapRef,omitempty"`
}

// StargateClusterTemplate defines global rules to apply to all Stargate pods in all datacenters in the cluster.
// These rules will be merged with rules defined at datacenter level in a StargateDatacenterTemplate; dc-level rules
// have precedence over cluster-level ones.
type StargateClusterTemplate struct {
	StargateTemplate `json:",inline"`

	// Size is the number of Stargate instances to deploy in each datacenter. They will be spread evenly across racks.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Size int32 `json:"size"`
}

// StargateDatacenterTemplate defines rules to apply to all Stargate pods in a given datacenter.
// These rules will be merged with rules defined at rack level in a StargateRackTemplate; rack-level rules
// have precedence over datacenter-level ones.
type StargateDatacenterTemplate struct {
	StargateClusterTemplate `json:",inline"`

	// Racks allow customizing Stargate characteristics for specific racks in the datacenter.
	// +optional
	Racks []StargateRackTemplate `json:"racks,omitempty"`
}

// Coalesce compares this StargateDatacenterTemplate with the given StargateClusterTemplate and returns the first
// non-nil StargateDatacenterTemplate it finds.
// TODO revisit the merging strategy and/or find a better name for this method
func (in *StargateDatacenterTemplate) Coalesce(clusterTemplate *StargateClusterTemplate) *StargateDatacenterTemplate {
	if in == nil && clusterTemplate == nil {
		return nil
	} else if in == nil {
		return &StargateDatacenterTemplate{StargateClusterTemplate: *clusterTemplate}
	} else {
		return in
	}
}

// StargateRackTemplate defines custom rules for Stargate pods in a given rack.
// These rules will be merged with rules defined at datacenter level in a StargateDatacenterTemplate; rack-level rules
// have precedence over datacenter-level ones.
type StargateRackTemplate struct {
	StargateTemplate `json:",inline"`

	// Name is the rack name. It must correspond to an existing rack name in the CassandraDatacenter resource where
	// Stargate is being deployed, otherwise it will be ignored.
	// +kubebuilder:validation:MinLength=2
	Name string `json:"name"`
}

// Coalesce compares this StargateRackTemplate with the given StargateDatacenterTemplate and returns the first non-nil
// StargateTemplate it finds.
// TODO revisit the merging strategy and/or find a better name for this method
func (in *StargateRackTemplate) Coalesce(dcTemplate *StargateDatacenterTemplate) *StargateTemplate {
	if in == nil && dcTemplate == nil {
		return nil
	} else if in == nil {
		return &dcTemplate.StargateTemplate
	} else {
		return &in.StargateTemplate
	}
}

// StargateSpec defines the desired state of a Stargate resource.
type StargateSpec struct {
	StargateDatacenterTemplate `json:",inline"`

	// DatacenterRef is the namespace-local reference of a CassandraDatacenter resource where
	// Stargate should be deployed.
	// +kubebuilder:validation:Required
	DatacenterRef corev1.LocalObjectReference `json:"datacenterRef"`
}

type ContainerImage struct {

	// +kubebuilder:default="docker.io"
	// +optional
	Registry *string `json:"registry,omitempty"`

	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// +kubebuilder:default="latest"
	// +optional
	Tag *string `json:"tag,omitempty"`

	// +kubebuilder:default="IfNotPresent"
	// +optional
	PullPolicy *corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// StargateProgress is a word summarizing the state of a Stargate resource.
type StargateProgress string

const (
	StargateProgressPending   = StargateProgress("Pending")
	StargateProgressDeploying = StargateProgress("Deploying")
	StargateProgressRunning   = StargateProgress("Running")
)

// StargateStatus defines the observed state of a Stargate resource.
type StargateStatus struct {

	// Progress is the progress of this Stargate object.
	// +kubebuilder:validation:Enum=Pending;Deploying;Running
	Progress StargateProgress `json:"progress"`

	// +optional
	Conditions []StargateCondition `json:"conditions,omitempty"`

	// DeploymentRefs is the names of the Deployment objects that were created for this Stargate
	// object.
	// +optional
	DeploymentRefs []string `json:"deploymentRefs,omitempty"`

	// ServiceRef is the name of the Service object that was created for this Stargate
	// object.
	// +optional
	ServiceRef *string `json:"serviceRef,omitempty"`

	// ReadyReplicasRatio is a "X/Y" string representing the ratio between ReadyReplicas and
	// Replicas in the Stargate deployment.
	// +kubebuilder:validation:Pattern=\d+/\d+
	// +optional
	ReadyReplicasRatio *string `json:"readyReplicasRatio,omitempty"`

	// Total number of non-terminated pods targeted by the Stargate deployment (their labels match
	// the selector).
	// Will be zero if the deployment has not been created yet.
	Replicas int32 `json:"replicas"`

	// ReadyReplicas is the total number of ready pods targeted by the Stargate deployment.
	// Will be zero if the deployment has not been created yet.
	ReadyReplicas int32 `json:"readyReplicas"`

	// UpdatedReplicas is the total number of non-terminated pods targeted by the Stargate
	// deployment that have the desired template spec.
	// Will be zero if the deployment has not been created yet.
	UpdatedReplicas int32 `json:"updatedReplicas"`

	// Total number of available pods targeted by the Stargate deployment.
	// Will be zero if the deployment has not been created yet.
	AvailableReplicas int32 `json:"availableReplicas"`
}

type StargateConditionType string

const (
	StargateReady StargateConditionType = "Ready"
)

type StargateCondition struct {
	Type   StargateConditionType  `json:"type"`
	Status corev1.ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transited from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="DC",type=string,JSONPath=`.spec.datacenterRef.name`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.progress`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.readyReplicasRatio`
// +kubebuilder:printcolumn:name="Up-to-date",type=integer,JSONPath=`.status.updatedReplicas`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.availableReplicas`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Stargate is the Schema for the stargates API
type Stargate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of this Stargate resource.
	// +optional
	Spec StargateSpec `json:"spec,omitempty"`

	// Most recently observed status of this Stargate resource.
	// +optional
	Status StargateStatus `json:"status,omitempty"`
}

func (in *StargateStatus) IsReady() bool {
	if in == nil {
		return false
	}
	if in.Progress != StargateProgressRunning {
		return false
	}
	for _, condition := range in.Conditions {
		if condition.Type == StargateReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (in *StargateStatus) GetConditionStatus(conditionType StargateConditionType) corev1.ConditionStatus {
	if in != nil {
		for _, condition := range in.Conditions {
			if condition.Type == conditionType {
				return condition.Status
			}
		}
	}
	return corev1.ConditionUnknown
}

func (in *StargateStatus) SetCondition(condition StargateCondition) {
	for i, c := range in.Conditions {
		if c.Type == condition.Type {
			in.Conditions[i] = condition
			return
		}
	}
	in.Conditions = append(in.Conditions, condition)
}

func (in *Stargate) GetRackTemplate(name string) *StargateRackTemplate {
	for _, rack := range in.Spec.Racks {
		if rack.Name == name {
			return &rack
		}
	}
	return nil
}

// +kubebuilder:object:root=true

// StargateList contains a list of Stargate
type StargateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Stargate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Stargate{}, &StargateList{})
}
