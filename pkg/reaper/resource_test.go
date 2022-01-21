package reaper

import (
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_computeReaperDcAvailability(t *testing.T) {
	tests := []struct {
		name string
		kc   *k8ssandraapi.K8ssandraCluster
		want string
	}{
		{
			"cluster-level reaper, single dc",
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
			"cluster-level reaper, multi dc",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}},
							{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc2"}},
						},
					},
					Reaper: &reaperapi.ReaperClusterTemplate{},
				},
			},
			DatacenterAvailabilityEach,
		},
		{
			"dc-level reaper, single dc",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{
								Meta:   k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"},
								Reaper: &reaperapi.ReaperDatacenterTemplate{},
							},
						},
					},
				},
			},
			DatacenterAvailabilityAll,
		},
		{
			"dc-level reaper, multi dc, single reaper",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{
								Meta:   k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"},
								Reaper: &reaperapi.ReaperDatacenterTemplate{},
							},
							{
								Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc2"},
							},
						},
					},
				},
			},
			DatacenterAvailabilityAll,
		},
		{
			"dc-level reaper, multi dc, some reapers",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{
								Meta:   k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"},
								Reaper: &reaperapi.ReaperDatacenterTemplate{},
							},
							{
								Meta:   k8ssandraapi.EmbeddedObjectMeta{Name: "dc2"},
								Reaper: &reaperapi.ReaperDatacenterTemplate{},
							},
							{
								Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc3"},
							},
						},
					},
				},
			},
			DatacenterAvailabilityAll,
		},
		{
			"dc-level reaper, multi dc, all reapers",
			&k8ssandraapi.K8ssandraCluster{
				Spec: k8ssandraapi.K8ssandraClusterSpec{
					Cassandra: &k8ssandraapi.CassandraClusterTemplate{
						Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
							{
								Meta:   k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"},
								Reaper: &reaperapi.ReaperDatacenterTemplate{},
							},
							{
								Meta:   k8ssandraapi.EmbeddedObjectMeta{Name: "dc2"},
								Reaper: &reaperapi.ReaperDatacenterTemplate{},
							},
							{
								Meta:   k8ssandraapi.EmbeddedObjectMeta{Name: "dc3"},
								Reaper: &reaperapi.ReaperDatacenterTemplate{},
							},
						},
					},
				},
			},
			DatacenterAvailabilityEach,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeReaperDcAvailability(tt.kc)
			assert.Equal(t, tt.want, got)
		})
	}
}
