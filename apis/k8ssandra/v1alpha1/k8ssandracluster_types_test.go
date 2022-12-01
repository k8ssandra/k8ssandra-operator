package v1alpha1

import (
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
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
				HostNetwork: pointer.Bool(true),
			},
			&cassdcapi.NetworkingConfig{
				HostNetwork: true,
			},
		},
		{
			"host network false",
			&NetworkingConfig{
				HostNetwork: pointer.Bool(false),
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
				HostNetwork: pointer.Bool(true),
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
