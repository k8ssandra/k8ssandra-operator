package cassandra

import (
	"encoding/json"
	"github.com/Jeffail/gabs"
	"strings"
)

type NodeConfig map[string]interface{}

func getOperatorSuppliedConfig(dcs []string, replicationFactor int) NodeConfig {
	return NodeConfig{
		"jvm-options": NodeConfig{
			"additional-jvm-options": NodeConfig{
				"-Dcassandra.system_distributed_replication_dc_names": strings.Join(dcs, ","),
				"-Dcassandra.system_distributed_replication_per_dc": replicationFactor,
			},
		},
	}
}

func GetMergedConfig(config []byte, dcs []string, replicationFactor int) ([]byte, error) {
	operatorValues := getOperatorSuppliedConfig(dcs, replicationFactor)
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
