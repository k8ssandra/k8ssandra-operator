package v1alpha1

import (
	"testing"
	"time"

	controlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRefreshGlobalStatus(t *testing.T) {
	// setup Task status data
	dcMap := generateDatacenters()

	kts := K8ssandraTaskStatus{
		CassandraTaskStatus: controlapi.CassandraTaskStatus{},
		Datacenters:         dcMap,
	}
	kt := K8ssandraTask{
		Status: kts,
	}
	kt.RefreshGlobalStatus(3)
	assert.Equal(t, 2, kt.Status.Active)
	assert.Equal(t, 1, kt.Status.Failed)
	assert.Equal(t, 1, kt.Status.Succeeded)
	assert.Equal(t, 0, kt.Status.StartTime.Compare(time.Date(2023, 6, 10, 10, 10, 10, 10, time.UTC)))
	assert.Equal(t, metav1.ConditionTrue, kt.GetConditionStatus(controlapi.JobRunning))
	assert.Equal(t, metav1.ConditionTrue, kt.GetConditionStatus(controlapi.JobFailed))
	// the test data does not have a completion time, so it should be Unknown
	assert.Equal(t, metav1.ConditionUnknown, kt.GetConditionStatus(controlapi.JobComplete))
}

func generateDatacenters() map[string]controlapi.CassandraTaskStatus {
	return map[string]controlapi.CassandraTaskStatus{
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
			Conditions: []metav1.Condition{
				{
					Type:   string(controlapi.JobComplete),
					Status: metav1.ConditionTrue,
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
			Conditions: []metav1.Condition{
				{
					Type:   string(controlapi.JobFailed),
					Status: metav1.ConditionTrue,
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
			Conditions: []metav1.Condition{
				{
					Type:   string(controlapi.JobRunning),
					Status: metav1.ConditionTrue,
				},
			},
		},
	}
}
