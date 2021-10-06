package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

var (
	quantity256Mi = resource.MustParse("256Mi")
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

func TestStargateDatacenterTemplate(t *testing.T) {
	t.Run("Coalesce", testStargateDatacenterTemplateCoalesce)
}

func TestStargateRackTemplate(t *testing.T) {
	t.Run("Coalesce", testStargateRackTemplateCoalesce)
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

func testStargateDatacenterTemplateCoalesce(t *testing.T) {

	t.Run("Nil dc with nil cluster", func(t *testing.T) {
		var clusterTemplate *StargateClusterTemplate = nil
		var dcTemplate *StargateDatacenterTemplate = nil
		actual := dcTemplate.Coalesce(clusterTemplate)
		assert.Nil(t, actual)
	})

	t.Run("nil dc with non nil cluster", func(t *testing.T) {
		clusterTemplate := &StargateClusterTemplate{Size: 10}
		var dcTemplate *StargateDatacenterTemplate = nil
		expected := &StargateDatacenterTemplate{StargateClusterTemplate: *clusterTemplate}
		actual := dcTemplate.Coalesce(clusterTemplate)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Non nil dc with nil cluster", func(t *testing.T) {
		var clusterTemplate *StargateClusterTemplate = nil
		dcTemplate := &StargateDatacenterTemplate{
			StargateClusterTemplate: StargateClusterTemplate{Size: 10},
		}
		actual := dcTemplate.Coalesce(clusterTemplate)
		assert.NotNil(t, actual)
		assert.Equal(t, dcTemplate, actual)
	})

	t.Run("Non nil dc with non nil cluster", func(t *testing.T) {
		clusterTemplate := &StargateClusterTemplate{Size: 10}
		dcTemplate := &StargateDatacenterTemplate{
			StargateClusterTemplate: StargateClusterTemplate{Size: 20},
		}
		actual := dcTemplate.Coalesce(clusterTemplate)
		assert.NotNil(t, actual)
		assert.Equal(t, dcTemplate, actual)
	})
}

func testStargateRackTemplateCoalesce(t *testing.T) {

	t.Run("Nil rack with nil dc", func(t *testing.T) {
		var dcTemplate *StargateDatacenterTemplate = nil
		var rackTemplate *StargateRackTemplate = nil
		actual := rackTemplate.Coalesce(dcTemplate)
		assert.Nil(t, actual)
	})

	t.Run("Nil rack with non nil dc", func(t *testing.T) {
		dcTemplate := &StargateDatacenterTemplate{
			StargateClusterTemplate: StargateClusterTemplate{
				StargateTemplate: StargateTemplate{HeapSize: &quantity256Mi},
			},
		}
		var rackTemplate *StargateRackTemplate = nil
		actual := rackTemplate.Coalesce(dcTemplate)
		assert.NotNil(t, actual)
		assert.Equal(t, &dcTemplate.StargateTemplate, actual)
	})

	t.Run("Non nil rack with nil dc", func(t *testing.T) {
		var dcTemplate *StargateDatacenterTemplate = nil
		rackTemplate := &StargateRackTemplate{
			StargateTemplate: StargateTemplate{HeapSize: &quantity512Mi},
		}
		actual := rackTemplate.Coalesce(dcTemplate)
		assert.NotNil(t, actual)
		assert.Equal(t, &rackTemplate.StargateTemplate, actual)
	})

	t.Run("Non nil rack with non nil dc", func(t *testing.T) {
		dcTemplate := &StargateDatacenterTemplate{
			StargateClusterTemplate: StargateClusterTemplate{
				StargateTemplate: StargateTemplate{HeapSize: &quantity256Mi},
			},
		}
		rackTemplate := &StargateRackTemplate{
			StargateTemplate: StargateTemplate{HeapSize: &quantity512Mi},
		}
		actual := rackTemplate.Coalesce(dcTemplate)
		assert.NotNil(t, actual)
		assert.Equal(t, &rackTemplate.StargateTemplate, actual)
	})
}
