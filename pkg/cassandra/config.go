package cassandra

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
)

const (
	SystemReplicationFactorStrategy = "-Dcassandra.system_distributed_replication"
	allowAlterRf                    = "-Dcassandra.allow_alter_rf_during_range_movement=true"
	disableNodeSyncOption           = "-Ddse.nodesync.disable_on_new_tables=true"
)

// createJsonConfig parses a CassandraConfig into raw JSON bytes as required by the
// CassandraDatacenter.Spec.Config field, which is processed by cass-config-builder.
func createJsonConfig(config api.CassandraConfig, serverVersion *semver.Version, serverType api.ServerDistribution) ([]byte, error) {

	out := make(unstructured.Unstructured)

	// cassandra.yaml is an unstructured map, we simply append it to the output
	// TODO validate cassandra.yaml settings with JSON schemas
	// TODO postprocess cassandra.yaml settings, e.g. mountable volumes or resource.Quantity syntax
	if len(config.CassandraYaml) > 0 {
		out["cassandra-yaml"] = config.CassandraYaml
	}

	// dse.yaml is an unstructured map, we simply append it to the output
	// TODO validate dse.yaml settings with JSON schemas
	// TODO postprocess dse.yaml settings, e.g. mountable volumes or resource.Quantity syntax
	if len(config.DseYaml) > 0 {
		out["dse-yaml"] = config.DseYaml
	}

	// JvmOptions is a struct, we need to convert it to a map using preMarshalConfig
	jvmOptionsVal := reflect.ValueOf(config.JvmOptions)
	jvmOptionsOut, err := preMarshalConfig(jvmOptionsVal, serverVersion, string(serverType))
	if err != nil {
		return nil, err
	}
	out.PutAll(jvmOptionsOut)

	return json.Marshal(out)
}

func DisableNodeSync(dcConfig *DatacenterConfig) {
	addOptionIfMissing(dcConfig, disableNodeSyncOption)
}

// AddNumTokens adds the num_tokens option to cassandra.yaml if it is not already present, because
// otherwise Cassandra defaults to num_tokens: 1, which is not recommended.
func AddNumTokens(template *DatacenterConfig) {
	// Note: we put int64 values because even if int values can be marshaled just fine,
	// Unstructured.DeepCopy() would reject them since int is not a supported json type.
	if template.ServerType == api.ServerDistributionCassandra && template.ServerVersion.Major() == 3 {
		template.CassandraConfig.CassandraYaml.PutIfAbsent("num_tokens", int64(256))
	} else {
		template.CassandraConfig.CassandraYaml.PutIfAbsent("num_tokens", int64(16))
	}
}

// AddStartRpc adds the start_rpc option to cassandra.yaml, but only if Cassandra is 3.x.
func AddStartRpc(template *DatacenterConfig) {
	if template.ServerType == api.ServerDistributionCassandra && template.ServerVersion.Major() == 3 {
		template.CassandraConfig.CassandraYaml.PutIfAbsent("start_rpc", false)
	}
}

// HandleDeprecatedJvmOptions handles the deprecated settings: HeapSize and HeapNewGenSize by
// copying their values, if any, to the appropriate destination settings, iif these are nil.
//
//goland:noinspection GoDeprecation
func HandleDeprecatedJvmOptions(jvmOptions *api.JvmOptions) {
	// Transfer the global heap size to specific keys
	if jvmOptions.HeapSize != nil {
		if jvmOptions.InitialHeapSize == nil {
			jvmOptions.InitialHeapSize = jvmOptions.HeapSize
		}
		if jvmOptions.MaxHeapSize == nil {
			jvmOptions.MaxHeapSize = jvmOptions.HeapSize
		}
	}
	// Transfer HeapNewGenSize
	if jvmOptions.HeapNewGenSize != nil {
		if jvmOptions.CmsHeapSizeYoungGeneration == nil {
			jvmOptions.CmsHeapSizeYoungGeneration = jvmOptions.HeapNewGenSize
		}
	}
}

// validateCassandraYaml provides semantic validation for cassandra.yaml settings.
// TODO this is a relic of the structured YAML approach. Only the bits that are still relevant were ported over.
// From now on, all syntactic validation is expected to happen externally using JSON schemas.
// Do not use this function to validate the syntax of cassandra.yaml settings. Use ONLY to validate
// any semantics that cannot be validated with JSON schemas.
func validateCassandraYaml(cassandraYaml unstructured.Unstructured) error {
	commitLogSync := cassandraYaml["commitlog_sync_period_in_ms"]
	commitLogSyncBatch := cassandraYaml["commitlog_sync_batch_window_in_ms"]
	if commitLogSync != nil && commitLogSyncBatch != nil {
		return fmt.Errorf("commitlog_sync_period_in_ms and commitlog_sync_batch_window_in_ms are mutually exclusive")
	}
	return nil
}

// ApplySystemReplication adds system properties to configure replication of system
// keyspaces.
func ApplySystemReplication(dcConfig *DatacenterConfig, replication SystemReplication) {
	replicationFactors := make([]string, 0, len(replication))
	dcs := make([]string, 0, len(replication))

	// Sort to make verification in tests easier.
	for k := range replication {
		dcs = append(dcs, k)
	}
	sort.Strings(dcs)

	for _, dc := range dcs {
		replicationFactors = append(replicationFactors, fmt.Sprintf("%s:%d", dc, replication[dc]))
	}
	replicationStrategy := SystemReplicationFactorStrategy + "=" + strings.Join(replicationFactors, ",")
	addOptionIfMissing(dcConfig, replicationStrategy)
}

func AllowAlterRfDuringRangeMovement(dcConfig *DatacenterConfig) {
	addOptionIfMissing(dcConfig, allowAlterRf)
}

// EnableSmartTokenAllocation adds the allocate_tokens_for_local_replication_factor option to
// cassandra.yaml if it is not already present when running DSE.
// This option is enabled by default in Cassandra but not DSE.
func EnableSmartTokenAllocation(template *DatacenterConfig) {
	// Note: we put int64 values because even if int values can be marshaled just fine,
	// Unstructured.DeepCopy() would reject them since int is not a supported json type.
	if template.ServerType == api.ServerDistributionDse {
		template.CassandraConfig.CassandraYaml.PutIfAbsent("allocate_tokens_for_local_replication_factor", int64(3))
	}
}
