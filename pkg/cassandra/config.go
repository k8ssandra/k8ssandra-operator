package cassandra

import (
	"encoding/json"
	"github.com/Jeffail/gabs"
<<<<<<< HEAD
	"strconv"
=======
>>>>>>> 1c51550 (first pass at configuring replication)
	"strings"
)

type NodeConfig map[string]interface{}

<<<<<<< HEAD
func getOperatorSuppliedConfig(dcs []string, replicationFactor int, cassandraVersion string) NodeConfig {
	jvmOpts := "jvm-server-options"
	if strings.HasPrefix(cassandraVersion, "3.") {
		jvmOpts = "jvm-options"
	}
	return NodeConfig{
		jvmOpts: NodeConfig{
			"additional-jvm-opts": []string{
				"-Dcassandra.system_distributed_replication_dc_names=" + strings.Join(dcs, ","),
				"-Dcassandra.system_distributed_replication_per_dc=" + strconv.Itoa(replicationFactor),
=======
func getOperatorSuppliedConfig(dcs []string) NodeConfig {
	return NodeConfig{
		"jvm-options": NodeConfig{
			"additional-jvm-options": NodeConfig{
				"-Dcassandra.system_distributed_replication_dc_names": strings.Join(dcs, ","),
>>>>>>> 1c51550 (first pass at configuring replication)
			},
		},
	}
}

<<<<<<< HEAD
func GetMergedConfig(config []byte, dcs []string, replicationFactor int, cassandraVersion string) ([]byte, error) {
	operatorValues := getOperatorSuppliedConfig(dcs, replicationFactor, cassandraVersion)
=======
func GetMergedConfig(config []byte, dcs []string) ([]byte, error) {
	operatorValues := getOperatorSuppliedConfig(dcs)
>>>>>>> 1c51550 (first pass at configuring replication)
	operatorBytes, err := json.Marshal(operatorValues)
	if err != nil {
		return nil, err
	}

	operatorParsedConfig, err := gabs.ParseJSON(operatorBytes)
	if err != nil {
		return nil, err
	}

	parsedConfig, err := gabs.ParseJSON(config)
	if err != nil {
		return nil, err
	}

	if err = operatorParsedConfig.Merge(parsedConfig); err != nil {
		return nil, err
	}

	return operatorParsedConfig.Bytes(), nil
}
