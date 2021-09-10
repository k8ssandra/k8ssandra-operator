package cassandra

import (
	"github.com/Jeffail/gabs"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
	"testing"
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
				CassandraConfig: &api.CassandraConfig{
					JvmOptions: &api.JvmOptions{
						AdditionalOptions: []string{
							systemReplicationDcNames + "=dc1",
							systemReplicationFactor + "=3",
						},
					},
				},
			},
		},
		{
			name: "sing-dc with jvm options",
			dcConfig: &DatacenterConfig{
				CassandraConfig: &api.CassandraConfig{
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
					},
				},
			},
			replication: SystemReplication{
				Datacenters:       []string{"dc1"},
				ReplicationFactor: 3,
			},
			want: &DatacenterConfig{
				CassandraConfig: &api.CassandraConfig{
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
						AdditionalOptions: []string{
							systemReplicationDcNames + "=dc1",
							systemReplicationFactor + "=3",
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
				CassandraConfig: &api.CassandraConfig{
					JvmOptions: &api.JvmOptions{
						AdditionalOptions: []string{
							systemReplicationDcNames + "=dc1,dc2,dc3",
							systemReplicationFactor + "=3",
						},
					},
				},
			},
		},
		{
			name: "multi-dc with jvm options",
			dcConfig: &DatacenterConfig{
				CassandraConfig: &api.CassandraConfig{
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
					},
				},
			},
			replication: SystemReplication{
				Datacenters:       []string{"dc1", "dc2", "dc3"},
				ReplicationFactor: 3,
			},
			want: &DatacenterConfig{
				CassandraConfig: &api.CassandraConfig{
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
						AdditionalOptions: []string{
							systemReplicationDcNames + "=dc1,dc2,dc3",
							systemReplicationFactor + "=3",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		ApplySystemReplication(tc.dcConfig, tc.replication)
		if !reflect.DeepEqual(tc.want, tc.dcConfig) {
			t.Errorf("%s - expected: %+v, got: %+v", tc.name, *tc.want, *tc.dcConfig)
		}
	}
}

func TestCreateJsonConfig(t *testing.T) {
	type test struct {
		name             string
		cassandraVersion string
		config           *api.CassandraConfig
		got              []byte
		want             string
	}

	heapSize := resource.MustParse("1024Mi")

	tests := []test{
		{
			name:             "concurrent_reads, concurrent_writes, concurrent_counter_writes",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					ConcurrentReads:         intPtr(8),
					ConcurrentWrites:        intPtr(16),
					ConcurrentCounterWrites: intPtr(4),
				},
			},
			want: `{
              "cassandra-yaml": {
                "concurrent_reads": 8,
                "concurrent_writes": 16,
                "concurrent_counter_writes": 4
              }
            }`,
		},
		{
			name:             "heap size - Cassandra 4.0",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				JvmOptions: &api.JvmOptions{
					HeapSize: &heapSize,
				},
			},
			want: `{
              "jvm-server-options": {
                "initial_heap_size": 1073741824,
                "max_heap_size": 1073741824
              }
            }`,
		},
		{
			name:             "concurrent_reads and concurrent_writes with system replication",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					ConcurrentReads:  intPtr(8),
					ConcurrentWrites: intPtr(16),
				},
				JvmOptions: &api.JvmOptions{
					AdditionalOptions: []string{
						systemReplicationDcNames + "=dc1,dc2,dc3",
						systemReplicationFactor + "=3",
					},
				},
			},
			want: `{
              "cassandra-yaml": {
                "concurrent_reads": 8,
                "concurrent_writes": 16
              },
              "jvm-server-options": {
                "additional-jvm-opts": [
                  "-Dcassandra.system_distributed_replication_dc_names=dc1,dc2,dc3", 
                  "-Dcassandra.system_distributed_replication_per_dc=3"
                ]
              }
            }`,
		},
		{
			name:             "auto_snapshot, memtable_flush_writers, commitlog_segment_size_in_mb",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					AutoSnapshot:           boolPtr(true),
					MemtableFlushWriters:   intPtr(10),
					CommitLogSegmentSizeMb: intPtr(8192),
				},
			},
			want: `{
              "cassandra-yaml": {
                "auto_snapshot": true,
                "memtable_flush_writers": 10,
                "commitlog_segment_size_in_mb": 8192
              }
            }`,
		},
		{
			name:             "concurrent_compactors, compaction_throughput_mb_per_sec, sstable_preemptive_open_interval_in_mb",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					ConcurrentCompactors:            intPtr(4),
					CompactionThroughputMbPerSec:    intPtr(64),
					SstablePreemptiveOpenIntervalMb: intPtr(0),
				},
			},
			want: `{
              "cassandra-yaml": {
                "concurrent_compactors": 4,
                "compaction_throughput_mb_per_sec": 64,
                "sstable_preemptive_open_interval_in_mb": 0
              }
            }`,
		},
		{
			name:             "key_cache_size_in_mb, counter_cache_size_in_mb, prepared_statements_cache_size_mb, slow_query_log_timeout_in_ms",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					KeyCacheSizeMb:                intPtr(100),
					CounterCacheSizeMb:            intPtr(50),
					PreparedStatementsCacheSizeMb: intPtr(180),
					SlowQueryLogTimeoutMs:         intPtr(500),
				},
			},
			want: `{
              "cassandra-yaml": {
                "key_cache_size_in_mb": 100,
                "counter_cache_size_in_mb": 50,
                "prepared_statements_cache_size_mb": 180,
                "slow_query_log_timeout_in_ms": 500
              }
            }`,
		},
		{
			name:             "file_cache_size_in_mb, row_cache_size_in_mb",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					FileCacheSizeMb: intPtr(500),
					RowCacheSizeMb:  intPtr(100),
				},
			},
			want: `{
              "cassandra-yaml": {
                "file_cache_size_in_mb": 500,
                "row_cache_size_in_mb": 100
              }
            }`,
		},
		{
			name:             "[3.11.10] start_rpc, thrift_prepared_statements_cache_size_mb",
			cassandraVersion: "3.11.10",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					StartRpc:                           boolPtr(false),
					ThriftPreparedStatementCacheSizeMb: intPtr(1),
				},
			},
			want: `{
              "cassandra-yaml": {
                "start_rpc": false,
                "thrift_prepared_statements_cache_size_mb": 1
              }
            }`,
		},
		{
			name:             "[4.0] start_rpc, thrift_prepared_statements_cache_size_mb",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					StartRpc:                           boolPtr(false),
					ThriftPreparedStatementCacheSizeMb: intPtr(1),
				},
			},
			want: `{
              "cassandra-yaml": { 
              }
            }`,
		},
		//{
		//	name: "auth",
		//	cassandraVersion: "4.0",
		//	config: &api.CassandraConfig{
		//		CassandraYaml: &api.CassandraYaml{
		//			Authenticator: "FakeAuthenticator",
		//			Authorizer: "FakeAuthorizer",
		//		},
		//	},
		//	want: `{
		//      "cassandra-yaml": {
		//        "authenticator": "FakeAuthenticator",
		//        "authorizer": "FakeAuthorizer"
		//      }
		//    }`,
		//},
	}

	for _, tc := range tests {
		var err error
		tc.got, err = CreateJsonConfig(tc.config, tc.cassandraVersion)
		if err != nil {
			t.Errorf("%s - failed to create json dcConfig: %s", tc.name, err)
			continue
		}

		expected, err := gabs.ParseJSON([]byte(tc.want))
		if err != nil {
			t.Errorf("%s - failed to parse expected value: %s", tc.name, err)
			continue
		}

		actual, err := gabs.ParseJSON(tc.got)
		if err != nil {
			t.Errorf("%s - failed to parse actual value: %s", tc.name, err)
			continue
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Errorf("%s - wanted: %s, got: %s", tc.name, expected, actual)
		}
	}
}

func intPtr(n int) *int {
	return &n
}

func boolPtr(b bool) *bool {
	return &b
}

func parseResource(quantity string) *resource.Quantity {
	parsed := resource.MustParse(quantity)
	return &parsed
}
