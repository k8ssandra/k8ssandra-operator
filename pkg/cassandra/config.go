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
		if config.CassandraYaml.ConcurrentReads != nil {
			cassandraYaml["concurrent_reads"] = config.CassandraYaml.ConcurrentReads
		}

		if config.CassandraYaml.ConcurrentWrites != nil {
			cassandraYaml["concurrent_writes"] = config.CassandraYaml.ConcurrentWrites
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
