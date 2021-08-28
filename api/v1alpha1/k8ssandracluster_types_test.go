package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestK8ssandraClusterStatus(t *testing.T) {
	t.Run("IsStargateAuthKeyspaceCreated", testK8ssandraClusterIsStargateAuthKeyspaceCreated)
	t.Run("SetStargateAuthKeyspaceCreated", testK8ssandraClusterSetStargateAuthKeyspaceCreated)
	t.Run("GetConditionStatus", testK8ssandraClusterGetConditionStatus)
	t.Run("SetConditionStatus", testK8ssandraClusterSetCondition)
	t.Run("SetCondition", testK8ssandraClusterSetCondition)
}

func testK8ssandraClusterIsStargateAuthKeyspaceCreated(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var status *K8ssandraClusterStatus = nil
		assert.False(t, status.IsStargateAuthKeyspaceCreated())
	})
	t.Run("no conditions", func(t *testing.T) {
		status := K8ssandraClusterStatus{}
		assert.False(t, status.IsStargateAuthKeyspaceCreated())
	})
	t.Run("condition status false", func(t *testing.T) {
		status := K8ssandraClusterStatus{
			Conditions: []K8ssandraClusterCondition{{Type: K8ssandraClusterStargateAuthKeyspaceCreated, Status: corev1.ConditionFalse}},
		}
		assert.False(t, status.IsStargateAuthKeyspaceCreated())
	})
	t.Run("condition status true", func(t *testing.T) {
		status := K8ssandraClusterStatus{
			Conditions: []K8ssandraClusterCondition{{Type: K8ssandraClusterStargateAuthKeyspaceCreated, Status: corev1.ConditionTrue}},
		}
		assert.True(t, status.IsStargateAuthKeyspaceCreated())
	})
}

func testK8ssandraClusterSetStargateAuthKeyspaceCreated(t *testing.T) {
	t.Run("nil receiver panics", func(t *testing.T) {
		var status *K8ssandraClusterStatus = nil
		assert.Panics(t, func() { status.SetStargateAuthKeyspaceCreated() })
	})
	t.Run("condition status set to true", func(t *testing.T) {
		status := K8ssandraClusterStatus{}
		status.SetStargateAuthKeyspaceCreated()
		assert.Len(t, status.Conditions, 1)
		assert.Equal(t, K8ssandraClusterStargateAuthKeyspaceCreated, status.Conditions[0].Type)
		assert.Equal(t, corev1.ConditionTrue, status.Conditions[0].Status)
	})
}

func testK8ssandraClusterGetConditionStatus(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var status *K8ssandraClusterStatus = nil
		assert.Equal(t, corev1.ConditionUnknown, status.GetConditionStatus(K8ssandraClusterStargateAuthKeyspaceCreated))
	})
	t.Run("no conditions", func(t *testing.T) {
		status := &K8ssandraClusterStatus{}
		assert.Equal(t, corev1.ConditionUnknown, status.GetConditionStatus(K8ssandraClusterStargateAuthKeyspaceCreated))
	})
	t.Run("unknown condition type", func(t *testing.T) {
		var k8ssandraCluster *K8ssandraClusterStatus = nil
		assert.Equal(t, corev1.ConditionUnknown, k8ssandraCluster.GetConditionStatus("nonexistent"))
	})
	t.Run("no matching condition type", func(t *testing.T) {
		status := K8ssandraClusterStatus{
			Conditions: []K8ssandraClusterCondition{{Type: "other", Status: corev1.ConditionTrue}},
		}
		assert.Equal(t, corev1.ConditionUnknown, status.GetConditionStatus(K8ssandraClusterStargateAuthKeyspaceCreated))
	})
	t.Run("matching condition type is false", func(t *testing.T) {
		status := K8ssandraClusterStatus{
			Conditions: []K8ssandraClusterCondition{{Type: K8ssandraClusterStargateAuthKeyspaceCreated, Status: corev1.ConditionFalse}},
		}
		assert.Equal(t, corev1.ConditionFalse, status.GetConditionStatus(K8ssandraClusterStargateAuthKeyspaceCreated))
	})
	t.Run("matching condition type is true", func(t *testing.T) {
		status := K8ssandraClusterStatus{
			Conditions: []K8ssandraClusterCondition{{Type: K8ssandraClusterStargateAuthKeyspaceCreated, Status: corev1.ConditionTrue}},
		}
		assert.Equal(t, corev1.ConditionTrue, status.GetConditionStatus(K8ssandraClusterStargateAuthKeyspaceCreated))
	})
}

func testK8ssandraClusterSetConditionStatus(t *testing.T) {
	t.Run("nil receiver panics", func(t *testing.T) {
		var status *K8ssandraClusterStatus = nil
		assert.Panics(t, func() { status.SetConditionStatus("condition1", corev1.ConditionTrue) })
	})
	t.Run("no conditions", func(t *testing.T) {
		status := &K8ssandraClusterStatus{}
		status.SetConditionStatus("condition1", corev1.ConditionTrue)
		assert.Contains(t, status.Conditions, K8ssandraClusterCondition{Type: "condition1", Status: corev1.ConditionTrue})
	})
	t.Run("condition type already exists", func(t *testing.T) {
		status := &K8ssandraClusterStatus{}
		status.SetConditionStatus("condition1", corev1.ConditionFalse)
		status.SetConditionStatus("condition1", corev1.ConditionTrue)
		assert.NotContains(t, status.Conditions, K8ssandraClusterCondition{Type: "condition1", Status: corev1.ConditionFalse})
		assert.Contains(t, status.Conditions, K8ssandraClusterCondition{Type: "condition1", Status: corev1.ConditionTrue})
	})
	t.Run("different condition type", func(t *testing.T) {
		status := &K8ssandraClusterStatus{}
		status.SetConditionStatus("condition1", corev1.ConditionTrue)
		status.SetConditionStatus("condition2", corev1.ConditionFalse)
		assert.Len(t, status.Conditions, 2)
		assert.Contains(t, status.Conditions, K8ssandraClusterCondition{Type: "condition1", Status: corev1.ConditionTrue})
		assert.Contains(t, status.Conditions, K8ssandraClusterCondition{Type: "condition2", Status: corev1.ConditionFalse})
	})
}

func testK8ssandraClusterSetCondition(t *testing.T) {
	success := K8ssandraClusterCondition{Type: "condition1", Status: corev1.ConditionTrue}
	failure := K8ssandraClusterCondition{Type: "condition1", Status: corev1.ConditionFalse}
	t.Run("nil receiver panics", func(t *testing.T) {
		var status *K8ssandraClusterStatus = nil
		assert.Panics(t, func() { status.SetCondition(success) })
	})
	t.Run("no conditions", func(t *testing.T) {
		status := &K8ssandraClusterStatus{}
		status.SetCondition(success)
		assert.Contains(t, status.Conditions, success)
	})
	t.Run("condition type already exists", func(t *testing.T) {
		status := &K8ssandraClusterStatus{}
		status.SetCondition(failure)
		status.SetCondition(success)
		assert.NotContains(t, status.Conditions, failure)
		assert.Contains(t, status.Conditions, success)
	})
	t.Run("different condition type", func(t *testing.T) {
		status := &K8ssandraClusterStatus{}
		status.SetCondition(success)
		other := K8ssandraClusterCondition{Type: "condition2", Status: corev1.ConditionFalse}
		status.SetCondition(other)
		assert.Len(t, status.Conditions, 2)
		assert.Contains(t, status.Conditions, success)
		assert.Contains(t, status.Conditions, other)
	})
}
