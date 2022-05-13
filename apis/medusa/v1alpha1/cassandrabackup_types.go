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
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CassandraBackupSpec defines the desired state of CassandraBackup
type CassandraBackupSpec struct {
	// The name of the backup.
	// TODO document format of generated name
	Name string `json:"name,omitempty"`

	// The name of the CassandraDatacenter to back up
	CassandraDatacenter string `json:"cassandraDatacenter"`

	// The type of the backup: "full" or "differential"
	// +kubebuilder:validation:Enum=differential;full;
	// +kubebuilder:default:=differential
	Type shared.BackupType `json:"backupType,omitempty"`
}

type CassandraDatacenterTemplateSpec struct {
	// Standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec cassdcapi.CassandraDatacenterSpec `json:"spec"`
}

// CassandraBackupStatus defines the observed state of CassandraBackup
type CassandraBackupStatus struct {
	CassdcTemplateSpec *CassandraDatacenterTemplateSpec `json:"cassdcTemplateSpec,omitempty"`

	StartTime metav1.Time `json:"startTime,omitempty"`

	FinishTime metav1.Time `json:"finishTime,omitempty"`

	InProgress []string `json:"inProgress,omitempty"`

	Finished []string `json:"finished,omitempty"`

	Failed []string `json:"failed,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="medusa.k8ssandra.com/v1alpha1 CassandraBackup/CassandraRestore are deprecated, use medusa.k8ssandra.com/v1alpha1 MedusaBackupJob/MedusaRestoreJob instead."

// CassandraBackup is the Schema for the cassandrabackups API
type CassandraBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraBackupSpec   `json:"spec,omitempty"`
	Status CassandraBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CassandraBackupList contains a list of CassandraBackup
type CassandraBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CassandraBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CassandraBackup{}, &CassandraBackupList{})
}
