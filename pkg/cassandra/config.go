package cassandra

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"k8s.io/utils/pointer"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
)

const (
	SystemReplicationFactorStrategy = "-Dcassandra.system_distributed_replication"
	allowAlterRf                    = "-Dcassandra.allow_alter_rf_during_range_movement=true"
)

// CreateJsonConfig parses a DatacenterConfig into a raw JSON base64-encoded string as required by
// the CassandraDatacenter.Spec.Config field, which is processed by cass-config-builder.
func CreateJsonConfig(apiConfig *DatacenterConfig) ([]byte, error) {
	cfg := &cassConfig{
		CassandraConfig: apiConfig.CassandraConfig.DeepCopy(),
	}
	addNumTokens(apiConfig, cfg)
	addEncryptionOptions(apiConfig, cfg)
	handleDeprecatedJvmOptions(cfg)
	if out, err := preMarshalConfig(reflect.ValueOf(cfg), apiConfig.ServerVersion, string(apiConfig.ServerType)); err != nil {
		return nil, err
	} else {
		return json.Marshal(out)
	}
}

// cassConfig is an internal type that is semantically equivalent to an api.CassandraConfig object,
// with the addition of a few hidden fields that the user cannot set directly.
type cassConfig struct {
	*api.CassandraConfig   `cass-config:";recurse"`
	StartRpc               bool              `cass-config:"^3.11.x:cassandra-yaml/start_rpc;retainzero"`
	ServerEncryptionStores *encryptionStores `cass-config:"cassandra-yaml/server_encryption_options;recurse"`
	ClientEncryptionStores *encryptionStores `cass-config:"cassandra-yaml/client_encryption_options;recurse"`
}

type encryptionStores struct {
	Keystore           string `cass-config:"keystore"`
	KeystorePassword   string `cass-config:"keystore_password"`
	Truststore         string `cass-config:"truststore"`
	TruststorePassword string `cass-config:"truststore_password"`
}

func addNumTokens(template *DatacenterConfig, cfg *cassConfig) {
	// Even though we default to Cassandra's stock defaults for num_tokens, we need to
	// explicitly set it because the config builder defaults to num_tokens: 1
	if cfg.CassandraYaml.NumTokens == nil {
		version := template.ServerVersion
		if template.ServerType == api.ServerDistributionCassandra && version.Major() == 3 {
			cfg.CassandraYaml.NumTokens = pointer.Int(256)
		} else {
			cfg.CassandraYaml.NumTokens = pointer.Int(16)
		}
	}
}

func addEncryptionOptions(template *DatacenterConfig, cfg *cassConfig) {
	if ClientEncryptionEnabled(template) {
		keystorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameKeystore), encryption.StoreNameKeystore)
		truststorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameTruststore), encryption.StoreNameTruststore)
		cfg.ClientEncryptionStores = &encryptionStores{
			Keystore:           keystorePath,
			Truststore:         truststorePath,
			KeystorePassword:   template.ClientKeystorePassword,
			TruststorePassword: template.ClientTruststorePassword,
		}
	}
	if ServerEncryptionEnabled(template) {
		keystorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeServer, encryption.StoreNameKeystore), encryption.StoreNameKeystore)
		truststorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeServer, encryption.StoreNameTruststore), encryption.StoreNameTruststore)
		cfg.ServerEncryptionStores = &encryptionStores{
			Keystore:           keystorePath,
			Truststore:         truststorePath,
			KeystorePassword:   template.ServerKeystorePassword,
			TruststorePassword: template.ServerTruststorePassword,
		}
	}
}

// Handles the deprecated settings: HeapSize and HeapNewGenSize by copying their values, if any,
// to the appropriate destination settings, iif these are nil.
//goland:noinspection GoDeprecation
func handleDeprecatedJvmOptions(cfg *cassConfig) {
	// Transfer the global heap size to specific keys
	if cfg.JvmOptions.HeapSize != nil {
		if cfg.JvmOptions.InitialHeapSize == nil {
			cfg.JvmOptions.InitialHeapSize = cfg.JvmOptions.HeapSize
		}
		if cfg.JvmOptions.MaxHeapSize == nil {
			cfg.JvmOptions.MaxHeapSize = cfg.JvmOptions.HeapSize
		}
	}
	// Transfer HeapNewGenSize
	if cfg.JvmOptions.HeapNewGenSize != nil {
		if cfg.JvmOptions.CmsHeapSizeYoungGeneration == nil {
			cfg.JvmOptions.CmsHeapSizeYoungGeneration = cfg.JvmOptions.HeapNewGenSize
		}
	}
}

// Some settings in Cassandra are using a float type, which isn't supported for CRDs.
// They were changed to use a string type, and we validate here that if set they can parse correctly to float.
// FIXME turn these validations into kubebuilder markup and enforce a pattern for floating point numbers and/or use int or resource.Quantity
func validateCassandraYaml(cassandraYaml *api.CassandraYaml) error {
	if cassandraYaml.CommitlogSyncBatchWindowInMs != nil {
		if _, err := strconv.ParseFloat(*cassandraYaml.CommitlogSyncBatchWindowInMs, 64); err != nil {
			return fmt.Errorf("CommitlogSyncBatchWindowInMs must be a valid float: %v", err)
		}
	}

	if cassandraYaml.DiskOptimizationEstimatePercentile != nil {
		if _, err := strconv.ParseFloat(*cassandraYaml.DiskOptimizationEstimatePercentile, 64); err != nil {
			return fmt.Errorf("DiskOptimizationEstimatePercentile must be a valid float: %v", err)
		}
	}

	if cassandraYaml.DynamicSnitchBadnessThreshold != nil {
		if _, err := strconv.ParseFloat(*cassandraYaml.DynamicSnitchBadnessThreshold, 64); err != nil {
			return fmt.Errorf("DynamicSnitchBadnessThreshold must be a valid float: %v", err)
		}
	}

	if cassandraYaml.MemtableCleanupThreshold != nil {
		if _, err := strconv.ParseFloat(*cassandraYaml.MemtableCleanupThreshold, 64); err != nil {
			return fmt.Errorf("MemtableCleanupThreshold must be a valid float: %v", err)
		}
	}

	if cassandraYaml.PhiConvictThreshold != nil {
		if _, err := strconv.ParseFloat(*cassandraYaml.PhiConvictThreshold, 64); err != nil {
			return fmt.Errorf("PhiConvictThreshold must be a valid float: %v", err)
		}
	}

	if cassandraYaml.RangeTombstoneListGrowthFactor != nil {
		if _, err := strconv.ParseFloat(*cassandraYaml.RangeTombstoneListGrowthFactor, 64); err != nil {
			return fmt.Errorf("RangeTombstoneListGrowthFactor must be a valid float: %v", err)
		}
	}

	if cassandraYaml.CommitlogSyncPeriodInMs != nil && cassandraYaml.CommitlogSyncBatchWindowInMs != nil {
		return fmt.Errorf("CommitlogSyncPeriodInMs and CommitlogSyncBatchWindowInMs are mutually exclusive")
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

	// prepend instead of append, so that user-specified options take precedence
	dcConfig.CassandraConfig.JvmOptions.AdditionalOptions = append(
		[]string{replicationStrategy},
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
