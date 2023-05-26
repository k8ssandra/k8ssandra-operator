package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
	t.Run("MergeWith", testStargateDatacenterTemplateMerge)
}

func TestStargateRackTemplate(t *testing.T) {
	t.Run("MergeWith", testStargateRackTemplateMerge)
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

func testStargateDatacenterTemplateMerge(t *testing.T) {
	tests := []struct {
		name            string
		dcTemplate      *StargateDatacenterTemplate
		clusterTemplate *StargateClusterTemplate
		want            *StargateDatacenterTemplate
	}{
		{
			"Nil dc with nil cluster",
			nil,
			nil,
			nil,
		},
		{
			"Nil dc with non-nil cluster",
			nil,
			&StargateClusterTemplate{Size: 10},
			&StargateDatacenterTemplate{StargateClusterTemplate: StargateClusterTemplate{Size: 10}},
		},
		{
			"Non-nil dc with nil cluster",
			&StargateDatacenterTemplate{
				StargateClusterTemplate: StargateClusterTemplate{Size: 10},
			},
			nil,
			&StargateDatacenterTemplate{
				StargateClusterTemplate: StargateClusterTemplate{Size: 10},
			},
		},
		{
			"Non-nil dc with non-nil cluster",
			&StargateDatacenterTemplate{
				StargateClusterTemplate: StargateClusterTemplate{
					StargateTemplate: StargateTemplate{
						ContainerImage: "reg1/repo2/img1",
						HeapSize:       &quantity512Mi,
						NodeSelector:   map[string]string{"k1": "v1", "k2": "v2"},
						Tolerations:    []corev1.Toleration{{Key: "k1", Value: "v1"}},
						Affinity: &corev1.Affinity{
							PodAffinity: &corev1.PodAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "k1", Operator: metav1.LabelSelectorOpIn, Values: []string{"v1"}},
											},
										},
										TopologyKey: "k1",
									},
								},
							},
						},
					},
					Size: 10,
				},
				Racks: []StargateRackTemplate{{
					Name: "rack1",
				}},
			},
			&StargateClusterTemplate{
				StargateTemplate: StargateTemplate{
					ContainerImage: "reg1/img2",
					ServiceAccount: pointer.String("sa2"),
					HeapSize:       &quantity256Mi,
					NodeSelector:   map[string]string{"k2": "v2a", "k3": "v3"},
					Tolerations:    []corev1.Toleration{{Key: "k2", Value: "v2"}},
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{Key: "k2", Operator: metav1.LabelSelectorOpIn, Values: []string{"v2"}},
										},
									},
									TopologyKey: "k2",
								},
							},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{Key: "k2", Operator: metav1.LabelSelectorOpIn, Values: []string{"v2"}},
										},
									},
									TopologyKey: "k2",
								},
							},
						},
					},
				},
				Size: 20,
			},
			&StargateDatacenterTemplate{
				StargateClusterTemplate: StargateClusterTemplate{
					StargateTemplate: StargateTemplate{
						ContainerImage: "reg1/repo2/img1",
						HeapSize:       &quantity512Mi,
						ServiceAccount: pointer.String("sa2"),
						// map will be merged
						NodeSelector: map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"},
						// slice will not be merged, slice1 will be kept intact
						Tolerations: []corev1.Toleration{{Key: "k1", Value: "v1"}},
						Affinity: &corev1.Affinity{
							PodAffinity: &corev1.PodAffinity{
								// slice will not be merged, slice1 will be kept intact
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "k1", Operator: metav1.LabelSelectorOpIn, Values: []string{"v1"}},
											},
										},
										TopologyKey: "k1",
									},
								},
							},
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "k2", Operator: metav1.LabelSelectorOpIn, Values: []string{"v2"}},
											},
										},
										TopologyKey: "k2",
									},
								},
							},
						},
					},
					Size: 10,
				},
				Racks: []StargateRackTemplate{{
					Name: "rack1",
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.dcTemplate.MergeWith(tt.clusterTemplate)
			assert.Equal(t, tt.want, actual)
		})
	}
}

func testStargateRackTemplateMerge(t *testing.T) {
	tests := []struct {
		name         string
		rackTemplate *StargateRackTemplate
		dcTemplate   *StargateDatacenterTemplate
		want         *StargateTemplate
	}{
		{
			"Nil rack with nil dc",
			nil,
			nil,
			nil,
		},
		{
			"Nil rack with non nil dc",
			nil,
			&StargateDatacenterTemplate{
				StargateClusterTemplate: StargateClusterTemplate{
					StargateTemplate: StargateTemplate{HeapSize: &quantity256Mi},
				},
			},
			&StargateTemplate{HeapSize: &quantity256Mi},
		},
		{
			"Non nil rack with nil dc",
			&StargateRackTemplate{
				StargateTemplate: StargateTemplate{HeapSize: &quantity512Mi},
			},
			nil,
			&StargateTemplate{HeapSize: &quantity512Mi},
		},
		{
			"Non nil rack with non nil dc",
			&StargateRackTemplate{
				StargateTemplate: StargateTemplate{
					ContainerImage: "reg1/img1",
					HeapSize:       &quantity512Mi,
					NodeSelector:   map[string]string{"k1": "v1", "k2": "v2"},
					Tolerations:    []corev1.Toleration{{Key: "k1", Value: "v1"}},
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{Key: "k1", Operator: metav1.LabelSelectorOpIn, Values: []string{"v1"}},
										},
									},
									TopologyKey: "k1",
								},
							},
						},
					},
				},
			},
			&StargateDatacenterTemplate{
				StargateClusterTemplate: StargateClusterTemplate{
					StargateTemplate: StargateTemplate{
						ContainerImage: "repo2/img2",
						ServiceAccount: pointer.String("sa2"),
						HeapSize:       &quantity256Mi,
						NodeSelector:   map[string]string{"k2": "v2a", "k3": "v3"},
						Tolerations:    []corev1.Toleration{{Key: "k2", Value: "v2"}},
						Affinity: &corev1.Affinity{
							PodAffinity: &corev1.PodAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "k2", Operator: metav1.LabelSelectorOpIn, Values: []string{"v2"}},
											},
										},
										TopologyKey: "k2",
									},
								},
							},
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{Key: "k2", Operator: metav1.LabelSelectorOpIn, Values: []string{"v2"}},
											},
										},
										TopologyKey: "k2",
									},
								},
							},
						},
					},
				},
			},
			&StargateTemplate{
				ContainerImage: "reg1/img1",
				HeapSize:       &quantity512Mi,
				ServiceAccount: pointer.String("sa2"),
				// map will be merged
				NodeSelector: map[string]string{"k1": "v1", "k2": "v2", "k3": "v3"},
				// slice will not be merged, slice1 will be kept intact
				Tolerations: []corev1.Toleration{{Key: "k1", Value: "v1"}},
				Affinity: &corev1.Affinity{
					PodAffinity: &corev1.PodAffinity{
						// slice will not be merged, slice1 will be kept intact
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{Key: "k1", Operator: metav1.LabelSelectorOpIn, Values: []string{"v1"}},
									},
								},
								TopologyKey: "k1",
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{Key: "k2", Operator: metav1.LabelSelectorOpIn, Values: []string{"v2"}},
									},
								},
								TopologyKey: "k2",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.rackTemplate.MergeWith(tt.dcTemplate)
			assert.Equal(t, tt.want, actual)
		})
	}
}
