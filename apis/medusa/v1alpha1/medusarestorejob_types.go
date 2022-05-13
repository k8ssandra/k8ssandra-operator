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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MedusaRestoreJobSpec defines the desired state of MedusaRestoreJob
type MedusaRestoreJobSpec struct {
	// The name of the CassandraBackup to restore.
	Backup string `json:"backup"`

	// Name of the Cassandra datacenter to perform the restore on.
	CassandraDatacenter string `json:"cassandraDatacenter"`
}

// MedusaRestoreJobStatus defines the observed state of MedusaRestoreJob
type MedusaRestoreJobStatus struct {
	// A unique key that identifies the restore operation.
	RestoreKey string `json:"restoreKey"`

	RestorePrepared bool `json:"restorePrepared,omitempty"`

	StartTime metav1.Time `json:"startTime,omitempty"`

	FinishTime metav1.Time `json:"finishTime,omitempty"`

	DatacenterStopped metav1.Time `json:"datacenterStopped,omitempty"`

	InProgress []string `json:"inProgress,omitempty"`

	Finished []string `json:"finished,omitempty"`

	Failed []string `json:"failed,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MedusaRestoreJob is the Schema for the medusarestorejobs API
type MedusaRestoreJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MedusaRestoreJobSpec   `json:"spec,omitempty"`
	Status MedusaRestoreJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MedusaRestoreJobList contains a list of MedusaRestoreJob
type MedusaRestoreJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MedusaRestoreJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MedusaRestoreJob{}, &MedusaRestoreJobList{})
}
