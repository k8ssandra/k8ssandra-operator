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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClientConfigSpec defines the desired state of KubeConfig
type ClientConfigSpec struct {
	// ContextName allows to override the object name for context-name. If not set, the ClientConfig.Name is used as context name
	// +kubebuilder:validation:optional
	ContextName string `json:"contextName,omitempty"`

	// KubeConfigSecret should reference an existing secret; the actual configuration will be read from
	// this secret's "kubeconfig" key.
	KubeConfigSecret corev1.LocalObjectReference `json:"kubeConfigSecret,omitempty"`
}

//+kubebuilder:object:root=true

// ClientConfig is the Schema for the kubeconfigs API
type ClientConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClientConfigSpec `json:"spec,omitempty"`
}

func (c *ClientConfig) GetContextName() string {
	if c.Spec.ContextName == "" {
		return c.Name
	}
	return c.Spec.ContextName
}

//+kubebuilder:object:root=true

// ClientConfigList contains a list of KubeConfig
type ClientConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClientConfig{}, &ClientConfigList{})
}
