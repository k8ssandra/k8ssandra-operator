package cassandra

import (
	"encoding/json"
	"k8s.io/utils/pointer"
	"strconv"
	"strings"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
)

const (
	SystemReplicationDcNames = "-Dcassandra.system_distributed_replication_dc_names"
	SystemReplicationFactor  = "-Dcassandra.system_distributed_replication_per_dc"
	allowAlterRf             = "-Dcassandra.allow_alter_rf_during_range_movement=true"
)

// config is an internal type that is intended to be marshaled into JSON that is a valid
// value for CassandraDatacenter.Spec.Config.
type config struct {
	api.CassandraYaml
	cassandraVersion     string
	jvmOptions           jvmOptions
	additionalJvmOptions []string
}

// jvmOptions is an internal type that is intended to be marshaled into JSON that is valid
// for the jvm options portion of the value supplied to CassandraDatacenter.Spec.Config.
type jvmOptions struct {
	InitialHeapSize *int64 `json:"initial_heap_size,omitempty"`
	MaxHeapSize     *int64 `json:"max_heap_size,omitempty"`
	HeapNewGenSize  *int64 `json:"heap_size_young_generation,omitempty"`
}

func IsCassandra3(version string) bool {
	return strings.HasPrefix(version, "3.")
}

func isCassandra4(version string) bool {
	return strings.HasPrefix(version, "4.")
}

func (c config) MarshalJSON() ([]byte, error) {
	config := make(map[string]interface{})

	if isCassandra4(c.cassandraVersion) {
		c.StartRpc = nil
		c.ThriftPreparedStatementCacheSizeMb = nil
	}

	// Even though we default to Cassandra's stock defaults for num_tokens, we need to
	// explicitly set it because the config builder defaults to num_tokens: 1
	if c.NumTokens == nil {
		if isCassandra4(c.cassandraVersion) {
			c.NumTokens = pointer.Int(16)
		} else {
			c.NumTokens = pointer.Int(256)
		}
	}

	config["cassandra-yaml"] = c.CassandraYaml

	if c.jvmOptions.InitialHeapSize != nil || c.jvmOptions.MaxHeapSize != nil || c.jvmOptions.HeapNewGenSize != nil {
		if isCassandra4(c.cassandraVersion) {
			config["jvm-server-options"] = c.jvmOptions
		} else {
			config["jvm-options"] = c.jvmOptions
		}
	}

	// Many config-builder templates accept a parameter called "additional-jvm-opts", e.g the jvm-options or the
	// cassandra-env-sh templates; it is safer to include this parameter in cassandra-env-sh, so that we can guarantee
	// that our options will be applied last and will override whatever options were specified by default.
	if c.additionalJvmOptions != nil {
		config["cassandra-env-sh"] = map[string]interface{}{
			"additional-jvm-opts": c.additionalJvmOptions,
		}
	}

	return json.Marshal(&config)
}

func newConfig(apiConfig api.CassandraConfig, cassandraVersion string) config {
	cfg := config{CassandraYaml: apiConfig.CassandraYaml, cassandraVersion: cassandraVersion}

	if apiConfig.JvmOptions.HeapSize != nil {
		heapSize := apiConfig.JvmOptions.HeapSize.Value()
		cfg.jvmOptions.InitialHeapSize = &heapSize
		cfg.jvmOptions.MaxHeapSize = &heapSize
	}

	if apiConfig.JvmOptions.HeapNewGenSize != nil {
		newGenSize := apiConfig.JvmOptions.HeapNewGenSize.Value()
		cfg.jvmOptions.HeapNewGenSize = &newGenSize
	}

	cfg.additionalJvmOptions = apiConfig.JvmOptions.AdditionalOptions

	return cfg
}

// ApplySystemReplication adds system properties to configure replication of system
// keyspaces.
func ApplySystemReplication(dcConfig *DatacenterConfig, replication SystemReplication) {
	dcNames := SystemReplicationDcNames + "=" + strings.Join(replication.Datacenters, ",")
	replicationFactor := SystemReplicationFactor + "=" + strconv.Itoa(replication.ReplicationFactor)
	// prepend instead of append, so that user-specified options take precedence
	dcConfig.CassandraConfig.JvmOptions.AdditionalOptions = append(
		[]string{dcNames, replicationFactor},
		dcConfig.CassandraConfig.JvmOptions.AdditionalOptions...,
	)
}

func AllowAlterRfDuringRangeMovement(dcConfig *DatacenterConfig) {
	// prepend instead of append, so that user-specified options take precedence
	dcConfig.CassandraConfig.JvmOptions.AdditionalOptions = append(
		[]string{allowAlterRf},
		dcConfig.CassandraConfig.JvmOptions.AdditionalOptions...,
	)
}

// CreateJsonConfig parses dcConfig into a raw JSON base64-encoded string. If config is nil
// then nil, nil is returned
func CreateJsonConfig(config api.CassandraConfig, cassandraVersion string) ([]byte, error) {
	cfg := newConfig(config, cassandraVersion)
	return json.Marshal(cfg)
}
