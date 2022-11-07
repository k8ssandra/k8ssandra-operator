package cassandra

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestComputeInitialTokens(t *testing.T) {
	tests := []struct {
		name      string
		dcConfigs []*DatacenterConfig
		want      []map[string][]string
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name: "single dc num_tokens < 16",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens": 1,
						},
					},
				},
			},
			want: []map[string][]string{
				{
					"cluster1-dc1-default-sts-0": {"-9223372036854775808"},
					"cluster1-dc1-default-sts-1": {"-3074457345618258603"},
					"cluster1-dc1-default-sts-2": {"3074457345618258602"},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "single dc num_tokens >= 16",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens": 16,
						},
					},
				},
			},
			want: []map[string][]string{
				nil,
			},
			wantErr: assert.Error,
		},
		{
			name: "multi dc num_tokens < 16",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					Racks:   []cassdcapi.Rack{{Name: "rack1"}, {Name: "rack2"}, {Name: "rack3"}},
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens": 4,
						},
					},
				},
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc2"},
					Size:    10,
					Racks:   []cassdcapi.Rack{{Name: "rack1"}, {Name: "rack2"}, {Name: "rack3"}},
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens": 8,
							"allocate_tokens_for_local_replication_factor": 5,
						},
					},
				},
			},
			want: []map[string][]string{
				// dc1: 3 RF nodes with 4 num_tokens each
				{
					"cluster1-dc1-rack1-sts-0": {"-9223372036854775808", "-4611686018427387905", "-2", "4611686018427387901"},
					"cluster1-dc1-rack2-sts-0": {"-7686143364045646507", "-3074457345618258604", "1537228672809129299", "6148914691236517202"},
					"cluster1-dc1-rack3-sts-0": {"-6148914691236517206", "-1537228672809129303", "3074457345618258600", "7686143364045646503"},
				},
				// dc2: 5 RF nodes with 8 num_tokens each
				{
					"cluster1-dc2-rack1-sts-0": {"-8840700218304418089", "-6534857209090724139", "-4229014199877030189", "-1923171190663336239", "382671818550357711", "2688514827764051661", "4994357836977745611", "7300200846191439561"},
					"cluster1-dc2-rack2-sts-0": {"-8379531616461679299", "-6073688607247985349", "-3767845598034291399", "-1462002588820597449", "843840420393096501", "3149683429606790451", "5455526438820484401", "7761369448034178351"},
					"cluster1-dc2-rack3-sts-0": {"-7918363014618940509", "-5612520005405246559", "-3306676996191552609", "-1000833986977858659", "1305009022235835291", "3610852031449529241", "5916695040663223191", "8222538049876917141"},
					"cluster1-dc2-rack1-sts-1": {"-7457194412776201719", "-5151351403562507769", "-2845508394348813819", "-539665385135119869", "1766177624078574081", "4072020633292268031", "6377863642505961981", "8683706651719655931"},
					"cluster1-dc2-rack2-sts-1": {"-6996025810933462929", "-4690182801719768979", "-2384339792506075029", "-78496783292381079", "2227346225921312871", "4533189235135006821", "6839032244348700771", "9144875253562394737"},
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ComputeInitialTokens(tt.dcConfigs)
			tt.wantErr(t, err)
			for i, dcConfig := range tt.dcConfigs {
				assert.Equal(t, tt.want[i], dcConfig.InitialTokensByPodName)
			}
		})
	}
}

func Test_collectTokenAllocationInfos(t *testing.T) {
	tests := []struct {
		name      string
		dcConfigs []*DatacenterConfig
		wantInfos []*tokenAllocationInfo
	}{
		{
			name: "all num_tokens < 16",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens":  1,
							"partitioner": "Murmur3Partitioner",
						},
					},
				},
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc2"},
					Size:    5,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens":  8,
							"partitioner": "Murmur3Partitioner",
							"allocate_tokens_for_local_replication_factor": 5,
						},
					},
				},
			},
			wantInfos: []*tokenAllocationInfo{
				{
					numTokens:   1,
					rf:          3,
					partitioner: &utils.Murmur3Partitioner,
				},
				{
					numTokens:   8,
					rf:          5,
					partitioner: &utils.Murmur3Partitioner,
				},
			},
		},
		{
			name: "num_tokens partially >= 16",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens": 1,
						},
					},
				},
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc2"},
					Size:    5,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens":  256,
							"partitioner": "Murmur3Partitioner",
							"allocate_tokens_for_local_replication_factor": 5,
						},
					},
				},
			},
			wantInfos: nil,
		},
		{
			name: "bad num_tokens",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens": "not a number",
						},
					},
				},
			},
			wantInfos: nil,
		},
		{
			name: "bad partitioner",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens":  4,
							"partitioner": "unknown",
						},
					},
				},
			},
			wantInfos: nil,
		},
		{
			name: "bad rf",
			dcConfigs: []*DatacenterConfig{
				{
					Cluster: "cluster1",
					Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
					Size:    3,
					CassandraConfig: api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"num_tokens": 4,
							"allocate_tokens_for_local_replication_factor": "not a number",
						},
					},
				},
			},
			wantInfos: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInfos, _ := collectTokenAllocationInfos(tt.dcConfigs)
			assert.Equal(t, tt.wantInfos, gotInfos)
		})
	}
}

func Test_computeDcNumTokens(t *testing.T) {
	tests := []struct {
		name string
		dc   *DatacenterConfig
		want int
	}{
		{
			name: "num_tokens < 16",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"num_tokens": 8,
					},
				},
			},
			want: 8,
		},
		{
			name: "num_tokens >= 16",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"num_tokens": 16,
					},
				},
			},
			want: -1,
		},
		{
			name: "num_tokens invalid",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"num_tokens": "not a number",
					},
				},
			},
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := computeDcNumTokens(tt.dc)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_computeDcPartitioner(t *testing.T) {
	tests := []struct {
		name            string
		dc              *DatacenterConfig
		wantPartitioner *utils.Partitioner
	}{
		{
			name: "default partitioner",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
			},
			wantPartitioner: &utils.Murmur3Partitioner,
		},
		{
			name: "Murmur3 partitioner simple name",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"partitioner": "Murmur3Partitioner",
					},
				},
			},
			wantPartitioner: &utils.Murmur3Partitioner,
		},
		{
			name: "Murmur3 partitioner FQCN",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"partitioner": "org.apache.cassandra.dht.Murmur3Partitioner",
					},
				},
			},
			wantPartitioner: &utils.Murmur3Partitioner,
		},
		{
			name: "Random partitioner simple name",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"partitioner": "RandomPartitioner",
					},
				},
			},
			wantPartitioner: &utils.RandomPartitioner,
		},
		{
			name: "Random partitioner FQCN",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"partitioner": "org.apache.cassandra.dht.RandomPartitioner",
					},
				},
			},
			wantPartitioner: &utils.RandomPartitioner,
		},
		{
			name: "Unsupported partitioner",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"partitioner": "org.apache.cassandra.dht.ByteOrderedPartitioner",
					},
				},
			},
			wantPartitioner: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPartitioner, _ := computeDcPartitioner(tt.dc)
			assert.Equal(t, tt.wantPartitioner, gotPartitioner)
		})
	}
}

func Test_computeDcReplicationFactor(t *testing.T) {
	tests := []struct {
		name    string
		dc      *DatacenterConfig
		want    int
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "default rf <= dc size",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Size:    3,
			},
			want:    3,
			wantErr: assert.NoError,
		},
		{
			name: "default rf > dc size",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Size:    2,
			},
			want:    2,
			wantErr: assert.NoError,
		},
		{
			name: "custom rf <= dc size",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Size:    10,
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"allocate_tokens_for_local_replication_factor": 5,
					},
				},
			},
			want:    5,
			wantErr: assert.NoError,
		},
		{
			name: "custom rf > dc size",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Size:    3,
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"allocate_tokens_for_local_replication_factor": 5,
					},
				},
			},
			want:    3,
			wantErr: assert.NoError,
		},
		{
			name: "invalid num_tokens",
			dc: &DatacenterConfig{
				Cluster: "cluster1",
				Meta:    api.EmbeddedObjectMeta{Name: "dc1"},
				Size:    3,
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"allocate_tokens_for_local_replication_factor": "not a number",
					},
				},
			},
			want:    -1,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := computeDcReplicationFactor(tt.dc)
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, gotErr)
		})
	}
}

func Test_checkPartitioner(t *testing.T) {
	tests := []struct {
		name    string
		infos   []*tokenAllocationInfo
		want    *utils.Partitioner
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "no mismatch",
			infos: []*tokenAllocationInfo{
				{partitioner: &utils.Murmur3Partitioner},
				{partitioner: &utils.Murmur3Partitioner},
			},
			want:    &utils.Murmur3Partitioner,
			wantErr: assert.NoError,
		},
		{
			name: "mismatch",
			infos: []*tokenAllocationInfo{
				{partitioner: nil},
				{partitioner: &utils.Murmur3Partitioner},
				{partitioner: &utils.RandomPartitioner},
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := checkPartitioner(tt.infos)
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, gotErr)
		})
	}
}

func Test_assignInitialTokens(t *testing.T) {
	type args struct {
		dcConfigs        []*DatacenterConfig
		infos            []*tokenAllocationInfo
		allInitialTokens [][]string
	}
	tests := []struct {
		name string
		args args
		want []map[string][]string
	}{
		{
			name: "single DC single rack",
			args: args{
				dcConfigs: []*DatacenterConfig{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Cluster: "cluster1", Racks: nil},
				},
				infos: []*tokenAllocationInfo{
					{rf: 3, numTokens: 4},
				},
				allInitialTokens: [][]string{
					{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
				},
			},
			want: []map[string][]string{
				{
					"cluster1-dc1-default-sts-0": {"1", "4", "7", "10"},
					"cluster1-dc1-default-sts-1": {"2", "5", "8", "11"},
					"cluster1-dc1-default-sts-2": {"3", "6", "9", "12"},
				},
			},
		},
		{
			name: "single DC single rack single token",
			args: args{
				dcConfigs: []*DatacenterConfig{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Cluster: "cluster1", Racks: nil},
				},
				infos: []*tokenAllocationInfo{
					{rf: 3, numTokens: 1},
				},
				allInitialTokens: [][]string{
					{"1", "2", "3"},
				},
			},
			want: []map[string][]string{
				{
					"cluster1-dc1-default-sts-0": {"1"},
					"cluster1-dc1-default-sts-1": {"2"},
					"cluster1-dc1-default-sts-2": {"3"},
				},
			},
		},
		{
			name: "single DC multiple racks",
			args: args{
				dcConfigs: []*DatacenterConfig{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Cluster: "cluster1", Racks: []cassdcapi.Rack{{Name: "rack1"}, {Name: "rack2"}}},
				},
				infos: []*tokenAllocationInfo{
					{rf: 3, numTokens: 4},
				},
				allInitialTokens: [][]string{
					{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
				},
			},
			want: []map[string][]string{
				{
					"cluster1-dc1-rack1-sts-0": {"1", "4", "7", "10"},
					"cluster1-dc1-rack2-sts-0": {"2", "5", "8", "11"},
					"cluster1-dc1-rack1-sts-1": {"3", "6", "9", "12"},
				},
			},
		},
		{
			name: "multi DC single rack num_tokens < 16",
			args: args{
				dcConfigs: []*DatacenterConfig{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Cluster: "cluster1", Racks: nil},
					{Meta: api.EmbeddedObjectMeta{Name: "dc2"}, Cluster: "cluster1", Racks: nil},
				},
				infos: []*tokenAllocationInfo{
					{rf: 3, numTokens: 4},
					{rf: 5, numTokens: 8},
				},
				allInitialTokens: [][]string{
					{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
					{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31", "32", "33", "34", "35", "36", "37", "38", "39", "40"},
				},
			},
			want: []map[string][]string{
				{
					"cluster1-dc1-default-sts-0": {"1", "4", "7", "10"},
					"cluster1-dc1-default-sts-1": {"2", "5", "8", "11"},
					"cluster1-dc1-default-sts-2": {"3", "6", "9", "12"},
				},
				{
					"cluster1-dc2-default-sts-0": {"1", "6", "11", "16", "21", "26", "31", "36"},
					"cluster1-dc2-default-sts-1": {"2", "7", "12", "17", "22", "27", "32", "37"},
					"cluster1-dc2-default-sts-2": {"3", "8", "13", "18", "23", "28", "33", "38"},
					"cluster1-dc2-default-sts-3": {"4", "9", "14", "19", "24", "29", "34", "39"},
					"cluster1-dc2-default-sts-4": {"5", "10", "15", "20", "25", "30", "35", "40"},
				},
			},
		},
		{
			name: "multi DC single rack single token",
			args: args{
				dcConfigs: []*DatacenterConfig{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Cluster: "cluster1", Racks: nil},
					{Meta: api.EmbeddedObjectMeta{Name: "dc2"}, Cluster: "cluster1", Racks: nil},
				},
				infos: []*tokenAllocationInfo{
					{rf: 3, numTokens: 1},
					{rf: 5, numTokens: 1},
				},
				allInitialTokens: [][]string{
					{"1", "2", "3"},
					{"1", "2", "3", "4", "5"},
				},
			},
			want: []map[string][]string{
				{
					"cluster1-dc1-default-sts-0": {"1"},
					"cluster1-dc1-default-sts-1": {"2"},
					"cluster1-dc1-default-sts-2": {"3"},
				},
				{
					"cluster1-dc2-default-sts-0": {"1"},
					"cluster1-dc2-default-sts-1": {"2"},
					"cluster1-dc2-default-sts-2": {"3"},
					"cluster1-dc2-default-sts-3": {"4"},
					"cluster1-dc2-default-sts-4": {"5"},
				},
			},
		},
		{
			name: "multi DC multiple racks num_tokens < 16",
			args: args{
				dcConfigs: []*DatacenterConfig{
					{Meta: api.EmbeddedObjectMeta{Name: "dc1"}, Cluster: "cluster1", Racks: []cassdcapi.Rack{{Name: "rack1"}, {Name: "rack2"}}},
					{Meta: api.EmbeddedObjectMeta{Name: "dc2"}, Cluster: "cluster1", Racks: []cassdcapi.Rack{{Name: "rack1"}, {Name: "rack2"}, {Name: "rack3"}}},
				},
				infos: []*tokenAllocationInfo{
					{rf: 3, numTokens: 4},
					{rf: 5, numTokens: 8},
				},
				allInitialTokens: [][]string{
					{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
					{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31", "32", "33", "34", "35", "36", "37", "38", "39", "40"},
				},
			},
			want: []map[string][]string{
				{
					"cluster1-dc1-rack1-sts-0": {"1", "4", "7", "10"},
					"cluster1-dc1-rack2-sts-0": {"2", "5", "8", "11"},
					"cluster1-dc1-rack1-sts-1": {"3", "6", "9", "12"},
				},
				{
					"cluster1-dc2-rack1-sts-0": {"1", "6", "11", "16", "21", "26", "31", "36"},
					"cluster1-dc2-rack2-sts-0": {"2", "7", "12", "17", "22", "27", "32", "37"},
					"cluster1-dc2-rack3-sts-0": {"3", "8", "13", "18", "23", "28", "33", "38"},
					"cluster1-dc2-rack1-sts-1": {"4", "9", "14", "19", "24", "29", "34", "39"},
					"cluster1-dc2-rack2-sts-1": {"5", "10", "15", "20", "25", "30", "35", "40"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignInitialTokens(tt.args.dcConfigs, tt.args.infos, tt.args.allInitialTokens)
			for i, dcConfig := range tt.args.dcConfigs {
				assert.Equal(t, tt.want[i], dcConfig.InitialTokensByPodName)
			}
		})
	}
}
