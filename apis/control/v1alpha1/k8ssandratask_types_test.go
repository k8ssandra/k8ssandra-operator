package v1alpha1

import (
	"testing"
	"time"

	"github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRefreshGlobalStatus(t *testing.T) {
	// setup Task status data
	dcMap := generateDatacenters()

	kts := K8ssandraTaskStatus{
		CassandraTaskStatus: v1alpha1.CassandraTaskStatus{},
		Datacenters:         dcMap,
	}
	kt := K8ssandraTask{
		Status: kts,
	}
	kt.RefreshGlobalStatus(3)
	assert.Equal(t, 2, kt.Status.Active)
	assert.Equal(t, 1, kt.Status.Failed)
	assert.Equal(t, 1, kt.Status.Succeeded)
	// assert.Equal(t, 0, kt.Status.StartTime.Compare(time.Date(2023, 6, 10, 10, 10, 10, 10, time.UTC)))
	assert.Equal(t, v1.ConditionTrue, kt.GetConditionStatus(v1alpha1.JobRunning))
	assert.Equal(t, v1.ConditionTrue, kt.GetConditionStatus(v1alpha1.JobFailed))
	// the test data does not have a completion time, so it should be Unknown
	assert.Equal(t, v1.ConditionUnknown, kt.GetConditionStatus(v1alpha1.JobComplete))
}

func generateDatacenters() map[string]v1alpha1.CassandraTaskStatus {
	return map[string]v1alpha1.CassandraTaskStatus{
		"dc1": {
			StartTime: &metav1.Time{
				Time: time.Date(2023, 6, 13, 13, 13, 13, 13, time.UTC),
			},
			CompletionTime: &metav1.Time{
				Time: time.Date(2023, 6, 14, 14, 14, 14, 14, time.UTC),
			},
			Active:    1,
			Succeeded: 1,
			Failed:    0,
			Conditions: []v1alpha1.JobCondition{
				{
					Type:   v1alpha1.JobComplete,
					Status: v1.ConditionTrue,
				},
			},
		},
		"dc2": {
			StartTime: &metav1.Time{
				Time: time.Date(2023, 6, 11, 11, 11, 11, 11, time.UTC),
			},
			CompletionTime: &metav1.Time{
				Time: time.Date(2023, 6, 12, 12, 12, 12, 12, time.UTC),
			},
			Active:    0,
			Succeeded: 0,
			Failed:    1,
			Conditions: []v1alpha1.JobCondition{
				{
					Type:   v1alpha1.JobFailed,
					Status: v1.ConditionTrue,
				},
			},
		},
		"dc3": {
			StartTime: &metav1.Time{
				Time: time.Date(2023, 6, 10, 10, 10, 10, 10, time.UTC),
			},
			CompletionTime: nil,
			Active:         1,
			Succeeded:      0,
			Failed:         0,
			Conditions: []v1alpha1.JobCondition{
				{
					Type:   v1alpha1.JobRunning,
					Status: v1.ConditionTrue,
				},
			},
		},
	}
}
