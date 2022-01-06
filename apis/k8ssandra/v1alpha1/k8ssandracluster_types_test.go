package v1alpha1

import (
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"testing"

	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestK8ssandraCluster(t *testing.T) {
	t.Run("HasStargates", testK8ssandraClusterHasStargates)
	t.Run("HasReapers", testK8ssandraClusterHasReapers)
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

func testK8ssandraClusterHasReapers(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var kc *K8ssandraCluster = nil
		assert.False(t, kc.HasReapers())
	})
	t.Run("no reapers", func(t *testing.T) {
		kc := K8ssandraCluster{}
		assert.False(t, kc.HasReapers())
	})
	t.Run("cluster-level reaper", func(t *testing.T) {
		kc := K8ssandraCluster{
			Spec: K8ssandraClusterSpec{
				Reaper: &reaperapi.ReaperClusterTemplate{
					Keyspace: "reaper",
				},
			},
		}
		assert.True(t, kc.HasReapers())
	})
	t.Run("dc-level reaper", func(t *testing.T) {
		kc := K8ssandraCluster{
			Spec: K8ssandraClusterSpec{
				Cassandra: &CassandraClusterTemplate{
					Datacenters: []CassandraDatacenterTemplate{
						{
							Size:   3,
							Reaper: nil,
						},
						{
							Size: 3,
							Reaper: &reaperapi.ReaperDatacenterTemplate{
								ServiceAccountName: "reaper_sa",
							},
						},
					},
				},
			},
		}
		assert.True(t, kc.HasReapers())
	})
}
