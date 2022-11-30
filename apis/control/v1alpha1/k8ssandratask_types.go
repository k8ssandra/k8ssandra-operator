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
	cassapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8ssandraTaskSpec defines the desired state of K8ssandraTask
type K8ssandraTaskSpec struct {

	// Which K8ssandraCluster this task is operating on.
	Cluster corev1.ObjectReference `json:"cluster,omitempty"`

	// The names of the targeted datacenters. If omitted, will default to all DCs in spec order.
	Datacenters []string `json:"datacenters,omitempty"`

	// TODO replace with CassandraTaskTemplate (once k8ssandra/cass-operator#458 merged)
	Template cassapi.CassandraTaskSpec `json:"template,omitempty"`
}

// K8ssandraTaskStatus defines the observed state of K8ssandraTask
type K8ssandraTaskStatus struct {
	cassapi.CassandraTaskStatus `json:",inline"`

	// The individual progress of the CassandraTask in each datacenter.
	Datacenters map[string]cassapi.CassandraTaskStatus `json:"datacenters,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// +kubebuilder:printcolumn:name="Job",type=string,JSONPath=".spec.template.jobs[0].command",description="The job that is executed"
// +kubebuilder:printcolumn:name="Scheduled",type="date",JSONPath=".spec.template.scheduledTime",description="When the execution of the task is allowed at earliest"
// +kubebuilder:printcolumn:name="Started",type="date",JSONPath=".status.startTime",description="When the execution of the task started"
// +kubebuilder:printcolumn:name="Completed",type="date",JSONPath=".status.completionTime",description="When the execution of the task finished"
// K8ssandraTask is the Schema for the k8ssandratasks API
type K8ssandraTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   K8ssandraTaskSpec   `json:"spec,omitempty"`
	Status K8ssandraTaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// K8ssandraTaskList contains a list of K8ssandraTask
type K8ssandraTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []K8ssandraTask `json:"items"`
}

func (t *K8ssandraTask) GetClusterKey() client.ObjectKey {
	return client.ObjectKey{
		Namespace: utils.FirstNonEmptyString(t.Spec.Cluster.Namespace, t.Namespace),
		Name:      t.Spec.Cluster.Name,
	}
}

func (t *K8ssandraTask) BuildGlobalStatus() {
	firstStartTime := &metav1.Time{}
	lastCompletionTime := &metav1.Time{}
	totalActive := 0
	totalSucceeded := 0
	totalFailed := 0
	allComplete := true
	anyRunning := false
	anyFailed := false

	for _, dcStatus := range t.Status.Datacenters {
		if firstStartTime.IsZero() || dcStatus.StartTime.Before(firstStartTime) {
			firstStartTime = dcStatus.StartTime
		}
		if lastCompletionTime.IsZero() || lastCompletionTime.Before(dcStatus.CompletionTime) {
			lastCompletionTime = dcStatus.CompletionTime
		}
		totalActive += dcStatus.Active
		totalSucceeded += dcStatus.Succeeded
		totalFailed += dcStatus.Failed
		if getConditionStatus(dcStatus, cassapi.JobRunning) == corev1.ConditionTrue {
			anyRunning = true
		}
		if getConditionStatus(dcStatus, cassapi.JobFailed) == corev1.ConditionTrue {
			anyFailed = true
		}
		if getConditionStatus(dcStatus, cassapi.JobComplete) != corev1.ConditionTrue {
			allComplete = false
		}
	}

	t.Status.StartTime = firstStartTime
	t.Status.Active = totalActive
	t.Status.Succeeded = totalSucceeded
	t.Status.Failed = totalFailed
	if anyRunning {
		t.setCondition(cassapi.JobRunning, corev1.ConditionTrue)
	} else {
		t.setCondition(cassapi.JobRunning, corev1.ConditionFalse)
	}
	if anyFailed {
		t.setCondition(cassapi.JobFailed, corev1.ConditionTrue)
	} else {
		t.setCondition(cassapi.JobFailed, corev1.ConditionFalse)
	}
	if allComplete {
		t.Status.CompletionTime = lastCompletionTime
		t.setCondition(cassapi.JobComplete, corev1.ConditionTrue)
	}
}

func (t *K8ssandraTask) setCondition(condition cassapi.JobConditionType, status corev1.ConditionStatus) bool {
	existing := false
	for i := 0; i < len(t.Status.Conditions); i++ {
		cond := t.Status.Conditions[i]
		if cond.Type == condition {
			if cond.Status == status {
				// Already correct status
				return false
			}
			cond.Status = status
			cond.LastTransitionTime = metav1.Now()
			existing = true
			t.Status.Conditions[i] = cond
			break
		}
	}

	if !existing {
		cond := cassapi.JobCondition{
			Type:               condition,
			Status:             status,
			LastTransitionTime: metav1.Now(),
		}
		t.Status.Conditions = append(t.Status.Conditions, cond)
	}

	return true
}

func (t *K8ssandraTask) GetConditionStatus(conditionType cassapi.JobConditionType) corev1.ConditionStatus {
	for _, condition := range t.Status.Conditions {
		if condition.Type == conditionType {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func getConditionStatus(s cassapi.CassandraTaskStatus, conditionType cassapi.JobConditionType) corev1.ConditionStatus {
	for _, condition := range s.Conditions {
		if condition.Type == conditionType {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func init() {
	SchemeBuilder.Register(&K8ssandraTask{}, &K8ssandraTaskList{})
}
