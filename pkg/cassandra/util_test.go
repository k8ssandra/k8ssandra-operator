package cassandra

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/require"
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
		t.Run(tc.name, func(t *testing.T) {
			tc.got = ComputeSystemReplication(tc.kluster)
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func TestComputeReplication(t *testing.T) {
	tests := []struct {
		name     string
		dcs      []*cassdcapi.CassandraDatacenter
		expected map[string]int
	}{
		{
			"one dc",
			[]*cassdcapi.CassandraDatacenter{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
					Spec:       cassdcapi.CassandraDatacenterSpec{Size: 3},
				},
			},
			map[string]int{"dc1": 3},
		},
		{
			"small dc",
			[]*cassdcapi.CassandraDatacenter{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
					Spec:       cassdcapi.CassandraDatacenterSpec{Size: 1},
				},
			},
			map[string]int{"dc1": 1},
		},
		{
			"large dc",
			[]*cassdcapi.CassandraDatacenter{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
					Spec:       cassdcapi.CassandraDatacenterSpec{Size: 10},
				},
			},
			map[string]int{"dc1": 3},
		},
		{
			"many dcs",
			[]*cassdcapi.CassandraDatacenter{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
					Spec:       cassdcapi.CassandraDatacenterSpec{Size: 3},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
					Spec:       cassdcapi.CassandraDatacenterSpec{Size: 1},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "dc1"},
					Spec:       cassdcapi.CassandraDatacenterSpec{Size: 10},
				},
			},
			map[string]int{"dc1": 3, "dc2": 1, "dc3": 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ComputeReplication(3, tt.dcs...)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestCompareReplications(t *testing.T) {
	tests := []struct {
		name     string
		actual   map[string]string
		desired  map[string]int
		expected bool
	}{
		{"nil", nil, map[string]int{"dc1": 3}, false},
		{"empty", map[string]string{}, map[string]int{"dc1": 3}, false},
		{"wrong class", map[string]string{"class": "wrong"}, map[string]int{"dc1": 3}, false},
		{"wrong length", map[string]string{"class": networkTopology, "dc1": "3", "dc2": "3"}, map[string]int{"dc1": 3}, false},
		{"missing dc", map[string]string{"class": networkTopology, "dc2": "3"}, map[string]int{"dc1": 3}, false},
		{"invalid rf", map[string]string{"class": networkTopology, "dc1": "not a number"}, map[string]int{"dc1": 3}, false},
		{"wrong rf", map[string]string{"class": networkTopology, "dc1": "1"}, map[string]int{"dc1": 3}, false},
		{"success", map[string]string{"class": networkTopology, "dc1": "1", "dc2": "3"}, map[string]int{"dc1": 1, "dc2": 3}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareReplications(tt.actual, tt.desired)
			assert.Equal(t, tt.expected, result)
		})
	}
}
