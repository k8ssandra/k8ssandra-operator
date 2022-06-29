package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

var (
	quantity512Mi = resource.MustParse("512Mi")
)

func TestStargate(t *testing.T) {
	t.Run("GetRackTemplate", testStargateGetRackTemplate)
}

func TestStargateStatus(t *testing.T) {
	t.Run("IsReady", testStargateIsReady)
	t.Run("GetConditionStatus", testStargateGetConditionStatus)
	t.Run("SetCondition", testStargateSetCondition)
}

func testStargateGetRackTemplate(t *testing.T) {
	t.Run("no racks", func(t *testing.T) {
		stargate := Stargate{}
		actual := stargate.GetRackTemplate("irrelevant")
		assert.Nil(t, actual)
	})
	t.Run("rack not found", func(t *testing.T) {
		stargate := Stargate{Spec: StargateSpec{
			StargateDatacenterTemplate: StargateDatacenterTemplate{
				Racks: []StargateRackTemplate{{
					Name:             "rack1",
					StargateTemplate: StargateTemplate{HeapSize: &quantity512Mi},
				}},
			},
		}}
		actual := stargate.GetRackTemplate("rack2")
		assert.Nil(t, actual)
	})
	t.Run("rack found", func(t *testing.T) {
		rack1 := StargateRackTemplate{Name: "rack1"}
		stargate := Stargate{Spec: StargateSpec{
			StargateDatacenterTemplate: StargateDatacenterTemplate{
				Racks: []StargateRackTemplate{rack1},
			},
		}}
		actual := stargate.GetRackTemplate("rack1")
		assert.NotNil(t, actual)
		assert.Equal(t, &rack1, actual)
	})
}

func testStargateIsReady(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var stargate *StargateStatus = nil
		assert.False(t, stargate.IsReady())
	})
	t.Run("no status", func(t *testing.T) {
		stargate := StargateStatus{}
		assert.False(t, stargate.IsReady())
	})
	t.Run("status not running", func(t *testing.T) {
		stargate := StargateStatus{Progress: StargateProgressPending}
		assert.False(t, stargate.IsReady())
	})
	t.Run("no conditions", func(t *testing.T) {
		stargate := StargateStatus{Progress: StargateProgressRunning}
		assert.False(t, stargate.IsReady())
	})
	t.Run("condition status false", func(t *testing.T) {
		stargate := StargateStatus{
			Progress:   StargateProgressRunning,
			Conditions: []StargateCondition{{Type: StargateReady, Status: corev1.ConditionFalse}},
		}
		assert.False(t, stargate.IsReady())
	})
	t.Run("condition status true", func(t *testing.T) {
		stargate := StargateStatus{
			Progress:   StargateProgressRunning,
			Conditions: []StargateCondition{{Type: StargateReady, Status: corev1.ConditionTrue}},
		}
		assert.True(t, stargate.IsReady())
	})
}

func testStargateGetConditionStatus(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var stargate *StargateStatus = nil
		assert.Equal(t, corev1.ConditionUnknown, stargate.GetConditionStatus(StargateReady))
	})
	t.Run("no conditions", func(t *testing.T) {
		stargate := &StargateStatus{}
		assert.Equal(t, corev1.ConditionUnknown, stargate.GetConditionStatus(StargateReady))
	})
	t.Run("unknown condition type", func(t *testing.T) {
		var stargate *StargateStatus = nil
		assert.Equal(t, corev1.ConditionUnknown, stargate.GetConditionStatus("nonexistent"))
	})
	t.Run("no matching condition type", func(t *testing.T) {
		stargate := StargateStatus{
			Conditions: []StargateCondition{{Type: "other", Status: corev1.ConditionTrue}},
		}
		assert.Equal(t, corev1.ConditionUnknown, stargate.GetConditionStatus(StargateReady))
	})
	t.Run("matching condition type is false", func(t *testing.T) {
		stargate := StargateStatus{
			Conditions: []StargateCondition{{Type: StargateReady, Status: corev1.ConditionFalse}},
		}
		assert.Equal(t, corev1.ConditionFalse, stargate.GetConditionStatus(StargateReady))
	})
	t.Run("matching condition type is true", func(t *testing.T) {
		stargate := StargateStatus{
			Conditions: []StargateCondition{{Type: StargateReady, Status: corev1.ConditionTrue}},
		}
		assert.Equal(t, corev1.ConditionTrue, stargate.GetConditionStatus(StargateReady))
	})
}

func testStargateSetCondition(t *testing.T) {
	ready := StargateCondition{Type: StargateReady, Status: corev1.ConditionTrue}
	notReady := StargateCondition{Type: StargateReady, Status: corev1.ConditionFalse}
	t.Run("nil receiver panics", func(t *testing.T) {
		var stargate *StargateStatus = nil
		assert.Panics(t, func() { stargate.SetCondition(ready) })
	})
	t.Run("no conditions", func(t *testing.T) {
		stargate := &StargateStatus{}
		stargate.SetCondition(ready)
		assert.Contains(t, stargate.Conditions, ready)
	})
	t.Run("condition type already exists", func(t *testing.T) {
		stargate := &StargateStatus{}
		stargate.SetCondition(notReady)
		stargate.SetCondition(ready)
		assert.NotContains(t, stargate.Conditions, notReady)
		assert.Contains(t, stargate.Conditions, ready)
	})
	t.Run("different condition type", func(t *testing.T) {
		stargate := &StargateStatus{}
		stargate.SetCondition(ready)
		other := StargateCondition{Type: "other", Status: corev1.ConditionFalse}
		stargate.SetCondition(other)
		assert.Len(t, stargate.Conditions, 2)
		assert.Contains(t, stargate.Conditions, ready)
		assert.Contains(t, stargate.Conditions, other)
	})
}
