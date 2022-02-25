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
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MedusaBackupJobSpec defines the desired state of MedusaBackupJob
type MedusaBackupJobSpec struct {
	// The name of the CassandraDatacenter to back up
	CassandraDatacenter string `json:"cassandraDatacenter"`

	// The type of the backup: "full" or "differential"
	// +kubebuilder:validation:Enum=differential;full;
	// +kubebuilder:default:=differential
	Type shared.BackupType `json:"backupType,omitempty"`
}

// MedusaBackupJobStatus defines the observed state of MedusaBackupJob
type MedusaBackupJobStatus struct {
	StartTime metav1.Time `json:"startTime,omitempty"`

	FinishTime metav1.Time `json:"finishTime,omitempty"`

	InProgress []string `json:"inProgress,omitempty"`

	Finished []string `json:"finished,omitempty"`

	Failed []string `json:"failed,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MedusaBackupJob is the Schema for the medusabackupjobs API
type MedusaBackupJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MedusaBackupJobSpec   `json:"spec,omitempty"`
	Status MedusaBackupJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MedusaBackupJobList contains a list of MedusaBackupJob
type MedusaBackupJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MedusaBackupJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MedusaBackupJob{}, &MedusaBackupJobList{})
}
