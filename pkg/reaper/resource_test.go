package reaper

import (
	"testing"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	k8ssandrameta "github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/stretchr/testify/assert"
)

func Test_computeReaperDcAvailability(t *testing.T) {
	tests := []struct {
		name string
		kc   *k8ssandraapi.K8ssandraCluster
		want string
	}{
		{
			"single dc",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}},
						},
					},
					Reaper: &reaperapi.ReaperClusterTemplate{},
				},
			},
			DatacenterAvailabilityAll,
		},
		{
			"multi dc deployment per dc",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}},
							{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc2"}},
						},
					},
					Reaper: &reaperapi.ReaperClusterTemplate{
						DeploymentMode: reaperapi.DeploymentModePerDc,
					},
				},
			},
			DatacenterAvailabilityEach,
		},
		{
			"multi dc deployment single",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}},
							{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc2"}},
						},
					},
					Reaper: &reaperapi.ReaperClusterTemplate{
						DeploymentMode: reaperapi.DeploymentModeSingle,
					},
				},
			},
			DatacenterAvailabilityAll,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeReaperDcAvailability(tt.kc)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_transformCassandraClusterMeta(t *testing.T) {
	kc := &k8ssandraapi.K8ssandraCluster{
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				Meta: k8ssandrameta.CassandraClusterMeta{
					CommonLabels:      map[string]string{"testLabel": "testValue"},
					CommonAnnotations: map[string]string{"testAnnotation": "testValue"},
				},
			},
			Reaper: &reaperapi.ReaperClusterTemplate{},
		},
	}

	got := transformCassandraClusterMeta(kc)
	expected := &k8ssandrameta.ResourceMeta{
		CommonLabels: map[string]string{"testLabel": "testValue"},
		Pods: k8ssandrameta.Tags{
			Labels:      nil,
			Annotations: nil,
		},
		Service: k8ssandrameta.Tags{
			Labels:      map[string]string{"testLabel": "testValue"},
			Annotations: map[string]string{"testAnnotation": "testValue"},
		},
	}
	assert.Equal(t, expected, got)
}

func Test_transformCassandraClusterNoMeta(t *testing.T) {
	kc := &k8ssandraapi.K8ssandraCluster{
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{},
			Reaper:    &reaperapi.ReaperClusterTemplate{},
		},
	}

	got := transformCassandraClusterMeta(kc)
	expected := &k8ssandrameta.ResourceMeta{
		CommonLabels: nil,
		Pods: k8ssandrameta.Tags{
			Labels:      nil,
			Annotations: nil,
		},
		Service: k8ssandrameta.Tags{
			Labels:      nil,
			Annotations: nil,
		},
	}
	assert.Equal(t, expected, got)
}
