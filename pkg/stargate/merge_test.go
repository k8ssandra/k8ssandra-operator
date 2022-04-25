package stargate

import (
	"fmt"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"testing"
)

var (
	quantity256Mi = resource.MustParse("256Mi")
	quantity512Mi = resource.MustParse("512Mi")
)

func TestMergeStargateClusterTemplates(t *testing.T) {
	tests := []struct {
		name    string
		src     *api.StargateClusterTemplate
		dest    *api.StargateClusterTemplate
		want    *api.StargateClusterTemplate
		wantErr assert.ErrorAssertionFunc
	}{
		{
			"nil src nil dest",
			nil,
			nil,
			nil,
			assert.NoError,
		},
		{
			"non-nil src nil dest",
			&api.StargateClusterTemplate{Size: 10},
			nil,
			&api.StargateClusterTemplate{Size: 10},
			assert.NoError,
		},
		{
			"nil src non-nil dest",
			nil,
			&api.StargateClusterTemplate{Size: 10},
			&api.StargateClusterTemplate{Size: 10},
			assert.NoError,
		},
		{
			"non-nil src non-nil dest",
			&api.StargateClusterTemplate{
				Size: 10,
				StargateTemplate: api.StargateTemplate{
					ContainerImage: &images.Image{
						Registry: "reg1",
						Name:     "img1",
					},
					HeapSize:     &quantity512Mi,
					NodeSelector: map[string]string{"k1": "v1", "k2": "v2"},
					Tolerations:  []corev1.Toleration{{Key: "k1", Value: "v1"}},
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
			&api.StargateClusterTemplate{
				Size: 20,
				StargateTemplate: api.StargateTemplate{
					ContainerImage: &images.Image{
						Repository: "repo2",
						Name:       "img2",
					},
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
			&api.StargateClusterTemplate{
				Size: 20,
				StargateTemplate: api.StargateTemplate{
					ContainerImage: &images.Image{
						Registry:   "reg1",
						Repository: "repo2",
						Name:       "img2",
					},
					HeapSize:       &quantity256Mi,
					ServiceAccount: pointer.String("sa2"),
					// map will be merged
					NodeSelector: map[string]string{"k1": "v1", "k2": "v2a", "k3": "v3"},
					// slice will not be merged, slice1 will be kept intact
					Tolerations: []corev1.Toleration{{Key: "k2", Value: "v2"}},
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
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := MergeStargateClusterTemplates(tt.src, tt.dest)
			assert.Equal(t, tt.want, actual)
			tt.wantErr(t, err)
		})
	}
}

func TestMergeStargateTemplates(t *testing.T) {
	tests := []struct {
		name    string
		src     *api.StargateTemplate
		dest    *api.StargateTemplate
		want    *api.StargateTemplate
		wantErr assert.ErrorAssertionFunc
	}{
		{
			"nil src nil dest",
			nil,
			nil,
			nil,
			assert.NoError,
		},
		{
			"non-nil src nil dest",
			&api.StargateTemplate{HeapSize: &quantity256Mi},
			nil,
			&api.StargateTemplate{HeapSize: &quantity256Mi},
			assert.NoError,
		},
		{
			"nil src non-nil dest",
			nil,
			&api.StargateTemplate{HeapSize: &quantity512Mi},
			&api.StargateTemplate{HeapSize: &quantity512Mi},
			assert.NoError,
		},
		{
			"non-nil src non-nil dest",
			&api.StargateTemplate{
				ContainerImage: &images.Image{
					Repository: "repo1",
					Name:       "img1",
				},
				ServiceAccount: pointer.String("sa1"),
				HeapSize:       &quantity256Mi,
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
					PodAntiAffinity: &corev1.PodAntiAffinity{
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
			&api.StargateTemplate{
				ContainerImage: &images.Image{
					Registry: "reg2",
					Name:     "img2",
				},
				HeapSize:     &quantity512Mi,
				NodeSelector: map[string]string{"k2": "v2a", "k3": "v3"},
				Tolerations:  []corev1.Toleration{{Key: "k2", Value: "v2"}},
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
				},
			},
			&api.StargateTemplate{
				ContainerImage: &images.Image{
					Registry:   "reg2",
					Repository: "repo1",
					Name:       "img2",
				},
				HeapSize:       &quantity512Mi,
				ServiceAccount: pointer.String("sa1"),
				// map will be merged
				NodeSelector: map[string]string{"k1": "v1", "k2": "v2a", "k3": "v3"},
				// slice will not be merged, slice1 will be kept intact
				Tolerations: []corev1.Toleration{{Key: "k2", Value: "v2"}},
				Affinity: &corev1.Affinity{
					PodAffinity: &corev1.PodAffinity{
						// slice will not be merged, slice1 will be kept intact
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
										{Key: "k1", Operator: metav1.LabelSelectorOpIn, Values: []string{"v1"}},
									},
								},
								TopologyKey: "k1",
							},
						},
					},
				},
			},
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := MergeStargateTemplates(tt.src, tt.dest)
			assert.Equal(t, tt.want, actual)
			tt.wantErr(t, err)
		})
	}
}

func ExampleMergeStargateTemplates() {

	{
		src := &api.StargateTemplate{
			NodeSelector: map[string]string{"k1": "v1", "k2": "v2"},
		}
		dest := &api.StargateTemplate{
			NodeSelector: map[string]string{"k2": "v2-bis", "k3": "v3"},
		}
		merged, _ := MergeStargateTemplates(src, dest)
		fmt.Printf("%v\n", merged.NodeSelector)
	}
	{
		src := &api.StargateTemplate{
			ContainerImage: &images.Image{
				Registry:   "reg1",
				Repository: "repo1",
				Name:       "img1",
			},
		}
		dest := &api.StargateTemplate{
			ContainerImage: &images.Image{
				Name: "img2",
				Tag:  "latest",
			},
		}
		merged, _ := MergeStargateTemplates(src, dest)
		fmt.Printf("%v\n", merged.ContainerImage)
	}
	{
		src := &api.StargateTemplate{
			ServiceAccount: pointer.String("sa1"),
		}
		dest := &api.StargateTemplate{
			ServiceAccount: pointer.String("sa2"),
		}
		merged, _ := MergeStargateTemplates(src, dest)
		fmt.Printf("%v\n", *merged.ServiceAccount)
	}
	{
		src := &api.StargateTemplate{
			HeapSize: resource.NewQuantity(123, resource.BinarySI),
		}
		dest := &api.StargateTemplate{
			HeapSize: resource.NewQuantity(456, resource.DecimalSI),
		}
		merged, _ := MergeStargateTemplates(src, dest)
		fmt.Printf("%+v\n", *merged.HeapSize)
	}
	{
		src := &api.StargateTemplate{
			Tolerations: []corev1.Toleration{{
				Key:      "k1",
				Operator: corev1.TolerationOpExists,
			}},
		}
		dest := &api.StargateTemplate{
			Tolerations: []corev1.Toleration{{
				Key:      "k2",
				Operator: corev1.TolerationOpEqual,
			}},
		}
		merged, _ := MergeStargateTemplates(src, dest)
		fmt.Printf("%+v\n", merged.Tolerations)
	}
	// output:
	// map[k1:v1 k2:v2-bis k3:v3]
	// reg1/repo1/img2:latest
	// sa2
	// {i:{value:456 scale:0} d:{Dec:<nil>} s: Format:DecimalSI}
	// [{Key:k2 Operator:Equal Value: Effect: TolerationSeconds:<nil>}]
}
