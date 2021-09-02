package cassandra

import (
	"encoding/json"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"strconv"
	"strings"
)

const (
	systemReplicationDcNames = "-Dcassandra.system_distributed_replication_dc_names"
	systemReplicationFactor  = "-Dcassandra.system_distributed_replication_per_dc"
)

type NodeConfig map[string]interface{}

// ApplySystemReplication adds system properties to configure replication of system
// keyspaces.
func ApplySystemReplication(dcConfig *DatacenterConfig, replication SystemReplication) {
	config := dcConfig.CassandraConfig
	if config == nil {
		config = &api.CassandraConfig{
			JvmOptions: &api.JvmOptions{},
		}
	} else if config.JvmOptions == nil {
		config.JvmOptions = &api.JvmOptions{}
	}

	jvmOpts := config.JvmOptions
	additionalOpts := jvmOpts.AdditionalOptions
	if additionalOpts == nil {
		additionalOpts = make([]string, 0, 2)
	}

	dcNames := "-Dcassandra.system_distributed_replication_dc_names=" + strings.Join(replication.Datacenters, ",")
	replicationFactor := "-Dcassandra.system_distributed_replication_per_dc=" + strconv.Itoa(replication.ReplicationFactor)
	additionalOpts = append(additionalOpts, dcNames, replicationFactor)

	jvmOpts.AdditionalOptions = additionalOpts
	config.JvmOptions = jvmOpts
	dcConfig.CassandraConfig = config
}

// CreateJsonConfig parses dcConfig into a raw JSON base64-encoded string.
func CreateJsonConfig(config *api.CassandraConfig, cassandraVersion string) ([]byte, error) {
	rawConfig := NodeConfig{}
	cassandraYaml := NodeConfig{}
	jvmOpts := NodeConfig{}

	if config == nil {
		return nil, nil
	}

	if config.CassandraYaml != nil {
		//if len(config.CassandraYaml.Authenticator) > 0 {
		//	cassandraYaml["authenticator"] = config.CassandraYaml.Authenticator
		//}
		//
		//if len(config.CassandraYaml.Authorizer) > 0 {
		//	cassandraYaml["authorizer"] = config.CassandraYaml.Authorizer
		//}
		//
		//if len(config.CassandraYaml.RoleManager) > 0 {
		//
		//}

		if config.CassandraYaml.ConcurrentReads != nil {
			cassandraYaml["concurrent_reads"] = config.CassandraYaml.ConcurrentReads
		}

		if config.CassandraYaml.ConcurrentWrites != nil {
			cassandraYaml["concurrent_writes"] = config.CassandraYaml.ConcurrentWrites
		}

		if config.CassandraYaml.AutoSnapshot != nil {
			cassandraYaml["auto_snapshot"] = config.CassandraYaml.AutoSnapshot
		}

		if config.CassandraYaml.MemtableFlushWriters != nil {
			cassandraYaml["memtable_flush_writers"] = config.CassandraYaml.MemtableFlushWriters
		}

		if config.CassandraYaml.CommitLogSegmentSizeMb != nil {
			cassandraYaml["commitlog_segment_size_in_mb"] = config.CassandraYaml.CommitLogSegmentSizeMb
		}

		if config.CassandraYaml.ConcurrentCompactors != nil {
			cassandraYaml["concurrent_compactors"] = config.CassandraYaml.ConcurrentCompactors
		}

		if config.CassandraYaml.CompactionThroughputMbPerSec != nil {
			cassandraYaml["compaction_throughput_mb_per_sec"] = config.CassandraYaml.CompactionThroughputMbPerSec
		}

		if config.CassandraYaml.SstablePreemptiveOpenIntervalMb != nil {
			cassandraYaml["sstable_preemptive_open_interval_in_mb"] = config.CassandraYaml.SstablePreemptiveOpenIntervalMb
		}

		if config.CassandraYaml.KeyCacheSizeMb != nil {
			cassandraYaml["key_cache_size_in_mb"] = config.CassandraYaml.KeyCacheSizeMb
		}

		if config.CassandraYaml.FileCacheSizeMb != nil {
			cassandraYaml["file_cache_size_in_mb"] = config.CassandraYaml.FileCacheSizeMb
		}

		if config.CassandraYaml.RowCacheSizeMb != nil {
			cassandraYaml["row_cache_size_in_mb"] = config.CassandraYaml.RowCacheSizeMb
		}

		if config.CassandraYaml.PreparedStatementsCacheSizeMb != nil {
			cassandraYaml["prepared_statements_cache_size_mb"] = config.CassandraYaml.PreparedStatementsCacheSizeMb
		}

		if config.CassandraYaml.SlowQueryLogTimeoutMs != nil {
			cassandraYaml["slow_query_log_timeout_in_ms"] = config.CassandraYaml.SlowQueryLogTimeoutMs
		}

		if config.CassandraYaml.CounterCacheSizeMb != nil {
			cassandraYaml["counter_cache_size_in_mb"] = config.CassandraYaml.CounterCacheSizeMb
		}

		if config.CassandraYaml.ConcurrentCounterWrites != nil {
			cassandraYaml["concurrent_counter_writes"] = config.CassandraYaml.ConcurrentCounterWrites
		}

		if strings.HasPrefix(cassandraVersion, "3.") {
			if config.CassandraYaml.StartRpc != nil {
				cassandraYaml["start_rpc"] = config.CassandraYaml.StartRpc
			}

			if config.CassandraYaml.ThriftPreparedStatementCacheSizeMb != nil {
				cassandraYaml["thrift_prepared_statements_cache_size_mb"] = config.CassandraYaml.ThriftPreparedStatementCacheSizeMb
			}
		}

		rawConfig["cassandra-yaml"] = cassandraYaml
	}

	if config.JvmOptions != nil {
		if config.JvmOptions.HeapSize != nil {
			jvmOpts["initial_heap_size"] = config.JvmOptions.HeapSize.Value()
			jvmOpts["max_heap_size"] = config.JvmOptions.HeapSize.Value()
		}

		if len(config.JvmOptions.AdditionalOptions) > 0 {
			jvmOpts["additional-jvm-opts"] = config.JvmOptions.AdditionalOptions
		}

		if strings.HasPrefix(cassandraVersion, "3.") {
			rawConfig["jvm-options"] = jvmOpts
		} else {
			rawConfig["jvm-server-options"] = jvmOpts
		}
	}

	jsonConfig, err := json.Marshal(rawConfig)
	if err != nil {
		return nil, err
	}

	return jsonConfig, nil
}
