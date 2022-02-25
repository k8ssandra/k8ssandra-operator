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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MedusaTaskSpec defines the desired state of MedusaTask
type MedusaTaskSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Requested operation to perform.
	// +kubebuilder:validation:Enum=sync;purge;prepare_restore
	// +kubebuilder:validation:Required
	Operation OperationType `json:"operation,omitempty"`

	// The name of the CassandraDatacenter to run the task on
	CassandraDatacenter string `json:"cassandraDatacenter"`

	// Name of the backup.
	// Will be necessary for operations such as verify or status.
	// +kubebuilder:validation:Optional
	BackupName string `json:"backupName,omitempty"`

	// Restore key to use for the prepare_restore operation.
	// +kubebuilder:validation:Optional
	RestoreKey string `json:"restoreKey,omitempty"`
}

// MedusaTaskStatus defines the observed state of MedusaTask
type MedusaTaskStatus struct {
	StartTime metav1.Time `json:"startTime,omitempty"`

	FinishTime metav1.Time `json:"finishTime,omitempty"`

	InProgress []string `json:"inProgress,omitempty"`

	Finished []TaskResult `json:"finished,omitempty"`

	Failed []string `json:"failed,omitempty"`
}

type TaskResult struct {
	// Name of the pod that ran the task. Always populated.
	PodName string `json:"podName,omitempty"`

	// Number of backups that were purged. Only populated for purge tasks.
	NbBackupsPurged int `json:"nbBackupsPurged,omitempty"`

	// Number of objects/files that were purged. Only populated for purge tasks.
	NbObjectsPurged int `json:"nbObjectsPurged,omitempty"`

	// Total size of purged files. Only populated for purge tasks.
	TotalPurgedSize int `json:"totalPurgedSize,omitempty"`

	// Number of objects that couldn't be deleted due to Medusa GC grace. Only populated for purge tasks.
	TotalObjectsWithinGcGrace int `json:"totalObjectsWithinGcGrace,omitempty"`
}

type OperationType string

const (
	OperationTypePurge          = OperationType("purge")
	OperationTypeSync           = OperationType("sync")
	OperationTypePrepareRestore = OperationType("prepare_restore")
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MedusaTask is the Schema for the MedusaTasks API
type MedusaTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MedusaTaskSpec   `json:"spec,omitempty"`
	Status MedusaTaskStatus `json:"status,omitempty"`
}

func (task *MedusaTask) String() string {
	return fmt.Sprintf("%s/%s", task.Spec.Operation, task.Namespace)
}

//+kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MedusaTaskList contains a list of MedusaTask
type MedusaTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MedusaTask `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MedusaTask{}, &MedusaTaskList{})
}
