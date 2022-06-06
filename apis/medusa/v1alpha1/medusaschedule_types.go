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

// MedusaBackupScheduleSpec defines the desired state of MedusaBackupSchedule
type MedusaBackupScheduleSpec struct {
	// CronSchedule is a cronjob format schedule for backups. Overrides any easier methods of defining the schedule
	// +kubebuilder:validation:MinLength=1
	CronSchedule string `json:"cronSchedule"`

	// TODO Suspend / Disabled etc parameter?

	// BackupSpec defines the CassandraBackup to be created for this job
	BackupSpec MedusaBackupJobSpec `json:"backupSpec"`
}

// MedusaBackupScheduleStatus defines the observed state of MedusaBackupSchedule
type MedusaBackupScheduleStatus struct {
	// NextSchedule indicates when the next backup is going to be done
	NextSchedule metav1.Time `json:"nextSchedule,omitempty"`

	// LastExecution tells when the backup was last time taken. If empty, the backup has never been taken
	LastExecution metav1.Time `json:"lastExecution,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="Datacenter",type=string,JSONPath=".spec.backupSpec.cassandraDatacenter",description="Datacenter which the task targets"
// +kubebuilder:printcolumn:name="ScheduledExecution",type="date",JSONPath=".status.nextSchedule",description="Next scheduled execution time"
// +kubebuilder:printcolumn:name="LastExecution",type="date",JSONPath=".status.lastExecution",description="Previous execution time"
// +kubebuilder:printcolumn:name="BackupType",type="string",JSONPath=".spec.backupSpec.backupType",description="Type of backup"
// MedusaBackupSchedule is the Schema for the medusabackupschedules API
type MedusaBackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MedusaBackupScheduleSpec   `json:"spec,omitempty"`
	Status MedusaBackupScheduleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MedusaBackupScheduleList contains a list of MedusaBackupSchedule
type MedusaBackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MedusaBackupSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MedusaBackupSchedule{}, &MedusaBackupScheduleList{})
}
