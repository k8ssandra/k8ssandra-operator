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

// MedusaBackupSpec defines the desired state of MedusaBackup
type MedusaBackupSpec struct {
	// The name of the CassandraDatacenter to back up
	CassandraDatacenter string `json:"cassandraDatacenter"`

	// The type of the backup: "full" or "differential"
	// +kubebuilder:validation:Enum=differential;full;
	// +kubebuilder:default:=differential
	Type shared.BackupType `json:"backupType,omitempty"`
}

// MedusaBackupStatus defines the observed state of MedusaBackup
type MedusaBackupStatus struct {
	StartTime     metav1.Time         `json:"startTime,omitempty"`
	FinishTime    metav1.Time         `json:"finishTime,omitempty"`
	TotalNodes    int32               `json:"totalNodes,omitempty"`
	FinishedNodes int32               `json:"finishedNodes,omitempty"`
	Nodes         []*MedusaBackupNode `json:"nodes,omitempty"`
	TotalFiles    int64               `json:"totalFiles,omitempty"`
	TotalSize     string              `json:"totalSize,omitempty"`
	Status        string              `json:"status,omitempty"`
}

type MedusaBackupNode struct {
	Host       string  `json:"host,omitempty"`
	Tokens     []int64 `json:"tokens,omitempty"`
	Datacenter string  `json:"datacenter,omitempty"`
	Rack       string  `json:"rack,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Started",type=date,JSONPath=".status.startTime",description="Backup start time"
//+kubebuilder:printcolumn:name="Finished",type=date,JSONPath=".status.finishTime",description="Backup finish time"
//+kubebuilder:printcolumn:name="Nodes",type=string,JSONPath=".status.totalNodes",description="Total number of nodes at the time of the backup"
//+kubebuilder:printcolumn:name="Files",type=integer,JSONPath=".status.totalFiles",description="Total number of files in the backup"
//+kubebuilder:printcolumn:name="Size",type=string,JSONPath=".status.totalSize",description="Human-readable total size of the backup"
//+kubebuilder:printcolumn:name="Completed",type=string,JSONPath=".status.finishedNodes",description="Number of nodes that completed this backup"
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=".status.status",description="Backup completion status"

// MedusaBackup is the Schema for the medusabackups API
type MedusaBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MedusaBackupSpec   `json:"spec,omitempty"`
	Status MedusaBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MedusaBackupList contains a list of MedusaBackup
type MedusaBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MedusaBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MedusaBackup{}, &MedusaBackupList{})
}
