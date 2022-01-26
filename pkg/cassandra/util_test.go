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
			tc.got = ComputeInitialSystemReplication(tc.kluster)
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
					ObjectMeta: metav1.ObjectMeta{Name: "dc2"},
					Spec:       cassdcapi.CassandraDatacenterSpec{Size: 1},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "dc3"},
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

func TestComputeReplicationFromDcTemplates(t *testing.T) {
	tests := []struct {
		name     string
		dcs      []api.CassandraDatacenterTemplate
		expected map[string]int
	}{
		{"one dc", []api.CassandraDatacenterTemplate{
			{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Size: 3},
		}, map[string]int{"dc1": 3}},
		{"small dc", []api.CassandraDatacenterTemplate{
			{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Size: 1},
		}, map[string]int{"dc1": 1}},
		{"large dc", []api.CassandraDatacenterTemplate{
			{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Size: 10},
		}, map[string]int{"dc1": 3}},
		{"many dcs", []api.CassandraDatacenterTemplate{
			{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Size: 3},
			{Meta: api.EmbeddedObjectMeta{Name: "dc2"}, Size: 1},
			{Meta: api.EmbeddedObjectMeta{Name: "dc3"}, Size: 10},
		}, map[string]int{"dc1": 3, "dc2": 1, "dc3": 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ComputeReplicationFromDcTemplates(3, tt.dcs...)
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
		{"wrong length", map[string]string{"class": NetworkTopology, "dc1": "3", "dc2": "3"}, map[string]int{"dc1": 3}, false},
		{"missing dc", map[string]string{"class": NetworkTopology, "dc2": "3"}, map[string]int{"dc1": 3}, false},
		{"invalid rf", map[string]string{"class": NetworkTopology, "dc1": "not a number"}, map[string]int{"dc1": 3}, false},
		{"wrong rf", map[string]string{"class": NetworkTopology, "dc1": "1"}, map[string]int{"dc1": 3}, false},
		{"success", map[string]string{"class": NetworkTopology, "dc1": "1", "dc2": "3"}, map[string]int{"dc1": 1, "dc2": 3}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareReplications(tt.actual, tt.desired)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseReplication(t *testing.T) {
	tests := []struct {
		name        string
		replication []byte
		want        *Replication
		valid       bool
	}{
		{
			name:        "valid replication - single DC",
			replication: []byte(`{"dc2": {"ks1": 3, "ks2": 3}}`),
			want: &Replication{
				datacenters: map[string]keyspacesReplication{
					"dc2": {
						"ks1": 3,
						"ks2": 3,
					},
				},
			},
			valid: true,
		},
		{
			name:        "valid replication - multiple DCs",
			replication: []byte(`{"dc2": {"ks1": 3, "ks2": 3}, "dc3": {"ks1": 5, "ks2": 1}}`),
			want: &Replication{
				datacenters: map[string]keyspacesReplication{
					"dc2": {
						"ks1": 3,
						"ks2": 3,
					},
					"dc3": {
						"ks1": 5,
						"ks2": 1,
					},
				},
			},
			valid: true,
		},
		{
			name:        "invalid replication - wrong type",
			replication: []byte(`{"dc2": {"ks1": 3, "ks2": 3}, "dc3": {"ks1": 5, "ks2": "1"}}`),
			want:        nil,
			valid:       false,
		},
		{
			name:        "invalid replication - replica count < 0",
			replication: []byte(`{"dc2": {"ks1": 3, "ks2": 3}, "dc3": {"ks1": 5, "ks2": -1}}`),
			want:        nil,
			valid:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReplication(tt.replication)
			if tt.valid {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestReplicationFactor(t *testing.T) {
	replication, err := ParseReplication([]byte(`{"dc2": {"ks1": 3, "ks2": 5}}`))
	require.NoError(t, err)

	assert.Equal(t, 3, replication.ReplicationFactor("dc2", "ks1"))
	assert.Equal(t, 0, replication.ReplicationFactor("dc2", "ks3"))
	assert.Equal(t, 0, replication.ReplicationFactor("dc3", "ks1"))
}
