package v1alpha1

import (
	"testing"

	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/stretchr/testify/assert"
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
					Cluster: "cluster1",
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
