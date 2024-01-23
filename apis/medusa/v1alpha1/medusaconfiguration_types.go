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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MedusaConfigurationSpec defines the desired state of MedusaConfiguration
type MedusaConfigurationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// StorageProperties defines the storage backend settings to use for the backups.
	StorageProperties Storage `json:"storageProperties,omitempty"`
}

type MedusaConfigurationConditionType string

const (
	ControlStatusSecretAvailable MedusaConfigurationConditionType = "SecretAvailable"
	ControlStatusReady           MedusaConfigurationConditionType = "Ready"
)

// MedusaConfigurationStatus defines the observed state of MedusaConfiguration
type MedusaConfigurationStatus struct {
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// SetCondition sets the condition with the given type to the given status.
// Returns true if the condition was changed.
func (m *MedusaConfigurationStatus) SetCondition(msg MedusaConfigurationConditionType, status metav1.ConditionStatus) bool {
	condition := metav1.Condition{
		Type:               string(msg),
		Status:             status,
		Reason:             string(msg),
		Message:            string(msg),
		LastTransitionTime: metav1.Now(),
	}
	found := false
	updated := false
	for i, c := range m.Conditions {
		if c.Type == string(msg) {
			found = true
			if c.Status == status {
				continue
			}
			m.Conditions[i] = condition
			updated = true
		}
	}
	if !found {
		m.Conditions = append(m.Conditions, condition)
		updated = true
	}
	return updated
}

func (m *MedusaConfigurationStatus) SetConditionMessage(msg MedusaConfigurationConditionType, message string) {
	for i, c := range m.Conditions {
		if c.Type == string(msg) {
			m.Conditions[i].Message = message
			return
		}
	}
}

func (m *MedusaConfigurationStatus) GetCondition(msg MedusaConfigurationConditionType) *metav1.Condition {
	for _, c := range m.Conditions {
		if c.Type == string(msg) {
			return &c
		}
	}
	return nil
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MedusaConfiguration is the Schema for the medusaconfigurations API
type MedusaConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MedusaConfigurationSpec   `json:"spec,omitempty"`
	Status MedusaConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MedusaConfigurationList contains a list of MedusaConfiguration
type MedusaConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MedusaConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MedusaConfiguration{}, &MedusaConfigurationList{})
}
