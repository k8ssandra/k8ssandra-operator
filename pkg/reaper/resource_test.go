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
						DeploymentMode: DeploymentModePerDc,
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
						DeploymentMode: DeploymentModeSingle,
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
