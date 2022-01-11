package cassandra

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/utils/pointer"

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

func newConfig(apiConfig api.CassandraConfig, cassandraVersion string) (config, error) {
	// Filters out config element which do not exist in the Cassandra version in use
	filteredConfig := apiConfig.DeepCopy()
	filterConfigForVersion(cassandraVersion, filteredConfig)
	cfg := config{CassandraYaml: filteredConfig.CassandraYaml, cassandraVersion: cassandraVersion}
	err := validateConfig(filteredConfig)
	if err != nil {
		return cfg, err
	}

	if filteredConfig.JvmOptions.HeapSize != nil {
		heapSize := filteredConfig.JvmOptions.HeapSize.Value()
		cfg.jvmOptions.InitialHeapSize = &heapSize
		cfg.jvmOptions.MaxHeapSize = &heapSize
	}

	if filteredConfig.JvmOptions.HeapNewGenSize != nil {
		newGenSize := filteredConfig.JvmOptions.HeapNewGenSize.Value()
		cfg.jvmOptions.HeapNewGenSize = &newGenSize
	}

	cfg.additionalJvmOptions = filteredConfig.JvmOptions.AdditionalOptions

	return cfg, nil
}

// Some settings in Cassandra are using a float type, which isn't supported for CRDs.
// They were changed to use a string type, and we validate here that if set they can parse correctly to float.
func validateConfig(config *api.CassandraConfig) error {
	if config.CassandraYaml.CommitlogSyncBatchWindowInMs != nil {
		if _, err := strconv.ParseFloat(*config.CassandraYaml.CommitlogSyncBatchWindowInMs, 64); err != nil {
			return fmt.Errorf("CommitlogSyncBatchWindowInMs must be a valid float: %v", err)
		}
	}

	if config.CassandraYaml.DiskOptimizationEstimatePercentile != nil {
		if _, err := strconv.ParseFloat(*config.CassandraYaml.DiskOptimizationEstimatePercentile, 64); err != nil {
			return fmt.Errorf("DiskOptimizationEstimatePercentile must be a valid float: %v", err)
		}
	}

	if config.CassandraYaml.DynamicSnitchBadnessThreshold != nil {
		if _, err := strconv.ParseFloat(*config.CassandraYaml.DynamicSnitchBadnessThreshold, 64); err != nil {
			return fmt.Errorf("DynamicSnitchBadnessThreshold must be a valid float: %v", err)
		}
	}

	if config.CassandraYaml.MemtableCleanupThreshold != nil {
		if _, err := strconv.ParseFloat(*config.CassandraYaml.MemtableCleanupThreshold, 64); err != nil {
			return fmt.Errorf("MemtableCleanupThreshold must be a valid float: %v", err)
		}
	}

	if config.CassandraYaml.PhiConvictThreshold != nil {
		if _, err := strconv.ParseFloat(*config.CassandraYaml.PhiConvictThreshold, 64); err != nil {
			return fmt.Errorf("PhiConvictThreshold must be a valid float: %v", err)
		}
	}

	if config.CassandraYaml.RangeTombstoneListGrowthFactor != nil {
		if _, err := strconv.ParseFloat(*config.CassandraYaml.RangeTombstoneListGrowthFactor, 64); err != nil {
			return fmt.Errorf("RangeTombstoneListGrowthFactor must be a valid float: %v", err)
		}
	}
	return nil
}

// Filters out config element which do not exist in the Cassandra version in use
// Generated using the filter columns in the first sheet of https://docs.google.com/spreadsheets/d/1P0bw5avkppBnoLXY00qVQmntgx6UJQbUidZtHfCRp_c/edit?usp=sharing
func filterConfigForVersion(cassandraVersion string, filteredConfig *api.CassandraConfig) {
	if IsCassandra3(cassandraVersion) {
		filteredConfig.CassandraYaml.AllocateTokensForLocalReplicationFactor = nil
		filteredConfig.CassandraYaml.AuditLoggingOptions = nil
		filteredConfig.CassandraYaml.AuthReadConsistencyLevel = nil
		filteredConfig.CassandraYaml.AuthWriteConsistencyLevel = nil
		filteredConfig.CassandraYaml.AutoHintsCleanupEnabled = nil
		filteredConfig.CassandraYaml.AutoOptimiseFullRepairStreams = nil
		filteredConfig.CassandraYaml.AutoOptimiseIncRepairStreams = nil
		filteredConfig.CassandraYaml.AutoOptimisePreviewRepairStreams = nil
		filteredConfig.CassandraYaml.AutocompactionOnStartupEnabled = nil
		filteredConfig.CassandraYaml.AutomaticSstableUpgrade = nil
		filteredConfig.CassandraYaml.AvailableProcessors = nil
		filteredConfig.CassandraYaml.BlockForPeersInRemoteDcs = nil
		filteredConfig.CassandraYaml.BlockForPeersTimeoutInSecs = nil
		filteredConfig.CassandraYaml.ClientErrorReportingExclusions = nil
		filteredConfig.CassandraYaml.CommitlogSyncGroupWindowInMs = nil
		filteredConfig.CassandraYaml.CompactionTombstoneWarningThreshold = nil
		filteredConfig.CassandraYaml.ConcurrentMaterializedViewBuilders = nil
		filteredConfig.CassandraYaml.ConcurrentValidations = nil
		filteredConfig.CassandraYaml.ConsecutiveMessageErrorsThreshold = nil
		filteredConfig.CassandraYaml.CorruptedTombstoneStrategy = nil
		filteredConfig.CassandraYaml.DefaultKeyspaceRf = nil
		filteredConfig.CassandraYaml.DenylistConsistencyLevel = nil
		filteredConfig.CassandraYaml.DenylistInitialLoadRetrySeconds = nil
		filteredConfig.CassandraYaml.DenylistMaxKeysPerTable = nil
		filteredConfig.CassandraYaml.DenylistMaxKeysTotal = nil
		filteredConfig.CassandraYaml.DenylistRefreshSeconds = nil
		filteredConfig.CassandraYaml.DiagnosticEventsEnabled = nil
		filteredConfig.CassandraYaml.EnableDenylistRangeReads = nil
		filteredConfig.CassandraYaml.EnableDenylistReads = nil
		filteredConfig.CassandraYaml.EnableDenylistWrites = nil
		filteredConfig.CassandraYaml.EnablePartitionDenylist = nil
		filteredConfig.CassandraYaml.EnableTransientReplication = nil
		filteredConfig.CassandraYaml.FailureDetector = nil
		filteredConfig.CassandraYaml.FileCacheEnabled = nil
		filteredConfig.CassandraYaml.FlushCompression = nil
		filteredConfig.CassandraYaml.FullQueryLoggingOptions = nil
		filteredConfig.CassandraYaml.HintWindowPersistentEnabled = nil
		filteredConfig.CassandraYaml.IdealConsistencyLevel = nil
		filteredConfig.CassandraYaml.InitialRangeTombstoneListAllocationSize = nil
		filteredConfig.CassandraYaml.InternodeApplicationReceiveQueueCapacityInBytes = nil
		filteredConfig.CassandraYaml.InternodeApplicationReceiveQueueReserveEndpointCapacityInBytes = nil
		filteredConfig.CassandraYaml.InternodeApplicationReceiveQueueReserveGlobalCapacityInBytes = nil
		filteredConfig.CassandraYaml.InternodeApplicationSendQueueCapacityInBytes = nil
		filteredConfig.CassandraYaml.InternodeApplicationSendQueueReserveEndpointCapacityInBytes = nil
		filteredConfig.CassandraYaml.InternodeApplicationSendQueueReserveGlobalCapacityInBytes = nil
		filteredConfig.CassandraYaml.InternodeErrorReportingExclusions = nil
		filteredConfig.CassandraYaml.InternodeMaxMessageSizeInBytes = nil
		filteredConfig.CassandraYaml.InternodeSocketReceiveBufferSizeInBytes = nil
		filteredConfig.CassandraYaml.InternodeSocketSendBufferSizeInBytes = nil
		filteredConfig.CassandraYaml.InternodeStreamingTcpUserTimeoutInMs = nil
		filteredConfig.CassandraYaml.InternodeTcpConnectTimeoutInMs = nil
		filteredConfig.CassandraYaml.InternodeTcpUserTimeoutInMs = nil
		filteredConfig.CassandraYaml.KeyCacheMigrateDuringCompaction = nil
		filteredConfig.CassandraYaml.KeyspaceCountWarnThreshold = nil
		filteredConfig.CassandraYaml.MaxConcurrentAutomaticSstableUpgrades = nil
		filteredConfig.CassandraYaml.MinimumKeyspaceRf = nil
		filteredConfig.CassandraYaml.NativeTransportAllowOlderProtocols = nil
		filteredConfig.CassandraYaml.NativeTransportIdleTimeoutInMs = nil
		filteredConfig.CassandraYaml.NativeTransportMaxRequestsPerSecond = nil
		filteredConfig.CassandraYaml.NativeTransportRateLimitingEnabled = nil
		filteredConfig.CassandraYaml.NativeTransportReceiveQueueCapacityInBytes = nil
		filteredConfig.CassandraYaml.NetworkAuthorizer = nil
		filteredConfig.CassandraYaml.NetworkingCacheSizeInMb = nil
		filteredConfig.CassandraYaml.PaxosCacheSizeInMb = nil
		filteredConfig.CassandraYaml.PeriodicCommitlogSyncLagBlockInMs = nil
		filteredConfig.CassandraYaml.RangeTombstoneListGrowthFactor = nil
		filteredConfig.CassandraYaml.RejectRepairCompactionThreshold = nil
		filteredConfig.CassandraYaml.RepairCommandPoolFullStrategy = nil
		filteredConfig.CassandraYaml.RepairCommandPoolSize = nil
		filteredConfig.CassandraYaml.RepairSessionSpaceInMb = nil
		filteredConfig.CassandraYaml.RepairedDataTrackingForPartitionReadsEnabled = nil
		filteredConfig.CassandraYaml.RepairedDataTrackingForRangeReadsEnabled = nil
		filteredConfig.CassandraYaml.ReportUnconfirmedRepairedDataMismatches = nil
		filteredConfig.CassandraYaml.SnapshotLinksPerSecond = nil
		filteredConfig.CassandraYaml.SnapshotOnRepairedDataMismatch = nil
		filteredConfig.CassandraYaml.StreamEntireSstables = nil
		filteredConfig.CassandraYaml.StreamingConnectionsPerHost = nil
		filteredConfig.CassandraYaml.TableCountWarnThreshold = nil
		filteredConfig.CassandraYaml.TrackWarnings = nil
		filteredConfig.CassandraYaml.TraverseAuthFromRoot = nil
		filteredConfig.CassandraYaml.UseDeterministicTableId = nil
		filteredConfig.CassandraYaml.UseOffheapMerkleTrees = nil
		filteredConfig.CassandraYaml.ValidationPreviewPurgeHeadStartInSec = nil
	}
	if isCassandra4(cassandraVersion) {
		filteredConfig.CassandraYaml.AuthReadConsistencyLevel = nil
		filteredConfig.CassandraYaml.AuthWriteConsistencyLevel = nil
		filteredConfig.CassandraYaml.AutoHintsCleanupEnabled = nil
		filteredConfig.CassandraYaml.AvailableProcessors = nil
		filteredConfig.CassandraYaml.ClientErrorReportingExclusions = nil
		filteredConfig.CassandraYaml.CompactionTombstoneWarningThreshold = nil
		filteredConfig.CassandraYaml.DefaultKeyspaceRf = nil
		filteredConfig.CassandraYaml.DenylistConsistencyLevel = nil
		filteredConfig.CassandraYaml.DenylistInitialLoadRetrySeconds = nil
		filteredConfig.CassandraYaml.DenylistMaxKeysPerTable = nil
		filteredConfig.CassandraYaml.DenylistMaxKeysTotal = nil
		filteredConfig.CassandraYaml.DenylistRefreshSeconds = nil
		filteredConfig.CassandraYaml.EnableDenylistRangeReads = nil
		filteredConfig.CassandraYaml.EnableDenylistReads = nil
		filteredConfig.CassandraYaml.EnableDenylistWrites = nil
		filteredConfig.CassandraYaml.EnablePartitionDenylist = nil
		filteredConfig.CassandraYaml.FailureDetector = nil
		filteredConfig.CassandraYaml.HintWindowPersistentEnabled = nil
		filteredConfig.CassandraYaml.IndexInterval = nil
		filteredConfig.CassandraYaml.InternodeErrorReportingExclusions = nil
		filteredConfig.CassandraYaml.InternodeRecvBuffSizeInBytes = nil
		filteredConfig.CassandraYaml.InternodeSendBuffSizeInBytes = nil
		filteredConfig.CassandraYaml.MinimumKeyspaceRf = nil
		filteredConfig.CassandraYaml.NativeTransportMaxRequestsPerSecond = nil
		filteredConfig.CassandraYaml.NativeTransportRateLimitingEnabled = nil
		filteredConfig.CassandraYaml.OtcBacklogExpirationIntervalMs = nil
		filteredConfig.CassandraYaml.PaxosCacheSizeInMb = nil
		filteredConfig.CassandraYaml.RequestScheduler = nil
		filteredConfig.CassandraYaml.RequestSchedulerId = nil
		filteredConfig.CassandraYaml.RequestSchedulerOptions = nil
		filteredConfig.CassandraYaml.RpcMaxThreads = nil
		filteredConfig.CassandraYaml.RpcMinThreads = nil
		filteredConfig.CassandraYaml.RpcRecvBuffSizeInBytes = nil
		filteredConfig.CassandraYaml.RpcSendBuffSizeInBytes = nil
		filteredConfig.CassandraYaml.StreamingSocketTimeoutInMs = nil
		filteredConfig.CassandraYaml.ThriftFramedTransportSizeInMb = nil
		filteredConfig.CassandraYaml.ThriftMaxMessageLengthInMb = nil
		filteredConfig.CassandraYaml.ThriftPreparedStatementsCacheSizeMb = nil
		filteredConfig.CassandraYaml.TrackWarnings = nil
		filteredConfig.CassandraYaml.TraverseAuthFromRoot = nil
		filteredConfig.CassandraYaml.UseDeterministicTableId = nil
	}
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
	cfg, err := newConfig(config, cassandraVersion)
	if err != nil {
		return nil, err
	}
	return json.Marshal(cfg)
}
