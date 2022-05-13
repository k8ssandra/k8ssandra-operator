/*


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

type CassandraDatacenterConfig struct {
	// The name to give the new, restored CassandraDatacenter
	Name string `json:"name"`

	// The name to give the C* cluster.
	ClusterName string `json:"clusterName"`
}

// CassandraRestoreSpec defines the desired state of CassandraRestore
type CassandraRestoreSpec struct {
	// The name of the CassandraBackup to restore
	Backup string `json:"backup"`

	// When true the restore will be performed on the source cluster from which the backup
	// was taken. There will be a rolling restart of the source cluster.
	InPlace bool `json:"inPlace,omitEmpty"`

	// When set to true, the cluster is shutdown before the restore is applied. This is necessary
	// process if there are schema changes between the backup and current schema. Recommended.
	Shutdown bool `json:"shutdown,omitEmpty"`

	CassandraDatacenter CassandraDatacenterConfig `json:"cassandraDatacenter"`
}

// CassandraRestoreStatus defines the observed state of CassandraRestore
type CassandraRestoreStatus struct {
	// A unique key that identifies the restore operation.
	RestoreKey string `json:"restoreKey"`

	StartTime metav1.Time `json:"startTime,omitempty"`

	FinishTime metav1.Time `json:"finishTime,omitempty"`

	DatacenterStopped metav1.Time `json:"datacenterStopped,omitempty"`

	InProgress []string `json:"inProgress,omitempty"`

	Finished []string `json:"finished,omitempty"`

	Failed []string `json:"failed,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="medusa.k8ssandra.com/v1alpha1 CassandraBackup/CassandraRestore are deprecated, use medusa.k8ssandra.com/v1alpha1 MedusaBackupJob/MedusaRestoreJob instead."

// CassandraRestore is the Schema for the cassandrarestores API
type CassandraRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraRestoreSpec   `json:"spec,omitempty"`
	Status CassandraRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CassandraRestoreList contains a list of CassandraRestore
type CassandraRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CassandraRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CassandraRestore{}, &CassandraRestoreList{})
}
