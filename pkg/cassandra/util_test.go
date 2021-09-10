package cassandra

import (
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"reflect"
	"testing"
)

func TestComputeSystemReplication(t *testing.T) {
	type test struct {
		name string

		kluster *api.K8ssandraCluster

		want SystemReplication

		got SystemReplication
	}

	tests := []test{
		{
			name: "single-dc",
			kluster: &api.K8ssandraCluster{
				Spec: api.K8ssandraClusterSpec{
					Cassandra: &api.CassandraClusterTemplate{
						Datacenters: []api.CassandraDatacenterTemplate{
							{
								Meta: api.EmbeddedObjectMeta{Name: "dc1"},
								Size: 6,
							},
						},
					},
				},
			},
			want: SystemReplication{
				Datacenters:       []string{"dc1"},
				ReplicationFactor: 3,
			},
		},
		{
			name: "multi-dc with same size",
			kluster: &api.K8ssandraCluster{
				Spec: api.K8ssandraClusterSpec{
					Cassandra: &api.CassandraClusterTemplate{
						Datacenters: []api.CassandraDatacenterTemplate{
							{
								Meta: api.EmbeddedObjectMeta{Name: "dc1"},
								Size: 3,
							},
							{
								Meta: api.EmbeddedObjectMeta{Name: "dc2"},
								Size: 3,
							},
							{
								Meta: api.EmbeddedObjectMeta{Name: "dc3"},
								Size: 3,
							},
						},
					},
				},
			},
			want: SystemReplication{
				Datacenters:       []string{"dc1", "dc2", "dc3"},
				ReplicationFactor: 3,
			},
		},
		{
			name: "multi-dc with different sizes",
			kluster: &api.K8ssandraCluster{
				Spec: api.K8ssandraClusterSpec{
					Cassandra: &api.CassandraClusterTemplate{
						Datacenters: []api.CassandraDatacenterTemplate{
							{
								Meta: api.EmbeddedObjectMeta{Name: "dc1"},
								Size: 6,
							},
							{
								Meta: api.EmbeddedObjectMeta{Name: "dc2"},
								Size: 3,
							},
							{
								Meta: api.EmbeddedObjectMeta{Name: "dc3"},
								Size: 1,
							},
						},
					},
				},
			},
			want: SystemReplication{
				Datacenters:       []string{"dc1", "dc2", "dc3"},
				ReplicationFactor: 1,
			},
		},
	}

	for _, tc := range tests {
		tc.got = ComputeSystemReplication(tc.kluster)
		if !reflect.DeepEqual(tc.want, tc.got) {
			t.Errorf("%s - expected: %+v, got: %+v", tc.name, tc.want, tc.got)
		}
	}
}
