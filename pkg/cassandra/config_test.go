package cassandra

import (
	"encoding/json"
	"github.com/Jeffail/gabs"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
	"strconv"
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
			name:             "concurrent_reads and concurrent_writes",
			cassandraVersion: "4.0",
			config: &api.CassandraConfig{
				CassandraYaml: &api.CassandraYaml{
					ConcurrentReads:  intPtr(8),
					ConcurrentWrites: intPtr(16),
				},
			},
			want: `{
              "cassandra-yaml": {
                "concurrent_reads": 8,
                "concurrent_writes": 16
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
			name: "concurrent_reads and concurrent_writes with system replication",
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

func parseResource(quantity string) *resource.Quantity {
	parsed := resource.MustParse(quantity)
	return &parsed
}

func resourceQuantityToNumber(q resource.Quantity) json.Number {
	val := q.Value()
	return json.Number(strconv.FormatInt(val, 10))
}
