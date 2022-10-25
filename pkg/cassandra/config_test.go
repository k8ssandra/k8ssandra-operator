package cassandra

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/utils/pointer"

	"github.com/Jeffail/gabs"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplySystemReplication(t *testing.T) {
	type test struct {
		name        string
		dcConfig    *DatacenterConfig
		replication SystemReplication
		want        *DatacenterConfig
	}

	tests := []test{
		{
			name:        "single-dc with no jvm options",
			dcConfig:    &DatacenterConfig{},
			replication: SystemReplication{"dc1": 3},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						AdditionalOptions: []string{
							SystemReplicationFactorStrategy + "=dc1:3",
						},
					},
				},
			},
		},
		{
			name: "sing-dc with jvm options",
			dcConfig: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						MaxHeapSize: parseQuantity("1024Mi"),
					},
				},
			},
			replication: SystemReplication{"dc1": 3},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						MaxHeapSize: parseQuantity("1024Mi"),
						AdditionalOptions: []string{
							SystemReplicationFactorStrategy + "=dc1:3",
						},
					},
				},
			},
		},
		{
			name:     "multi-dc with no jvm options",
			dcConfig: &DatacenterConfig{},
			replication: SystemReplication{
				"dc1": 3,
				"dc2": 3,
				"dc3": 1,
			},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						AdditionalOptions: []string{
							SystemReplicationFactorStrategy + "=dc1:3,dc2:3,dc3:1",
						},
					},
				},
			},
		},
		{
			name: "multi-dc with jvm options",
			dcConfig: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						MaxHeapSize: parseQuantity("1024Mi"),
					},
				},
			},
			replication: SystemReplication{
				"dc1": 3,
				"dc2": 2,
				"dc3": 1,
			},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						MaxHeapSize: parseQuantity("1024Mi"),
						AdditionalOptions: []string{
							SystemReplicationFactorStrategy + "=dc1:3,dc2:2,dc3:1",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ApplySystemReplication(tc.dcConfig, tc.replication)
			require.Equal(t, tc.want, tc.dcConfig)
		})
	}
}

func TestCreateJsonConfig(t *testing.T) {
	type test struct {
		name            string
		serverVersion   *semver.Version
		serverType      api.ServerDistribution
		cassandraConfig api.CassandraConfig
		got             []byte
		want            string
	}

	tests := []test{
		{
			name:          "[4.0.0] simple",
			serverVersion: semver.MustParse("4.0.0"),
			serverType:    api.ServerDistributionCassandra,
			cassandraConfig: api.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{
					"num_tokens":                16,
					"concurrent_reads":          8,
					"concurrent_writes":         16,
					"concurrent_counter_writes": 4,
				},
			},
			want: `{
             "cassandra-yaml": {
               "num_tokens": 16,
               "concurrent_reads": 8,
               "concurrent_writes": 16,
               "concurrent_counter_writes": 4
             }
           }`,
		},
		{
			name:          "[4.0.0] system replication",
			serverVersion: semver.MustParse("4.0.0"),
			serverType:    api.ServerDistributionCassandra,
			cassandraConfig: api.CassandraConfig{
				JvmOptions: api.JvmOptions{
					AdditionalOptions: []string{
						SystemReplicationFactorStrategy + "=dc1:3,dc2:3,dc3:3",
					},
				},
			},
			want: `{
             "cassandra-env-sh": {
               "additional-jvm-opts": [
                 "-Dcassandra.system_distributed_replication=dc1:3,dc2:3,dc3:3"
               ]
             }
           }`,
		},
		{
			name:          "[3.11.11] GC",
			serverVersion: semver.MustParse("3.11.11"),
			serverType:    api.ServerDistributionCassandra,
			cassandraConfig: api.CassandraConfig{
				JvmOptions: api.JvmOptions{
					GarbageCollector: pointer.String("G1GC"),
				},
			},
			want: `{
             "jvm-options": {
               "garbage_collector": "G1GC"
             }
           }`,
		},
		{
			name:          "[4.0.0] GC",
			serverVersion: semver.MustParse("4.0.0"),
			serverType:    api.ServerDistributionCassandra,
			cassandraConfig: api.CassandraConfig{
				JvmOptions: api.JvmOptions{
					GarbageCollector: pointer.String("ZGC"),
				},
			},
			want: `{
             "jvm11-server-options": {
               "garbage_collector": "ZGC"
             }
           }`,
		},
		{
			name:          "[DSE 6.8.25] simple",
			serverVersion: semver.MustParse("6.8.25"),
			serverType:    api.ServerDistributionDse,
			cassandraConfig: api.CassandraConfig{
				JvmOptions: api.JvmOptions{
					GarbageCollector:             pointer.String("ZGC"),
					AdditionalJvm11ServerOptions: []string{"-XX:+UseConcMarkSweepGC"},
				},
				DseYaml: unstructured.Unstructured{
					"authentication_options": map[string]interface{}{
						"enabled": true,
					},
				},
			},
			want: `{
             "jvm11-server-options": {
               "garbage_collector": "ZGC",
							 "additional-jvm-opts": ["-XX:+UseConcMarkSweepGC"]
             },
             "dse-yaml": {
               "authentication_options": {
				  "enabled": true
               }
             }
           }`,
		},
		{
			name:          "[DSE 6.8.25] multiple jvm-option files",
			serverVersion: semver.MustParse("6.8.25"),
			serverType:    api.ServerDistributionDse,
			cassandraConfig: api.CassandraConfig{
				JvmOptions: api.JvmOptions{
					GarbageCollector:             pointer.String("ZGC"),
					AdditionalJvm11ServerOptions: []string{"-XX:+UseConcMarkSweepGC"},
					AdditionalJvm8ServerOptions:  []string{"-XX:ThreadPriorityPolicy=42"},
					AdditionalJvmServerOptions:   []string{"-Dio.netty.maxDirectMemory=0"},
				},
				DseYaml: unstructured.Unstructured{
					"authentication_options": map[string]interface{}{
						"enabled": true,
					},
				},
			},
			want: `{
				    "jvm-server-options": {
							"additional-jvm-opts": ["-Dio.netty.maxDirectMemory=0"]
						},
						 "jvm8-server-options": {
							"additional-jvm-opts": ["-XX:ThreadPriorityPolicy=42"]
						},
            "jvm11-server-options": {
              "garbage_collector": "ZGC",
							"additional-jvm-opts": ["-XX:+UseConcMarkSweepGC"]
            },
            "dse-yaml": {
              "authentication_options": {
				        "enabled": true
               }
             }
           }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			tc.got, err = createJsonConfig(tc.cassandraConfig, tc.serverVersion, tc.serverType)
			require.NoError(t, err, "failed to create json dcConfig")
			expected, err := gabs.ParseJSON([]byte(tc.want))
			require.NoError(t, err, "failed to parse expected value")
			actual, err := gabs.ParseJSON(tc.got)
			require.NoError(t, err, "failed to parse actual value")
			assert.Equal(t, expected, actual)
		})
	}

}

func parseQuantity(quantity string) *resource.Quantity {
	parsed := resource.MustParse(quantity)
	return &parsed
}
