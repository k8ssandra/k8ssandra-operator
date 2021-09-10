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

// config is an internal type that is intended to be marshaled into JSON that is a valid
// value for CassandraDatacenter.Spec.Config.
type config struct {
	cassandraVersion string

	*api.CassandraYaml

	JvmOptions *jvmOptions
}

// jvmOptions is an internal type that is intended to be marshaled into JSON that is valid
// for the jvm options portion of the value supplied to CassandraDatacenter.Spec.Config.
type jvmOptions struct {
	InitialHeapSize   *int64   `json:"initial_heap_size,omitempty"`
	MaxHeapSize       *int64   `json:"max_heap_size,omitempty"`
	HeapNewGenSize    *int64   `json:"heap_size_young_generation,omitempty"`
	AdditionalOptions []string `json:"additional-jvm-opts,omitempty"`
}

func isCassandra4(version string) bool {
	return strings.HasPrefix(version, "4.0")
}

func (c config) MarshalJSON() ([]byte, error) {
	config := make(map[string]interface{}, 0)

	if c.CassandraYaml != nil {
		if isCassandra4(c.cassandraVersion) {
			c.StartRpc = nil
			c.ThriftPreparedStatementCacheSizeMb = nil
		}
		config["cassandra-yaml"] = c.CassandraYaml
	}

	if c.JvmOptions != nil {
		if strings.HasPrefix(c.cassandraVersion, "3.11") {
			config["jvm-options"] = c.JvmOptions
		} else {
			config["jvm-server-options"] = c.JvmOptions
		}
	}

	return json.Marshal(&config)
}

func newConfig(apiConfig *api.CassandraConfig, cassandraVersion string) config {
	config := config{cassandraVersion: cassandraVersion}

	if apiConfig.CassandraYaml != nil {
		config.CassandraYaml = apiConfig.CassandraYaml
	}

	if apiConfig.JvmOptions != nil {
		config.JvmOptions = &jvmOptions{}
		if apiConfig.JvmOptions.HeapSize != nil {
			heapSize := apiConfig.JvmOptions.HeapSize.Value()
			config.JvmOptions.InitialHeapSize = &heapSize
			config.JvmOptions.MaxHeapSize = &heapSize
		}

		if apiConfig.JvmOptions.HeapNewGenSize != nil {
			newGenSize := apiConfig.JvmOptions.HeapNewGenSize.Value()
			config.JvmOptions.HeapNewGenSize = &newGenSize
		}

		config.JvmOptions.AdditionalOptions = apiConfig.JvmOptions.AdditionalOptions
	}

	return config
}

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
	cfg := newConfig(config, cassandraVersion)
	return json.Marshal(cfg)
}
