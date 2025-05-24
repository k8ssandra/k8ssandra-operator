package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK8ssandraCluster(t *testing.T) {
	t.Run("HasStargates", testK8ssandraClusterHasStargates)
}

func testK8ssandraClusterHasStargates(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var kc *K8ssandraCluster = nil
		assert.False(t, kc.HasStargates())
	})
	t.Run("no stargates", func(t *testing.T) {
		kc := K8ssandraCluster{}
		assert.False(t, kc.HasStargates())
	})
	t.Run("cluster-level stargate", func(t *testing.T) {
		kc := K8ssandraCluster{
			Spec: K8ssandraClusterSpec{
				Stargate: &stargateapi.StargateClusterTemplate{
					Size: 3,
				},
			},
		}
		assert.True(t, kc.HasStargates())
	})
	t.Run("dc-level stargate", func(t *testing.T) {
		kc := K8ssandraCluster{
			Spec: K8ssandraClusterSpec{
				Cassandra: &CassandraClusterTemplate{
					Datacenters: []CassandraDatacenterTemplate{
						{
							Size:     3,
							Stargate: nil,
						},
						{
							Size: 3,
							Stargate: &stargateapi.StargateDatacenterTemplate{
								StargateClusterTemplate: stargateapi.StargateClusterTemplate{
									Size: 3,
								},
							},
						},
					},
				},
			},
		}
		assert.True(t, kc.HasStargates())
	})
}

func TestNetworkingConfig_ToCassNetworkingConfig(t *testing.T) {
	tests := []struct {
		name string
		in   *NetworkingConfig
		want *cassdcapi.NetworkingConfig
	}{
		{
			"nil",
			nil,
			nil,
		},
		{
			"empty",
			&NetworkingConfig{},
			&cassdcapi.NetworkingConfig{},
		},
		{
			"host network true",
			&NetworkingConfig{
				HostNetwork: ptr.To(true),
			},
			&cassdcapi.NetworkingConfig{
				HostNetwork: true,
			},
		},
		{
			"host network false",
			&NetworkingConfig{
				HostNetwork: ptr.To(false),
			},
			&cassdcapi.NetworkingConfig{
				HostNetwork: false,
			},
		},
		{
			"host network nil",
			&NetworkingConfig{
				HostNetwork: nil,
			},
			&cassdcapi.NetworkingConfig{
				HostNetwork: false,
			},
		},
		{
			"all set",
			&NetworkingConfig{
				HostNetwork: ptr.To(true),
				NodePort: &cassdcapi.NodePortConfig{
					Native:       1,
					NativeSSL:    2,
					Internode:    3,
					InternodeSSL: 4,
				},
			},
			&cassdcapi.NetworkingConfig{
				HostNetwork: true,
				NodePort: &cassdcapi.NodePortConfig{
					Native:       1,
					NativeSSL:    2,
					Internode:    3,
					InternodeSSL: 4,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.ToCassNetworkingConfig())
		})
	}
}

// TestUnmarshallDatacenterMeta checks that user-provided labels and annotations are properly set when unmarshalling
// YAML.
func TestUnmarshallDatacenterMeta(t *testing.T) {
	input := `
metadata:
  labels:
    label1: labelValue1
  annotations:
    annotation1: annotationValue1
  commonLabels:
    commonLabel1: commonLabelValue1
  pods:
    labels:
      podLabel1: podLabelValue1
    annotations:
      podAnnotation1: podAnnotationValue1
  services:
    dcService:
      labels:
        dcSvcLabel1: dcSvcValue1
      annotations:
        dcSvcAnnotation1: dcSvcAnnotationValue1
    seedService:
      labels:
        seedSvcLabel1: seedSvcLabelValue1
      annotations:
        seedSvcAnnotation1: seedSvcAnnotationValue1
    allPodsService:
      labels:
        allPodsSvcLabel1: allPodsSvcLabelValue1
      annotations:
        allPodsSvcAnnotation1: allPodsSvcAnnotationValue1
    additionalSeedService:
      labels:
        addSeedSvcLabel1: addSeedSvcLabelValue1
      annotations:
        addSeedSvcAnnotation1: addSeedSvcAnnotationValue1
    nodePortService:
      labels:
        nodePortSvcLabel1: nodePortSvcLabelValue1
      annotations:
        nodePortSvcAnnotation1: nodePortSvcAnnotationValue1
`
	dc := &CassandraDatacenterTemplate{}
	err := yaml.Unmarshal([]byte(input), dc)
	assert.NoError(t, err)

	assert.Equal(t, "labelValue1", dc.Meta.Labels["label1"])
	assert.Equal(t, "annotationValue1", dc.Meta.Annotations["annotation1"])
	assert.Equal(t, "commonLabelValue1", dc.Meta.CommonLabels["commonLabel1"])
	assert.Equal(t, "podLabelValue1", dc.Meta.Pods.Labels["podLabel1"])
	assert.Equal(t, "podAnnotationValue1", dc.Meta.Pods.Annotations["podAnnotation1"])
	assert.Equal(t, "dcSvcValue1", dc.Meta.ServiceConfig.DatacenterService.Labels["dcSvcLabel1"])
	assert.Equal(t, "dcSvcAnnotationValue1", dc.Meta.ServiceConfig.DatacenterService.Annotations["dcSvcAnnotation1"])
	assert.Equal(t, "seedSvcLabelValue1", dc.Meta.ServiceConfig.SeedService.Labels["seedSvcLabel1"])
	assert.Equal(t, "seedSvcAnnotationValue1", dc.Meta.ServiceConfig.SeedService.Annotations["seedSvcAnnotation1"])
	assert.Equal(t, "allPodsSvcLabelValue1", dc.Meta.ServiceConfig.AllPodsService.Labels["allPodsSvcLabel1"])
	assert.Equal(t, "allPodsSvcAnnotationValue1", dc.Meta.ServiceConfig.AllPodsService.Annotations["allPodsSvcAnnotation1"])
	assert.Equal(t, "addSeedSvcLabelValue1", dc.Meta.ServiceConfig.AdditionalSeedService.Labels["addSeedSvcLabel1"])
	assert.Equal(t, "addSeedSvcAnnotationValue1", dc.Meta.ServiceConfig.AdditionalSeedService.Annotations["addSeedSvcAnnotation1"])
	assert.Equal(t, "nodePortSvcLabelValue1", dc.Meta.ServiceConfig.NodePortService.Labels["nodePortSvcLabel1"])
	assert.Equal(t, "nodePortSvcAnnotationValue1", dc.Meta.ServiceConfig.NodePortService.Annotations["nodePortSvcAnnotation1"])
}

func TestGenerationChanged(t *testing.T) {
	assert := assert.New(t)
	kc := &K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 2,
		},
		Spec: K8ssandraClusterSpec{},
	}

	kc.Status = K8ssandraClusterStatus{
		ObservedGeneration: 0,
	}

	assert.True(kc.GenerationChanged())
	kc.Status.ObservedGeneration = 2
	assert.False(kc.GenerationChanged())
	kc.ObjectMeta.Generation = 3
	assert.True(kc.GenerationChanged())
}

func TestDcRemoved(t *testing.T) {
	kcOld := createClusterObjWithCassandraConfig("testcluster", "testns")
	kcNew := kcOld.DeepCopy()
	require.False(t, DcRemoved(kcOld.Spec, kcNew.Spec))
	kcOld.Spec.Cassandra.Datacenters = append(kcOld.Spec.Cassandra.Datacenters, CassandraDatacenterTemplate{
		Meta: EmbeddedObjectMeta{
			Name: "dc2",
		},
	})
	require.True(t, DcRemoved(kcOld.Spec, kcNew.Spec))
	kcOld = createClusterObjWithCassandraConfig("testcluster", "testns")
	kcNew = kcOld.DeepCopy()
	kcNew.Spec.Cassandra.Datacenters[0].Meta.Name = "newName"
	require.True(t, DcRemoved(kcOld.Spec, kcNew.Spec))
}

func TestDcAdded(t *testing.T) {
	kcOld := createClusterObjWithCassandraConfig("testcluster", "testns")
	kcNew := kcOld.DeepCopy()
	require.False(t, DcAdded(kcOld.Spec, kcNew.Spec))
	kcNew.Spec.Cassandra.Datacenters = append(kcOld.Spec.Cassandra.Datacenters, CassandraDatacenterTemplate{
		Meta: EmbeddedObjectMeta{
			Name: "dc2",
		},
	})
	require.True(t, DcAdded(kcOld.Spec, kcNew.Spec))

	kcOld = createClusterObjWithCassandraConfig("testcluster", "testns")
	kcNew = kcOld.DeepCopy()
	kcNew.Spec.Cassandra.Datacenters[0].Meta.Name = "newName"
	require.True(t, DcAdded(kcOld.Spec, kcNew.Spec))
}
