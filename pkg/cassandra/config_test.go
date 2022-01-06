package cassandra

import (
	"k8s.io/utils/pointer"
	"testing"

	"github.com/Jeffail/gabs"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
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
			name:     "single-dc with no jvm options",
			dcConfig: &DatacenterConfig{},
			replication: SystemReplication{
				Datacenters:       []string{"dc1"},
				ReplicationFactor: 3,
			},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						AdditionalOptions: []string{
							SystemReplicationDcNames + "=dc1",
							SystemReplicationFactor + "=3",
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
						HeapSize: parseResource("1024Mi"),
					},
				},
			},
			replication: SystemReplication{
				Datacenters:       []string{"dc1"},
				ReplicationFactor: 3,
			},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
						AdditionalOptions: []string{
							SystemReplicationDcNames + "=dc1",
							SystemReplicationFactor + "=3",
						},
					},
				},
			},
		},
		{
			name:     "multi-dc with no jvm options",
			dcConfig: &DatacenterConfig{},
			replication: SystemReplication{
				Datacenters:       []string{"dc1", "dc2", "dc3"},
				ReplicationFactor: 3,
			},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						AdditionalOptions: []string{
							SystemReplicationDcNames + "=dc1,dc2,dc3",
							SystemReplicationFactor + "=3",
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
						HeapSize: parseResource("1024Mi"),
					},
				},
			},
			replication: SystemReplication{
				Datacenters:       []string{"dc1", "dc2", "dc3"},
				ReplicationFactor: 3,
			},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					JvmOptions: api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
						AdditionalOptions: []string{
							SystemReplicationDcNames + "=dc1,dc2,dc3",
							SystemReplicationFactor + "=3",
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
		name             string
		cassandraVersion string
		config           api.CassandraConfig
		got              []byte
		want             string
	}

	heapSize := resource.MustParse("1024Mi")

	tests := []test{
		{
			name:             "[4.0.0] concurrent_reads, concurrent_writes, concurrent_counter_writes",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					ConcurrentReads:         pointer.Int(8),
					ConcurrentWrites:        pointer.Int(16),
					ConcurrentCounterWrites: pointer.Int(4),
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
			name:             "[3.11.11] heap size",
			cassandraVersion: "3.11.11",
			config: api.CassandraConfig{
				JvmOptions: api.JvmOptions{
					HeapSize: &heapSize,
				},
			},
			want: `{
              "cassandra-yaml": {
                "num_tokens": 256
              },
              "jvm-options": {
                "initial_heap_size": 1073741824,
                "max_heap_size": 1073741824
              }
            }`,
		},
		{
			name:             "[4.0.0] heap size",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				JvmOptions: api.JvmOptions{
					HeapSize: &heapSize,
				},
			},
			want: `{
              "cassandra-yaml": {
                "num_tokens": 16
              },
              "jvm-server-options": {
                "initial_heap_size": 1073741824,
                "max_heap_size": 1073741824
              }
            }`,
		},
		{
			name:             "[4.0.0] concurrent_reads and concurrent_writes with system replication",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					ConcurrentReads:  pointer.Int(8),
					ConcurrentWrites: pointer.Int(16),
				},
				JvmOptions: api.JvmOptions{
					AdditionalOptions: []string{
						SystemReplicationDcNames + "=dc1,dc2,dc3",
						SystemReplicationFactor + "=3",
					},
				},
			},
			want: `{
              "cassandra-yaml": {
                "num_tokens": 16,
                "concurrent_reads": 8,
                "concurrent_writes": 16
              },
              "cassandra-env-sh": {
                "additional-jvm-opts": [
                  "-Dcassandra.system_distributed_replication_dc_names=dc1,dc2,dc3", 
                  "-Dcassandra.system_distributed_replication_per_dc=3"
                ]
              }
            }`,
		},
		{
			name:             "[4.0.0] auto_snapshot, memtable_flush_writers, commitlog_segment_size_in_mb",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					AutoSnapshot:           pointer.Bool(true),
					MemtableFlushWriters:   pointer.Int(10),
					CommitLogSegmentSizeMb: pointer.Int(8192),
				},
			},
			want: `{
              "cassandra-yaml": {
                "num_tokens": 16,
                "auto_snapshot": true,
                "memtable_flush_writers": 10,
                "commitlog_segment_size_in_mb": 8192
              }
            }`,
		},
		{
			name:             "[4.0.0] concurrent_compactors, compaction_throughput_mb_per_sec, sstable_preemptive_open_interval_in_mb",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					ConcurrentCompactors:            pointer.Int(4),
					CompactionThroughputMbPerSec:    pointer.Int(64),
					SstablePreemptiveOpenIntervalMb: pointer.Int(0),
				},
			},
			want: `{
              "cassandra-yaml": {
				"num_tokens": 16,
                "concurrent_compactors": 4,
                "compaction_throughput_mb_per_sec": 64,
                "sstable_preemptive_open_interval_in_mb": 0
              }
            }`,
		},
		{
			name:             "[4.0.0] key_cache_size_in_mb, counter_cache_size_in_mb, prepared_statements_cache_size_mb, slow_query_log_timeout_in_ms",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					KeyCacheSizeMb:                pointer.Int(100),
					CounterCacheSizeMb:            pointer.Int(50),
					PreparedStatementsCacheSizeMb: pointer.Int(180),
					SlowQueryLogTimeoutMs:         pointer.Int(500),
				},
			},
			want: `{
              "cassandra-yaml": {
				"num_tokens": 16,
                "key_cache_size_in_mb": 100,
                "counter_cache_size_in_mb": 50,
                "prepared_statements_cache_size_mb": 180,
                "slow_query_log_timeout_in_ms": 500
              }
            }`,
		},
		{
			name:             "[4.0.0] file_cache_size_in_mb, row_cache_size_in_mb",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					FileCacheSizeMb: pointer.Int(500),
					RowCacheSizeMb:  pointer.Int(100),
				},
			},
			want: `{
              "cassandra-yaml": {
				"num_tokens": 16,
                "file_cache_size_in_mb": 500,
                "row_cache_size_in_mb": 100
              }
            }`,
		},
		{
			name:             "[3.11.10] start_rpc, thrift_prepared_statements_cache_size_mb",
			cassandraVersion: "3.11.10",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					StartRpc:                           pointer.Bool(false),
					ThriftPreparedStatementCacheSizeMb: pointer.Int(1),
				},
			},
			want: `{
              "cassandra-yaml": {
				"num_tokens": 256,
                "start_rpc": false,
                "thrift_prepared_statements_cache_size_mb": 1
              }
            }`,
		},
		{
			name:             "[4.0.0] start_rpc, thrift_prepared_statements_cache_size_mb",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					StartRpc:                           pointer.Bool(false),
					ThriftPreparedStatementCacheSizeMb: pointer.Int(1),
				},
			},
			want: `{
              "cassandra-yaml": {
				"num_tokens": 16
              }
            }`,
		},
		{
			name:             "[3.11.11] num_tokens",
			cassandraVersion: "3.11.11",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					NumTokens: pointer.Int(32),
				},
			},
			want: `{
              "cassandra-yaml": {
                "num_tokens": 32
              }
            }`,
		},
		{
			name:             "[4.0.0] num_tokens",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					NumTokens: pointer.Int(32),
				},
			},
			want: `{
              "cassandra-yaml": {
                "num_tokens": 32
              }
            }`,
		},
		{
			name:             "[4.0.0] allocate_tokens_for_local_replication_factor",
			cassandraVersion: "4.0.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					AllocateTokensForLocalReplicationFactor: pointer.Int(5),
				},
			},
			want: `{
              "cassandra-yaml": {
                "allocate_tokens_for_local_replication_factor": 5,
				"num_tokens": 16
              }
            }`,
		},
		{
			name:             "[3.11.11] auth",
			cassandraVersion: "3.11.11",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					Authenticator:                   pointer.String("FakeAuthenticator"),
					Authorizer:                      pointer.String("FakeAuthorizer"),
					RoleManager:                     pointer.String("FakeRoleManager"),
					RolesValidityMillis:             pointer.Int64(123),
					RolesUpdateIntervalMillis:       pointer.Int64(123),
					PermissionsValidityMillis:       pointer.Int64(456),
					PermissionsUpdateIntervalMillis: pointer.Int64(456),
					CredentialsValidityMillis:       pointer.Int64(789),
					CredentialsUpdateIntervalMillis: pointer.Int64(789),
				},
			},
			want: `{
				"cassandra-yaml": {
					"authenticator": "FakeAuthenticator",
					"authorizer": "FakeAuthorizer",
					"role_manager": "FakeRoleManager",
					"roles_validity_in_ms": 123,
					"roles_update_interval_in_ms": 123,
					"permissions_validity_in_ms": 456,
					"permissions_update_interval_in_ms": 456,
					"credentials_validity_in_ms": 789,
					"credentials_update_interval_in_ms": 789,
					"num_tokens": 256
				}
		   }`,
		},
		{
			name:             "[4.0.0] auth",
			cassandraVersion: "4.0",
			config: api.CassandraConfig{
				CassandraYaml: api.CassandraYaml{
					Authenticator:                   pointer.String("FakeAuthenticator"),
					Authorizer:                      pointer.String("FakeAuthorizer"),
					RoleManager:                     pointer.String("FakeRoleManager"),
					RolesValidityMillis:             pointer.Int64(123),
					RolesUpdateIntervalMillis:       pointer.Int64(123),
					PermissionsValidityMillis:       pointer.Int64(456),
					PermissionsUpdateIntervalMillis: pointer.Int64(456),
					CredentialsValidityMillis:       pointer.Int64(789),
					CredentialsUpdateIntervalMillis: pointer.Int64(789),
				},
			},
			want: `{
				"cassandra-yaml": {
					"authenticator": "FakeAuthenticator",
					"authorizer": "FakeAuthorizer",
					"role_manager": "FakeRoleManager",
					"roles_validity_in_ms": 123,
					"roles_update_interval_in_ms": 123,
					"permissions_validity_in_ms": 456,
					"permissions_update_interval_in_ms": 456,
					"credentials_validity_in_ms": 789,
					"credentials_update_interval_in_ms": 789,
					"num_tokens": 16
				}
		   }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			tc.got, err = CreateJsonConfig(tc.config, tc.cassandraVersion)
			require.NoError(t, err, "failed to create json dcConfig")
			expected, err := gabs.ParseJSON([]byte(tc.want))
			require.NoError(t, err, "failed to parse expected value")
			actual, err := gabs.ParseJSON(tc.got)
			require.NoError(t, err, "failed to parse actual value")
			assert.Equal(t, expected, actual)
		})
	}

}

func parseResource(quantity string) *resource.Quantity {
	parsed := resource.MustParse(quantity)
	return &parsed
}
