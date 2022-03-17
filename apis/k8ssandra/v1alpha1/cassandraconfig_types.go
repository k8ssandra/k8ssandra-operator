/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"k8s.io/apimachinery/pkg/api/resource"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type CassandraConfig struct {
	// +optional
	CassandraYaml CassandraYaml `json:"cassandraYaml,omitempty" cass-config:"*:cassandra-yaml;recurse"`

	// +optional
	JvmOptions JvmOptions `json:"jvmOptions,omitempty" cass-config:"*:;recurse"`
}

// CassandraYaml defines the contents of the cassandra.yaml file. For more info see:
// https://cassandra.apache.org/doc/latest/cassandra/configuration/cass_yaml_file.html
type CassandraYaml struct {
	// Exists in 3.11, 4.0, trunk
	// +optional
	AllocateTokensForKeyspace *string `json:"allocate_tokens_for_keyspace,omitempty" cass-config:"*:allocate_tokens_for_keyspace"`

	// Exists in: 4.0, trunk
	// +optional
	AllocateTokensForLocalReplicationFactor *int `json:"allocate_tokens_for_local_replication_factor,omitempty" cass-config:">=4.x:allocate_tokens_for_local_replication_factor"`

	// Exists in: 4.0, trunk
	// +optional
	AuditLoggingOptions *AuditLogOptions `json:"audit_logging_options,omitempty" cass-config:">=4.x:audit_logging_options;recurse"`

	// Exists in trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	AuthReadConsistencyLevel *string `json:"auth_read_consistency_level,omitempty" cass-config:">=4.1.x:auth_read_consistency_level"`

	// Exists in trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	AuthWriteConsistencyLevel *string `json:"auth_write_consistency_level,omitempty" cass-config:">=4.1.x:auth_write_consistency_level"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	Authenticator *string `json:"authenticator,omitempty" cass-config:"*:authenticator"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	Authorizer *string `json:"authorizer,omitempty" cass-config:"*:authorizer"`

	// Exists in trunk
	// +optional
	AutoHintsCleanupEnabled *bool `json:"auto_hints_cleanup_enabled,omitempty" cass-config:">=4.1.x:auto_hints_cleanup_enabled"`

	// Exists in: 4.0, trunk
	// +optional
	AutoOptimiseFullRepairStreams *bool `json:"auto_optimise_full_repair_streams,omitempty" cass-config:">=4.x:auto_optimise_full_repair_streams"`

	// Exists in: 4.0, trunk
	// +optional
	AutoOptimiseIncRepairStreams *bool `json:"auto_optimise_inc_repair_streams,omitempty" cass-config:">=4.x:auto_optimise_inc_repair_streams"`

	// Exists in: 4.0, trunk
	// +optional
	AutoOptimisePreviewRepairStreams *bool `json:"auto_optimise_preview_repair_streams,omitempty" cass-config:">=4.x:auto_optimise_preview_repair_streams"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	AutoSnapshot *bool `json:"auto_snapshot,omitempty" cass-config:"*:auto_snapshot"`

	// Exists in: 4.0, trunk
	// +optional
	AutocompactionOnStartupEnabled *bool `json:"autocompaction_on_startup_enabled,omitempty" cass-config:">=4.x:autocompaction_on_startup_enabled"`

	// Exists in: 4.0, trunk
	// +optional
	AutomaticSstableUpgrade *bool `json:"automatic_sstable_upgrade,omitempty" cass-config:">=4.x:automatic_sstable_upgrade"`

	// Exists in trunk
	// +optional
	AvailableProcessors *int `json:"available_processors,omitempty" cass-config:">=4.1.x:available_processors"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BackPressureEnabled *bool `json:"back_pressure_enabled,omitempty" cass-config:"*:back_pressure_enabled"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BackPressureStrategy *ParameterizedClass `json:"back_pressure_strategy,omitempty" cass-config:"*:back_pressure_strategy;recurse"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BatchSizeFailThresholdInKb *int `json:"batch_size_fail_threshold_in_kb,omitempty" cass-config:"*:batch_size_fail_threshold_in_kb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BatchSizeWarnThresholdInKb *int `json:"batch_size_warn_threshold_in_kb,omitempty" cass-config:"*:batch_size_warn_threshold_in_kb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BatchlogReplayThrottleInKb *int `json:"batchlog_replay_throttle_in_kb,omitempty" cass-config:"*:batchlog_replay_throttle_in_kb"`

	// Exists in: 4.0, trunk
	// +optional
	BlockForPeersInRemoteDcs *bool `json:"block_for_peers_in_remote_dcs,omitempty" cass-config:">=4.x:block_for_peers_in_remote_dcs"`

	// Exists in: 4.0, trunk
	// +optional
	BlockForPeersTimeoutInSecs *int `json:"block_for_peers_timeout_in_secs,omitempty" cass-config:">=4.x:block_for_peers_timeout_in_secs"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BufferPoolUseHeapIfExhausted *bool `json:"buffer_pool_use_heap_if_exhausted,omitempty" cass-config:"*:buffer_pool_use_heap_if_exhausted"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CasContentionTimeoutInMs *int `json:"cas_contention_timeout_in_ms,omitempty" cass-config:"*:cas_contention_timeout_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CdcEnabled *bool `json:"cdc_enabled,omitempty" cass-config:"*:cdc_enabled"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CdcFreeSpaceCheckIntervalMs *int `json:"cdc_free_space_check_interval_ms,omitempty" cass-config:"*:cdc_free_space_check_interval_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	// TODO mountable directory
	CdcRawDirectory *string `json:"cdc_raw_directory,omitempty" cass-config:"*:cdc_raw_directory"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CdcTotalSpaceInMb *int `json:"cdc_total_space_in_mb,omitempty" cass-config:"*:cdc_total_space_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CheckForDuplicateRowsDuringCompaction *bool `json:"check_for_duplicate_rows_during_compaction,omitempty" cass-config:"*:check_for_duplicate_rows_during_compaction"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CheckForDuplicateRowsDuringReads *bool `json:"check_for_duplicate_rows_during_reads,omitempty" cass-config:"*:check_for_duplicate_rows_during_reads"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ClientEncryptionOptions *encryption.ClientEncryptionOptions `json:"client_encryption_options,omitempty" cass-config:"*:client_encryption_options;recurse"`

	// Exists in trunk
	// +optional
	ClientErrorReportingExclusions *SubnetGroups `json:"client_error_reporting_exclusions,omitempty" cass-config:">=4.1.x:client_error_reporting_exclusions;recurse"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ColumnIndexCacheSizeInKb *int `json:"column_index_cache_size_in_kb,omitempty" cass-config:"*:column_index_cache_size_in_kb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ColumnIndexSizeInKb *int `json:"column_index_size_in_kb,omitempty" cass-config:"*:column_index_size_in_kb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogCompression *ParameterizedClass `json:"commitlog_compression,omitempty" cass-config:"*:commitlog_compression;recurse"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogMaxCompressionBuffersInPool *int `json:"commitlog_max_compression_buffers_in_pool,omitempty" cass-config:"*:commitlog_max_compression_buffers_in_pool"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogPeriodicQueueSize *int `json:"commitlog_periodic_queue_size,omitempty" cass-config:"*:commitlog_periodic_queue_size"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogSegmentSizeInMb *int `json:"commitlog_segment_size_in_mb,omitempty" cass-config:"*:commitlog_segment_size_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=periodic;batch;group
	// +optional
	CommitlogSync *string `json:"commitlog_sync,omitempty" cass-config:"*:commitlog_sync"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogSyncBatchWindowInMs *string `json:"commitlog_sync_batch_window_in_ms,omitempty" cass-config:"*:commitlog_sync_batch_window_in_ms"`

	// Exists in: 4.0, trunk
	// +optional
	CommitlogSyncGroupWindowInMs *int `json:"commitlog_sync_group_window_in_ms,omitempty" cass-config:">=4.x:commitlog_sync_group_window_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogSyncPeriodInMs *int `json:"commitlog_sync_period_in_ms,omitempty" cass-config:"*:commitlog_sync_period_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogTotalSpaceInMb *int `json:"commitlog_total_space_in_mb,omitempty" cass-config:"*:commitlog_total_space_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CompactionLargePartitionWarningThresholdMb *int `json:"compaction_large_partition_warning_threshold_mb,omitempty" cass-config:"*:compaction_large_partition_warning_threshold_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CompactionThroughputMbPerSec *int `json:"compaction_throughput_mb_per_sec,omitempty" cass-config:"*:compaction_throughput_mb_per_sec"`

	// Exists in trunk
	// +optional
	CompactionTombstoneWarningThreshold *int `json:"compaction_tombstone_warning_threshold,omitempty" cass-config:">=4.1.x:compaction_tombstone_warning_threshold"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentCompactors *int `json:"concurrent_compactors,omitempty" cass-config:"*:concurrent_compactors"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentCounterWrites *int `json:"concurrent_counter_writes,omitempty" cass-config:"*:concurrent_counter_writes"`

	// Exists in: 4.0, trunk
	// +optional
	ConcurrentMaterializedViewBuilders *int `json:"concurrent_materialized_view_builders,omitempty" cass-config:">=4.x:concurrent_materialized_view_builders"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentMaterializedViewWrites *int `json:"concurrent_materialized_view_writes,omitempty" cass-config:"*:concurrent_materialized_view_writes"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentReads *int `json:"concurrent_reads,omitempty" cass-config:"*:concurrent_reads"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentReplicates *int `json:"concurrent_replicates,omitempty" cass-config:"*:concurrent_replicates"`

	// Exists in: 4.0, trunk
	// +optional
	ConcurrentValidations *int `json:"concurrent_validations,omitempty" cass-config:">=4.x:concurrent_validations"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentWrites *int `json:"concurrent_writes,omitempty" cass-config:"*:concurrent_writes"`

	// Exists in: 4.0, trunk
	// +optional
	ConsecutiveMessageErrorsThreshold *int `json:"consecutive_message_errors_threshold,omitempty" cass-config:">=4.x:consecutive_message_errors_threshold"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=disabled;warn;exception
	// +optional
	CorruptedTombstoneStrategy *string `json:"corrupted_tombstone_strategy,omitempty" cass-config:">=4.x:corrupted_tombstone_strategy"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterCacheKeysToSave *int `json:"counter_cache_keys_to_save,omitempty" cass-config:"*:counter_cache_keys_to_save"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterCacheSavePeriod *int `json:"counter_cache_save_period,omitempty" cass-config:"*:counter_cache_save_period"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterCacheSizeInMb *int `json:"counter_cache_size_in_mb,omitempty" cass-config:"*:counter_cache_size_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterWriteRequestTimeoutInMs *int `json:"counter_write_request_timeout_in_ms,omitempty" cass-config:"*:counter_write_request_timeout_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CredentialsCacheMaxEntries *int `json:"credentials_cache_max_entries,omitempty" cass-config:"*:credentials_cache_max_entries"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CredentialsUpdateIntervalInMs *int `json:"credentials_update_interval_in_ms,omitempty" cass-config:"*:credentials_update_interval_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CredentialsValidityInMs *int `json:"credentials_validity_in_ms,omitempty" cass-config:"*:credentials_validity_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CrossNodeTimeout *bool `json:"cross_node_timeout,omitempty" cass-config:"*:cross_node_timeout"`

	// Exists in trunk
	// +optional
	DefaultKeyspaceRf *int `json:"default_keyspace_rf,omitempty" cass-config:">=4.1.x:default_keyspace_rf"`

	// Exists in trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	DenylistConsistencyLevel *string `json:"denylist_consistency_level,omitempty" cass-config:">=4.1.x:denylist_consistency_level"`

	// Exists in trunk
	// +optional
	DenylistInitialLoadRetrySeconds *int `json:"denylist_initial_load_retry_seconds,omitempty" cass-config:">=4.1.x:denylist_initial_load_retry_seconds"`

	// Exists in trunk
	// +optional
	DenylistMaxKeysPerTable *int `json:"denylist_max_keys_per_table,omitempty" cass-config:">=4.1.x:denylist_max_keys_per_table"`

	// Exists in trunk
	// +optional
	DenylistMaxKeysTotal *int `json:"denylist_max_keys_total,omitempty" cass-config:">=4.1.x:denylist_max_keys_total"`

	// Exists in trunk
	// +optional
	DenylistRefreshSeconds *int `json:"denylist_refresh_seconds,omitempty" cass-config:">=4.1.x:denylist_refresh_seconds"`

	// Exists in: 4.0, trunk
	// +optional
	DiagnosticEventsEnabled *bool `json:"diagnostic_events_enabled,omitempty" cass-config:">=4.x:diagnostic_events_enabled"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=auto;mmap;mmap_index_only;standard
	// +optional
	DiskAccessMode *string `json:"disk_access_mode,omitempty" cass-config:"*:disk_access_mode"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DiskOptimizationEstimatePercentile *string `json:"disk_optimization_estimate_percentile,omitempty" cass-config:"*:disk_optimization_estimate_percentile"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DiskOptimizationPageCrossChance *string `json:"disk_optimization_page_cross_chance,omitempty" cass-config:"*:disk_optimization_page_cross_chance"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=ssd;spinning
	// +optional
	DiskOptimizationStrategy *string `json:"disk_optimization_strategy,omitempty" cass-config:"*:disk_optimization_strategy"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitch *bool `json:"dynamic_snitch,omitempty" cass-config:"*:dynamic_snitch"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitchBadnessThreshold *string `json:"dynamic_snitch_badness_threshold,omitempty" cass-config:"*:dynamic_snitch_badness_threshold"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitchResetIntervalInMs *int `json:"dynamic_snitch_reset_interval_in_ms,omitempty" cass-config:"*:dynamic_snitch_reset_interval_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitchUpdateIntervalInMs *int `json:"dynamic_snitch_update_interval_in_ms,omitempty" cass-config:"*:dynamic_snitch_update_interval_in_ms"`

	// Exists in trunk
	// +optional
	EnableDenylistRangeReads *bool `json:"enable_denylist_range_reads,omitempty" cass-config:">=4.1.x:enable_denylist_range_reads"`

	// Exists in trunk
	// +optional
	EnableDenylistReads *bool `json:"enable_denylist_reads,omitempty" cass-config:">=4.1.x:enable_denylist_reads"`

	// Exists in trunk
	// +optional
	EnableDenylistWrites *bool `json:"enable_denylist_writes,omitempty" cass-config:">=4.1.x:enable_denylist_writes"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableDropCompactStorage *bool `json:"enable_drop_compact_storage,omitempty" cass-config:"*:enable_drop_compact_storage"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableMaterializedViews *bool `json:"enable_materialized_views,omitempty" cass-config:"*:enable_materialized_views"`

	// Exists in trunk
	// +optional
	EnablePartitionDenylist *bool `json:"enable_partition_denylist,omitempty" cass-config:">=4.1.x:enable_partition_denylist"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableSasiIndexes *bool `json:"enable_sasi_indexes,omitempty" cass-config:"*:enable_sasi_indexes"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableScriptedUserDefinedFunctions *bool `json:"enable_scripted_user_defined_functions,omitempty" cass-config:"*:enable_scripted_user_defined_functions"`

	// Exists in: 4.0, trunk
	// +optional
	EnableTransientReplication *bool `json:"enable_transient_replication,omitempty" cass-config:">=4.x:enable_transient_replication"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableUserDefinedFunctions *bool `json:"enable_user_defined_functions,omitempty" cass-config:"*:enable_user_defined_functions"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableUserDefinedFunctionsThreads *bool `json:"enable_user_defined_functions_threads,omitempty" cass-config:"*:enable_user_defined_functions_threads"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EndpointSnitch *string `json:"endpoint_snitch,omitempty" cass-config:"*:endpoint_snitch"`

	// Exists in trunk
	// +optional
	FailureDetector *string `json:"failure_detector,omitempty" cass-config:">=4.1.x:failure_detector"`

	// Exists in: 4.0, trunk
	// +optional
	FileCacheEnabled *bool `json:"file_cache_enabled,omitempty" cass-config:">=4.x:file_cache_enabled"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	FileCacheRoundUp *bool `json:"file_cache_round_up,omitempty" cass-config:"*:file_cache_round_up"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	FileCacheSizeInMb *int `json:"file_cache_size_in_mb,omitempty" cass-config:"*:file_cache_size_in_mb"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=none;fast;table
	// +optional
	FlushCompression *string `json:"flush_compression,omitempty" cass-config:">=4.x:flush_compression"`

	// Exists in: 4.0, trunk
	// +optional
	FullQueryLoggingOptions *FullQueryLoggerOptions `json:"full_query_logging_options,omitempty" cass-config:">=4.x:full_query_logging_options;recurse"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	GcLogThresholdInMs *int `json:"gc_log_threshold_in_ms,omitempty" cass-config:"*:gc_log_threshold_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	GcWarnThresholdInMs *int `json:"gc_warn_threshold_in_ms,omitempty" cass-config:"*:gc_warn_threshold_in_ms"`

	// Exists in trunk
	// +optional
	HintWindowPersistentEnabled *bool `json:"hint_window_persistent_enabled,omitempty" cass-config:">=4.1.x:hint_window_persistent_enabled"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintedHandoffDisabledDatacenters *[]string `json:"hinted_handoff_disabled_datacenters,omitempty" cass-config:"*:hinted_handoff_disabled_datacenters"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintedHandoffEnabled *bool `json:"hinted_handoff_enabled,omitempty" cass-config:"*:hinted_handoff_enabled"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintedHandoffThrottleInKb *int `json:"hinted_handoff_throttle_in_kb,omitempty" cass-config:"*:hinted_handoff_throttle_in_kb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintsCompression *ParameterizedClass `json:"hints_compression,omitempty" cass-config:"*:hints_compression;recurse"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintsFlushPeriodInMs *int `json:"hints_flush_period_in_ms,omitempty" cass-config:"*:hints_flush_period_in_ms"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	IdealConsistencyLevel *string `json:"ideal_consistency_level,omitempty" cass-config:">=4.x:ideal_consistency_level"`

	// Exists in 3.11
	// +optional
	IndexInterval *int `json:"index_interval,omitempty" cass-config:"^3.11.x:index_interval"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	IndexSummaryCapacityInMb *int `json:"index_summary_capacity_in_mb,omitempty" cass-config:"*:index_summary_capacity_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	IndexSummaryResizeIntervalInMinutes *int `json:"index_summary_resize_interval_in_minutes,omitempty" cass-config:"*:index_summary_resize_interval_in_minutes"`

	// Exists in: 4.0, trunk
	// +optional
	InitialRangeTombstoneListAllocationSize *int `json:"initial_range_tombstone_list_allocation_size,omitempty" cass-config:">=4.x:initial_range_tombstone_list_allocation_size"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	InterDcStreamThroughputOutboundMegabitsPerSec *int `json:"inter_dc_stream_throughput_outbound_megabits_per_sec,omitempty" cass-config:"*:inter_dc_stream_throughput_outbound_megabits_per_sec"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	InterDcTcpNodelay *bool `json:"inter_dc_tcp_nodelay,omitempty" cass-config:"*:inter_dc_tcp_nodelay"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationReceiveQueueCapacityInBytes *int `json:"internode_application_receive_queue_capacity_in_bytes,omitempty" cass-config:">=4.x:internode_application_receive_queue_capacity_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationReceiveQueueReserveEndpointCapacityInBytes *int `json:"internode_application_receive_queue_reserve_endpoint_capacity_in_bytes,omitempty" cass-config:">=4.x:internode_application_receive_queue_reserve_endpoint_capacity_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationReceiveQueueReserveGlobalCapacityInBytes *int `json:"internode_application_receive_queue_reserve_global_capacity_in_bytes,omitempty" cass-config:">=4.x:internode_application_receive_queue_reserve_global_capacity_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationSendQueueCapacityInBytes *int `json:"internode_application_send_queue_capacity_in_bytes,omitempty" cass-config:">=4.x:internode_application_send_queue_capacity_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationSendQueueReserveEndpointCapacityInBytes *int `json:"internode_application_send_queue_reserve_endpoint_capacity_in_bytes,omitempty" cass-config:">=4.x:internode_application_send_queue_reserve_endpoint_capacity_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationSendQueueReserveGlobalCapacityInBytes *int `json:"internode_application_send_queue_reserve_global_capacity_in_bytes,omitempty" cass-config:">=4.x:internode_application_send_queue_reserve_global_capacity_in_bytes"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	InternodeAuthenticator *string `json:"internode_authenticator,omitempty" cass-config:"*:internode_authenticator"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=all;none;dc
	// +optional
	InternodeCompression *string `json:"internode_compression,omitempty" cass-config:"*:internode_compression"`

	// Exists in trunk
	// +optional
	InternodeErrorReportingExclusions *SubnetGroups `json:"internode_error_reporting_exclusions,omitempty" cass-config:">=4.1.x:internode_error_reporting_exclusions;recurse"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeMaxMessageSizeInBytes *int `json:"internode_max_message_size_in_bytes,omitempty" cass-config:">=4.x:internode_max_message_size_in_bytes"`

	// Exists in 3.11
	// +optional
	InternodeRecvBuffSizeInBytes *int `json:"internode_recv_buff_size_in_bytes,omitempty" cass-config:"^3.11.x:internode_recv_buff_size_in_bytes"`

	// Exists in 3.11
	// +optional
	InternodeSendBuffSizeInBytes *int `json:"internode_send_buff_size_in_bytes,omitempty" cass-config:"^3.11.x:internode_send_buff_size_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeSocketReceiveBufferSizeInBytes *int `json:"internode_socket_receive_buffer_size_in_bytes,omitempty" cass-config:">=4.x:internode_socket_receive_buffer_size_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeSocketSendBufferSizeInBytes *int `json:"internode_socket_send_buffer_size_in_bytes,omitempty" cass-config:">=4.x:internode_socket_send_buffer_size_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeStreamingTcpUserTimeoutInMs *int `json:"internode_streaming_tcp_user_timeout_in_ms,omitempty" cass-config:">=4.x:internode_streaming_tcp_user_timeout_in_ms"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeTcpConnectTimeoutInMs *int `json:"internode_tcp_connect_timeout_in_ms,omitempty" cass-config:">=4.x:internode_tcp_connect_timeout_in_ms"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeTcpUserTimeoutInMs *int `json:"internode_tcp_user_timeout_in_ms,omitempty" cass-config:">=4.x:internode_tcp_user_timeout_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	KeyCacheKeysToSave *int `json:"key_cache_keys_to_save,omitempty" cass-config:"*:key_cache_keys_to_save"`

	// Exists in: 4.0, trunk
	// +optional
	KeyCacheMigrateDuringCompaction *bool `json:"key_cache_migrate_during_compaction,omitempty" cass-config:">=4.x:key_cache_migrate_during_compaction"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	KeyCacheSavePeriod *int `json:"key_cache_save_period,omitempty" cass-config:"*:key_cache_save_period"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	KeyCacheSizeInMb *int `json:"key_cache_size_in_mb,omitempty" cass-config:"*:key_cache_size_in_mb"`

	// Exists in: 4.0, trunk
	// +optional
	KeyspaceCountWarnThreshold *int `json:"keyspace_count_warn_threshold,omitempty" cass-config:">=4.x:keyspace_count_warn_threshold"`

	// Exists in: 4.0, trunk
	// +optional
	MaxConcurrentAutomaticSstableUpgrades *int `json:"max_concurrent_automatic_sstable_upgrades,omitempty" cass-config:">=4.x:max_concurrent_automatic_sstable_upgrades"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxHintWindowInMs *int `json:"max_hint_window_in_ms,omitempty" cass-config:"*:max_hint_window_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxHintsDeliveryThreads *int `json:"max_hints_delivery_threads,omitempty" cass-config:"*:max_hints_delivery_threads"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxHintsFileSizeInMb *int `json:"max_hints_file_size_in_mb,omitempty" cass-config:"*:max_hints_file_size_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxMutationSizeInKb *int `json:"max_mutation_size_in_kb,omitempty" cass-config:"*:max_mutation_size_in_kb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxStreamingRetries *int `json:"max_streaming_retries,omitempty" cass-config:"*:max_streaming_retries"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxValueSizeInMb *int `json:"max_value_size_in_mb,omitempty" cass-config:"*:max_value_size_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=unslabbed_heap_buffers;unslabbed_heap_buffers_logged;heap_buffers;offheap_buffers;offheap_objects
	// +optional
	MemtableAllocationType *string `json:"memtable_allocation_type,omitempty" cass-config:"*:memtable_allocation_type"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableCleanupThreshold *string `json:"memtable_cleanup_threshold,omitempty" cass-config:"*:memtable_cleanup_threshold"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableFlushWriters *int `json:"memtable_flush_writers,omitempty" cass-config:"*:memtable_flush_writers"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableHeapSpaceInMb *int `json:"memtable_heap_space_in_mb,omitempty" cass-config:"*:memtable_heap_space_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableOffheapSpaceInMb *int `json:"memtable_offheap_space_in_mb,omitempty" cass-config:"*:memtable_offheap_space_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MinFreeSpacePerDriveInMb *int `json:"min_free_space_per_drive_in_mb,omitempty" cass-config:"*:min_free_space_per_drive_in_mb"`

	// Exists in trunk
	// +optional
	MinimumKeyspaceRf *int `json:"minimum_keyspace_rf,omitempty" cass-config:">=4.1.x:minimum_keyspace_rf"`

	// Exists in: 4.0, trunk
	// +optional
	NativeTransportAllowOlderProtocols *bool `json:"native_transport_allow_older_protocols,omitempty" cass-config:">=4.x:native_transport_allow_older_protocols"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportFlushInBatchesLegacy *bool `json:"native_transport_flush_in_batches_legacy,omitempty" cass-config:"*:native_transport_flush_in_batches_legacy"`

	// Exists in: 4.0, trunk
	// +optional
	NativeTransportIdleTimeoutInMs *int `json:"native_transport_idle_timeout_in_ms,omitempty" cass-config:">=4.x:native_transport_idle_timeout_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentConnections *int `json:"native_transport_max_concurrent_connections,omitempty" cass-config:"*:native_transport_max_concurrent_connections"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentConnectionsPerIp *int `json:"native_transport_max_concurrent_connections_per_ip,omitempty" cass-config:"*:native_transport_max_concurrent_connections_per_ip"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentRequestsInBytes *int `json:"native_transport_max_concurrent_requests_in_bytes,omitempty" cass-config:"*:native_transport_max_concurrent_requests_in_bytes"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentRequestsInBytesPerIp *int `json:"native_transport_max_concurrent_requests_in_bytes_per_ip,omitempty" cass-config:"*:native_transport_max_concurrent_requests_in_bytes_per_ip"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxFrameSizeInMb *int `json:"native_transport_max_frame_size_in_mb,omitempty" cass-config:"*:native_transport_max_frame_size_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxNegotiableProtocolVersion *int `json:"native_transport_max_negotiable_protocol_version,omitempty" cass-config:"*:native_transport_max_negotiable_protocol_version"`

	// Exists in trunk
	// +optional
	NativeTransportMaxRequestsPerSecond *int `json:"native_transport_max_requests_per_second,omitempty" cass-config:">=4.1.x:native_transport_max_requests_per_second"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxThreads *int `json:"native_transport_max_threads,omitempty" cass-config:"*:native_transport_max_threads"`

	// Exists in trunk
	// +optional
	NativeTransportRateLimitingEnabled *bool `json:"native_transport_rate_limiting_enabled,omitempty" cass-config:">=4.1.x:native_transport_rate_limiting_enabled"`

	// Exists in: 4.0, trunk
	// +optional
	NativeTransportReceiveQueueCapacityInBytes *int `json:"native_transport_receive_queue_capacity_in_bytes,omitempty" cass-config:">=4.x:native_transport_receive_queue_capacity_in_bytes"`

	// Exists in: 4.0, trunk
	// +optional
	NetworkAuthorizer *string `json:"network_authorizer,omitempty" cass-config:">=4.x:network_authorizer"`

	// Exists in: 4.0, trunk
	// +optional
	NetworkingCacheSizeInMb *int `json:"networking_cache_size_in_mb,omitempty" cass-config:">=4.x:networking_cache_size_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NumTokens *int `json:"num_tokens,omitempty" cass-config:"*:num_tokens"`

	// Exists in 3.11
	// +optional
	OtcBacklogExpirationIntervalMs *int `json:"otc_backlog_expiration_interval_ms,omitempty" cass-config:"^3.11.x:otc_backlog_expiration_interval_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	OtcCoalescingEnoughCoalescedMessages *int `json:"otc_coalescing_enough_coalesced_messages,omitempty" cass-config:"*:otc_coalescing_enough_coalesced_messages"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	OtcCoalescingStrategy *string `json:"otc_coalescing_strategy,omitempty" cass-config:"*:otc_coalescing_strategy"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	OtcCoalescingWindowUs *int `json:"otc_coalescing_window_us,omitempty" cass-config:"*:otc_coalescing_window_us"`

	// Exists in trunk
	// +optional
	PaxosCacheSizeInMb *int `json:"paxos_cache_size_in_mb,omitempty" cass-config:">=4.1.x:paxos_cache_size_in_mb"`

	// Exists in: 4.0, trunk
	// +optional
	PeriodicCommitlogSyncLagBlockInMs *int `json:"periodic_commitlog_sync_lag_block_in_ms,omitempty" cass-config:">=4.x:periodic_commitlog_sync_lag_block_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PermissionsCacheMaxEntries *int `json:"permissions_cache_max_entries,omitempty" cass-config:"*:permissions_cache_max_entries"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PermissionsUpdateIntervalInMs *int `json:"permissions_update_interval_in_ms,omitempty" cass-config:"*:permissions_update_interval_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PermissionsValidityInMs *int `json:"permissions_validity_in_ms,omitempty" cass-config:"*:permissions_validity_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PhiConvictThreshold *string `json:"phi_convict_threshold,omitempty" cass-config:"*:phi_convict_threshold"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PreparedStatementsCacheSizeMb *int `json:"prepared_statements_cache_size_mb,omitempty" cass-config:"*:prepared_statements_cache_size_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RangeRequestTimeoutInMs *int `json:"range_request_timeout_in_ms,omitempty" cass-config:"*:range_request_timeout_in_ms"`

	// Exists in: 4.0, trunk
	// +optional
	RangeTombstoneListGrowthFactor *string `json:"range_tombstone_list_growth_factor,omitempty" cass-config:">=4.x:range_tombstone_list_growth_factor"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ReadRequestTimeoutInMs *int `json:"read_request_timeout_in_ms,omitempty" cass-config:"*:read_request_timeout_in_ms"`

	// Exists in: 4.0, trunk
	// +optional
	RejectRepairCompactionThreshold *int `json:"reject_repair_compaction_threshold,omitempty" cass-config:">=4.x:reject_repair_compaction_threshold"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=queue;reject
	// +optional
	RepairCommandPoolFullStrategy *string `json:"repair_command_pool_full_strategy,omitempty" cass-config:">=4.x:repair_command_pool_full_strategy"`

	// Exists in: 4.0, trunk
	// +optional
	RepairCommandPoolSize *int `json:"repair_command_pool_size,omitempty" cass-config:">=4.x:repair_command_pool_size"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RepairSessionMaxTreeDepth *int `json:"repair_session_max_tree_depth,omitempty" cass-config:"*:repair_session_max_tree_depth"`

	// Exists in: 4.0, trunk
	// +optional
	RepairSessionSpaceInMb *int `json:"repair_session_space_in_mb,omitempty" cass-config:">=4.x:repair_session_space_in_mb"`

	// Exists in: 4.0, trunk
	// +optional
	RepairedDataTrackingForPartitionReadsEnabled *bool `json:"repaired_data_tracking_for_partition_reads_enabled,omitempty" cass-config:">=4.x:repaired_data_tracking_for_partition_reads_enabled"`

	// Exists in: 4.0, trunk
	// +optional
	RepairedDataTrackingForRangeReadsEnabled *bool `json:"repaired_data_tracking_for_range_reads_enabled,omitempty" cass-config:">=4.x:repaired_data_tracking_for_range_reads_enabled"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ReplicaFilteringProtection *ReplicaFilteringProtectionOptions `json:"replica_filtering_protection,omitempty" cass-config:"*:replica_filtering_protection;recurse"`

	// Exists in: 4.0, trunk
	// +optional
	ReportUnconfirmedRepairedDataMismatches *bool `json:"report_unconfirmed_repaired_data_mismatches,omitempty" cass-config:">=4.x:report_unconfirmed_repaired_data_mismatches"`

	// Exists in 3.11
	// +optional
	RequestScheduler *string `json:"request_scheduler,omitempty" cass-config:"^3.11.x:request_scheduler"`

	// Exists in 3.11
	// +kubebuilder:validation:Enum=keyspace
	// +optional
	RequestSchedulerId *string `json:"request_scheduler_id,omitempty" cass-config:"^3.11.x:request_scheduler_id"`

	// Exists in 3.11
	// +optional
	RequestSchedulerOptions *RequestSchedulerOptions `json:"request_scheduler_options,omitempty" cass-config:"^3.11.x:request_scheduler_options;recurse"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RequestTimeoutInMs *int `json:"request_timeout_in_ms,omitempty" cass-config:"*:request_timeout_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RoleManager *string `json:"role_manager,omitempty" cass-config:"*:role_manager"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RolesCacheMaxEntries *int `json:"roles_cache_max_entries,omitempty" cass-config:"*:roles_cache_max_entries"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RolesUpdateIntervalInMs *int `json:"roles_update_interval_in_ms,omitempty" cass-config:"*:roles_update_interval_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RolesValidityInMs *int `json:"roles_validity_in_ms,omitempty" cass-config:"*:roles_validity_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheClassName *string `json:"row_cache_class_name,omitempty" cass-config:"*:row_cache_class_name"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheKeysToSave *int `json:"row_cache_keys_to_save,omitempty" cass-config:"*:row_cache_keys_to_save"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheSavePeriod *int `json:"row_cache_save_period,omitempty" cass-config:"*:row_cache_save_period"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheSizeInMb *int `json:"row_cache_size_in_mb,omitempty" cass-config:"*:row_cache_size_in_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ServerEncryptionOptions *encryption.ServerEncryptionOptions `json:"server_encryption_options,omitempty" cass-config:"*:server_encryption_options;recurse"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SlowQueryLogTimeoutInMs *int `json:"slow_query_log_timeout_in_ms,omitempty" cass-config:"*:slow_query_log_timeout_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SnapshotBeforeCompaction *bool `json:"snapshot_before_compaction,omitempty" cass-config:"*:snapshot_before_compaction"`

	// Exists in: 4.0, trunk
	// +optional
	SnapshotLinksPerSecond *int `json:"snapshot_links_per_second,omitempty" cass-config:">=4.x:snapshot_links_per_second"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SnapshotOnDuplicateRowDetection *bool `json:"snapshot_on_duplicate_row_detection,omitempty" cass-config:"*:snapshot_on_duplicate_row_detection"`

	// Exists in: 4.0, trunk
	// +optional
	SnapshotOnRepairedDataMismatch *bool `json:"snapshot_on_repaired_data_mismatch,omitempty" cass-config:">=4.x:snapshot_on_repaired_data_mismatch"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SstablePreemptiveOpenIntervalInMb *int `json:"sstable_preemptive_open_interval_in_mb,omitempty" cass-config:"*:sstable_preemptive_open_interval_in_mb"`

	// Exists in: 4.0, trunk
	// +optional
	StreamEntireSstables *bool `json:"stream_entire_sstables,omitempty" cass-config:">=4.x:stream_entire_sstables"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	StreamThroughputOutboundMegabitsPerSec *int `json:"stream_throughput_outbound_megabits_per_sec,omitempty" cass-config:"*:stream_throughput_outbound_megabits_per_sec"`

	// Exists in: 4.0, trunk
	// +optional
	StreamingConnectionsPerHost *int `json:"streaming_connections_per_host,omitempty" cass-config:">=4.x:streaming_connections_per_host"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	StreamingKeepAlivePeriodInSecs *int `json:"streaming_keep_alive_period_in_secs,omitempty" cass-config:"*:streaming_keep_alive_period_in_secs"`

	// Exists in 3.11
	// +optional
	StreamingSocketTimeoutInMs *int `json:"streaming_socket_timeout_in_ms,omitempty" cass-config:"^3.11.x:streaming_socket_timeout_in_ms"`

	// Exists in: 4.0, trunk
	// +optional
	TableCountWarnThreshold *int `json:"table_count_warn_threshold,omitempty" cass-config:">=4.x:table_count_warn_threshold"`

	// Exists in 3.11
	// +optional
	ThriftFramedTransportSizeInMb *int `json:"thrift_framed_transport_size_in_mb,omitempty" cass-config:"^3.11.x:thrift_framed_transport_size_in_mb"`

	// Exists in 3.11
	// +optional
	ThriftMaxMessageLengthInMb *int `json:"thrift_max_message_length_in_mb,omitempty" cass-config:"^3.11.x:thrift_max_message_length_in_mb"`

	// Exists in 3.11
	// +optional
	ThriftPreparedStatementsCacheSizeMb *int `json:"thrift_prepared_statements_cache_size_mb,omitempty" cass-config:"^3.11.x:thrift_prepared_statements_cache_size_mb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TombstoneFailureThreshold *int `json:"tombstone_failure_threshold,omitempty" cass-config:"*:tombstone_failure_threshold"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TombstoneWarnThreshold *int `json:"tombstone_warn_threshold,omitempty" cass-config:"*:tombstone_warn_threshold"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TracetypeQueryTtl *int `json:"tracetype_query_ttl,omitempty" cass-config:"*:tracetype_query_ttl"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TracetypeRepairTtl *int `json:"tracetype_repair_ttl,omitempty" cass-config:"*:tracetype_repair_ttl"`

	// Exists in trunk
	// +optional
	TrackWarnings *TrackWarnings `json:"track_warnings,omitempty" cass-config:">=4.1.x:track_warnings;recurse"`

	// Exists in trunk
	// +optional
	TraverseAuthFromRoot *bool `json:"traverse_auth_from_root,omitempty" cass-config:">=4.1.x:traverse_auth_from_root"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TrickleFsync *bool `json:"trickle_fsync,omitempty" cass-config:"*:trickle_fsync"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TrickleFsyncIntervalInKb *int `json:"trickle_fsync_interval_in_kb,omitempty" cass-config:"*:trickle_fsync_interval_in_kb"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TruncateRequestTimeoutInMs *int `json:"truncate_request_timeout_in_ms,omitempty" cass-config:"*:truncate_request_timeout_in_ms"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	UnloggedBatchAcrossPartitionsWarnThreshold *int `json:"unlogged_batch_across_partitions_warn_threshold,omitempty" cass-config:"*:unlogged_batch_across_partitions_warn_threshold"`

	// Exists in trunk
	// +optional
	UseDeterministicTableId *bool `json:"use_deterministic_table_id,omitempty" cass-config:">=4.1.x:use_deterministic_table_id"`

	// Exists in: 4.0, trunk
	// +optional
	UseOffheapMerkleTrees *bool `json:"use_offheap_merkle_trees,omitempty" cass-config:">=4.x:use_offheap_merkle_trees"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	UserDefinedFunctionFailTimeout *int `json:"user_defined_function_fail_timeout,omitempty" cass-config:"*:user_defined_function_fail_timeout"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	UserDefinedFunctionWarnTimeout *int `json:"user_defined_function_warn_timeout,omitempty" cass-config:"*:user_defined_function_warn_timeout"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=ignore;die;die_immediate
	// +optional
	UserFunctionTimeoutPolicy *string `json:"user_function_timeout_policy,omitempty" cass-config:"*:user_function_timeout_policy"`

	// Exists in: 4.0, trunk
	// +optional
	ValidationPreviewPurgeHeadStartInSec *int `json:"validation_preview_purge_head_start_in_sec,omitempty" cass-config:">=4.x:validation_preview_purge_head_start_in_sec"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	WindowsTimerInterval *int `json:"windows_timer_interval,omitempty" cass-config:"*:windows_timer_interval"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	WriteRequestTimeoutInMs *int `json:"write_request_timeout_in_ms,omitempty" cass-config:"*:write_request_timeout_in_ms"`
}

type JvmOptions struct {

	// MEMORY OPTIONS

	// Deprecated. Use heap_initial_size and heap_max_size instead. If this field is defined,
	// it applies to both max_heap_size and initial_heap_size.
	// +optional
	HeapSize *resource.Quantity `json:"heapSize,omitempty"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Xms.
	// +optional
	InitialHeapSize *resource.Quantity `json:"heap_initial_size,omitempty" cass-config:"^3.11.x:jvm-options/initial_heap_size,>=4.x:jvm-server-options/initial_heap_size"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Xmx.
	// +optional
	MaxHeapSize *resource.Quantity `json:"heap_max_size,omitempty" cass-config:"^3.11.x:jvm-options/max_heap_size,>=4.x:jvm-server-options/max_heap_size"`

	// GENERAL VM OPTIONS

	// Enable assertions.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -ea.
	// +optional
	EnableAssertions *bool `json:"vm_enable_assertions,omitempty" cass-config:"^3.11.x:jvm-options/enable_assertions,>=4.x:jvm-server-options/enable_assertions"`

	// Enable thread priorities.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UseThreadPriorities.
	// +optional
	EnableThreadPriorities *bool `json:"vm_enable_thread_priorities,omitempty" cass-config:"^3.11.x:jvm-options/use_thread_priorities,>=4.x:jvm-server-options/use_thread_priorities"`

	// Enable lowering thread priority without being root on linux.
	// See CASSANDRA-1181 for details.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:ThreadPriorityPolicy=42.
	// +optional
	EnableNonRootThreadPriority *bool `json:"vm_enable_non_root_thread_priority,omitempty" cass-config:"^3.11.x:jvm-options/thread_priority_policy_42"`

	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+HeapDumpOnOutOfMemoryError.
	// +optional
	HeapDumpOnOutOfMemoryError *bool `json:"vm_heap_dump_on_out_of_memory_error,omitempty" cass-config:"^3.11.x:jvm-options/heap_dump_on_out_of_memory_error,>=4.x:jvm-server-options/heap_dump_on_out_of_memory_error"`

	// Per-thread stack size.
	// Defaults to 256Ki.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Xss.
	// +optional
	PerThreadStackSize *resource.Quantity `json:"vm_per_thread_stack_size,omitempty" cass-config:"^3.11.x:jvm-options/per_thread_stack_size,>=4.x:jvm-server-options/per_thread_stack_size"`

	// The size of interned string table. Larger sizes are beneficial to gossip.
	// Defaults to 1000003.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:StringTableSize.
	// +optional
	StringTableSize *resource.Quantity `json:"vm_string_table_size,omitempty" cass-config:"^3.11.x:jvm-options/string_table_size,>=4.x:jvm-server-options/string_table_size"`

	// Ensure all memory is faulted and zeroed on startup.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+AlwaysPreTouch.
	// +optional
	AlwaysPreTouch *bool `json:"vm_always_pre_touch,omitempty" cass-config:"^3.11.x:jvm-options/always_pre_touch,>=4.x:jvm-server-options/always_pre_touch"`

	// Disable biased locking to avoid biased lock revocation pauses.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:-UseBiasedLocking.
	// Note: the Cass Config Builder option is named use_biased_locking, but setting it to true
	// disables biased locking.
	// +optional
	DisableBiasedLocking *bool `json:"vm_disable_biased_locking,omitempty" cass-config:"^3.11.x:jvm-options/use_biased_locking,>=4.x:jvm-server-options/use-biased-locking"`

	// Enable thread-local allocation blocks.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UseTLAB.
	// +optional
	UseTlab *bool `json:"vm_use_tlab,omitempty" cass-config:"^3.11.x:jvm-options/use_tlb,>=4.x:jvm-server-options/use_tlb"`

	// Allow resizing of thread-local allocation blocks.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+ResizeTLAB.
	// +optional
	ResizeTlab *bool `json:"vm_resize_tlab,omitempty" cass-config:"^3.11.x:jvm-options/resize_tlb,>=4.x:jvm-server-options/resize_tlb"`

	// Disable hsperfdata mmap'ed file.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+PerfDisableSharedMem.
	// +optional
	DisablePerfSharedMem *bool `json:"vm_disable_perf_shared_mem,omitempty" cass-config:"^3.11.x:jvm-options/perf_disable_shared_mem,>=4.x:jvm-server-options/perf_disable_shared_mem"`

	// Prefer binding to IPv4 network interfaces.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Djava.net.preferIPv4Stack=true.
	// +optional
	PreferIpv4 *bool `json:"vm_prefer_ipv4,omitempty" cass-config:"^3.11.x:jvm-options/java_net_prefer_ipv4_stack,>=4.x:jvm-server-options/java_net_prefer_ipv4_stack"`

	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UseNUMA.
	// +optional
	UseNuma *bool `json:"vm_use_numa,omitempty" cass-config:">=4.x:jvm-server-options/use_numa"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.printHeapHistogramOnOutOfMemoryError.
	// +optional
	PrintHeapHistogramOnOutOfMemoryError *bool `json:"vm_print_heap_histogram_on_out_of_memory_error,omitempty" cass-config:">=4.x:jvm-server-options/print_heap_histogram_on_out_of_memory_error"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+ExitOnOutOfMemoryError.
	// +optional
	ExitOnOutOfMemoryError *bool `json:"vm_exit_on_out_of_memory_error,omitempty" cass-config:">=4.x:jvm-server-options/exit_on_out_of_memory_error"`

	// Disabled by default. Requires `exit_on_out_of_memory_error` to be disabled..
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+CrashOnOutOfMemoryError.
	// +optional
	CrashOnOutOfMemoryError *bool `json:"vm_crash_on_out_of_memory_error,omitempty" cass-config:">=4.x:jvm-server-options/crash_on_out_of_memory_error"`

	// Defaults to 300000 milliseconds.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:GuaranteedSafepointInterval.
	// +optional
	GuaranteedSafepointIntervalMs *int `json:"vm_guaranteed_safepoint_interval_ms,omitempty" cass-config:">=4.x:jvm-server-options/guaranteed-safepoint-interval"`

	// Defaults to 65536.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dio.netty.eventLoop.maxPendingTasks.
	// +optional
	NettyEventloopMaxPendingTasks *int `json:"netty_eventloop_maxpendingtasks,omitempty" cass-config:">=4.x:jvm-server-options/io_netty_eventloop_maxpendingtasks"`

	// Netty setting `io.netty.tryReflectionSetAccessible`.
	// Defaults to true.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -Dio.netty.tryReflectionSetAccessible=true.
	// +optional
	NettyTryReflectionSetAccessible *bool `json:"netty_try_reflection_set_accessible,omitempty" cass-config:">=4.x:jvm11-server-options/io_netty_try_reflection_set_accessible"`

	// Defaults to 1048576.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Djdk.nio.maxCachedBufferSize.
	// +optional
	NioMaxCachedBufferSize *resource.Quantity `json:"nio_maxcachedbuffersize,omitempty" cass-config:">=4.x:jvm-server-options/jdk_nio_maxcachedbuffersize"`

	// Align direct memory allocations on page boundaries.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dsun.nio.PageAlignDirectMemory=true.
	// +optional
	NioAlignDirectMemory *bool `json:"nio_align_direct_memory,omitempty" cass-config:">=4.x:jvm-server-options/page-align-direct-memory"`

	// Allow the current VM to attach to itself.
	// Defaults to true.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -Djdk.attach.allowAttachSelf=true.
	// +optional
	JdkAllowAttachSelf *bool `json:"jdk_allow_attach_self,omitempty" cass-config:">=4.x:jvm11-server-options/jdk_attach_allow_attach_self"`

	// CASSANDRA STARTUP OPTIONS

	// Available CPU processors.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.available_processors.
	// +optional
	AvailableProcessors *int `json:"cassandra_available_processors,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_available_processors,>=4.x:jvm-server-options/cassandra_available_processors"`

	// Enable pluggable metrics reporter.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.metricsReporterConfigFile.
	// +optional
	// TODO mountable directory
	MetricsReporterConfigFile *string `json:"cassandra_metrics_reporter_config_file,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_metrics_reporter_config_file,>=4.x:jvm-server-options/cassandra_metrics_reporter_config_file"`

	// Amount of time in milliseconds that a node waits before joining the ring.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.ring_delay_ms.
	// +optional
	RingDelayMs *int `json:"cassandra_ring_delay_ms,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_ring_delay_ms,>=4.x:jvm-server-options/cassandra_ring_delay_ms"`

	// Default location for the trigger JARs.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.triggers_dir.
	// +optional
	// TODO mountable directory
	TriggersDirectory *string `json:"cassandra_triggers_directory,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_triggers_dir,>=4.x:jvm-server-options/cassandra_triggers_dir"`

	// For testing new compaction and compression strategies.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.write_survey.
	// +optional
	WriteSurvey *bool `json:"cassandra_write_survey,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_write_survey,>=4.x:jvm-server-options/cassandra_write_survey"`

	// Disable remote configuration via JMX of auth caches.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.disable_auth_caches_remote_configuration.
	// +optional
	DisableAuthCachesRemoteConfiguration *bool `json:"cassandra_disable_auth_caches_remote_configuration,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_disable_auth_caches_remote_configuration,>=4.x:jvm-server-options/cassandra_disable_auth_caches_remote_configuration"`

	// Disable dynamic calculation of the page size used when indexing an entire partition (during
	// initial index build/rebuild).
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.force_default_indexing_page_size.
	// +optional
	ForceDefaultIndexingPageSize *bool `json:"cassandra_force_default_indexing_page_size,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_force_default_indexing_page_size,>=4.x:jvm-server-options/cassandra_force_default_indexing_page_size"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -Dcassandra.force_3_0_protocol_version=true.
	// +optional
	Force30ProtocolVersion *bool `json:"cassandra_force_3_0_protocol_version,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_force_3_0_protocol_version"`

	// Defines how to handle INSERT requests with TTL exceeding the maximum supported expiration
	// date.
	// Possible values include `REJECT`, `CAP`, `CAP_NOWARN`.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.expiration_date_overflow_policy.
	// +optional
	ExpirationDateOverflowPolicy *string `json:"cassandra_expiration_date_overflow_policy,omitempty" cass-config:">=4.x:jvm-server-options/cassandra_expiration_date_overflow_policy"`

	// Imposes an upper bound on hint lifetime below the normal min gc_grace_seconds.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.maxHintTTL.
	// +optional
	MaxHintTtlSeconds *int `json:"cassandra_max_hint_ttl_seconds,omitempty" cass-config:">=4.x:jvm-server-options/cassandra_max_hint_ttl"`

	// GC OPTIONS

	// The name of the garbage collector to use. Depending on the Cassandra version, not all values
	// are supported: Cassandra 3.11 supports only G1GC and CMS; Cassandra 4.0 supports G1GC, ZGC,
	// Shenandoah and Graal. This option will unlock the corresponding garbage collector with a
	// default configuration; to further tune the GC settings, use the additional JVM options field.
	// Use the special value Custom if you intend to use non-standard garbage collectors.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// +kubebuilder:validation:Enum=G1GC;CMS;ZGC;Shenandoah;Graal;Custom
	// +kubebuilder:default=G1GC
	// +optional
	GarbageCollector string `json:"gc,omitempty" cass-config:"^3.11.x:jvm-options/garbage_collector,>=4.x:jvm11-server-options/garbage_collector"`

	// CMS

	// Deprecated. Use gc_cms_heap_size_young_generation instead.
	// Valid for CMS garbage collector only + Cassandra 3.11.
	// +optional
	HeapNewGenSize *resource.Quantity `json:"heapNewGenSize,omitempty"`

	// Disabled by default. Can only be used when CMS garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -Xmn.
	// +optional
	CmsHeapSizeYoungGeneration *resource.Quantity `json:"gc_cms_heap_size_young_generation,omitempty" cass-config:"^3.11.x:jvm-options/heap_size_young_generation"`

	// Defaults to 8. Can only be used when CMS garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:SurvivorRatio.
	// +optional
	CmsSurvivorRatio *int `json:"gc_cms_survivor_ratio,omitempty" cass-config:"^3.11.x:jvm-options/survivor_ratio"`

	// Defaults to 1. Can only be used when CMS garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:MaxTenuringThreshold.
	// +optional
	CmsMaxTenuringThreshold *int `json:"gc_cms_max_tenuring_threshold,omitempty" cass-config:"^3.11.x:jvm-options/max_tenuring_threshold"`

	// Defaults to 75. Can only be used when CMS garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:CMSInitiatingOccupancyFraction.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	CmsInitiatingOccupancyFraction *int `json:"gc_cms_initiating_occupancy_fraction,omitempty" cass-config:"^3.11.x:jvm-options/cms_initiating_occupancy_fraction"`

	// Defaults to 10000. Can only be used when CMS garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:CMSWaitDuration.
	// +optional
	CmsWaitDurationMs *int `json:"gc_cms_wait_duration_ms,omitempty" cass-config:"^3.11.x:jvm-options/cms_wait_duration"`

	// G1

	// G1GC Updating Pause Time Percentage. Defaults to 5. Can only be used when G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:G1RSetUpdatingPauseTimePercent.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	G1RSetUpdatingPauseTimePercent *int `json:"gc_g1_rset_updating_pause_time_percent,omitempty" cass-config:"^3.11.x:jvm-options/g1r_set_updating_pause_time_percent,>=4.x:jvm11-server-options/g1r_set_updating_pause_time_percent"`

	// G1GC Max GC Pause in milliseconds. Defaults to 500. Can only be used when G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:MaxGCPauseMillis.
	// +optional
	G1MaxGcPauseMs *int `json:"gc_g1_max_gc_pause_ms,omitempty" cass-config:"^3.11.x:jvm-options/max_gc_pause_millis,>=4.x:jvm11-server-options/max_gc_pause_millis"`

	// Initiating Heap Occupancy Percentage. Can only be used when G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:InitiatingHeapOccupancyPercent.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	G1InitiatingHeapOccupancyPercent *int `json:"gc_g1_initiating_heap_occupancy_percent,omitempty" cass-config:"^3.11.x:jvm-options/initiating_heap_occupancy_percent,>=4.x:jvm11-server-options/initiating_heap_occupancy_percent"`

	// Parallel GC Threads. Can only be used when G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:ParallelGCThreads.
	// +optional
	G1ParallelGcThreads *int `json:"gc_g1_parallel_threads,omitempty" cass-config:"^3.11.x:jvm-options/parallel_gc_threads,>=4.x:jvm11-server-options/parallel_gc_threads"`

	// Concurrent GC Threads. Can only be used when G1 garbage collector is used.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:ConcGCThreads.
	// +optional
	G1ConcGcThreads *int `json:"gc_g1_conc_threads,omitempty" cass-config:"^3.11.x:jvm-options/conc_gc_threads,>=4.x:jvm11-server-options/conc_gc_threads"`

	// GC LOGGING OPTIONS (currently only available for Cassandra 3.11)

	// Print GC details.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:+PrintGCDetails.
	// +optional
	PrintDetails *bool `json:"gc_print_details,omitempty" cass-config:"^3.11.x:jvm-options/print_gc_details"`

	// Print GC Date Stamps. Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:+PrintGCDateStamps.
	// +optional
	PrintDateStamps *bool `json:"gc_print_date_stamps,omitempty" cass-config:"^3.11.x:jvm-options/print_gc_date_stamps"`

	// Print Heap at GC.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:+PrintHeapAtGC.
	// +optional
	PrintHeap *bool `json:"gc_print_heap,omitempty" cass-config:"^3.11.x:jvm-options/print_heap_at_gc"`

	// Print tenuring distribution.
	// Defaults to false.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:+PrintTenuringDistribution.
	// +optional
	PrintTenuringDistribution *bool `json:"gc_print_tenuring_distribution,omitempty" cass-config:"^3.11.x:jvm-options/print_tenuring_distribution"`

	// Print GC Application Stopped Time.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:+PrintGCApplicationStoppedTime.
	// +optional
	PrintApplicationStoppedTime *bool `json:"gc_print_application_stopped_time,omitempty" cass-config:"^3.11.x:jvm-options/print_gc_application_stopped_time"`

	// Print promotion failure.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:+PrintPromotionFailure.
	// +optional
	PrintPromotionFailure *bool `json:"gc_print_promotion_failure,omitempty" cass-config:"^3.11.x:jvm-options/print_promotion_failure"`

	// Print FLSS Statistics.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:PrintFLSStatistics=1.
	// +optional
	PrintFlssStatistics *bool `json:"gc_print_flss_statistics,omitempty" cass-config:"^3.11.x:jvm-options/print_flss_statistics"`

	// Whether to print GC logs to /var/log/cassandra/gc.log.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -Xloggc:/var/log/cassandra/gc.log.
	// +optional
	UseLogFile *bool `json:"gc_print_use_log_file,omitempty" cass-config:"^3.11.x:jvm-options/log_gc"`

	// Use GC Log File Rotation.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:+UseGCLogFileRotation.
	// +optional
	UseLogFileRotation *bool `json:"gc_print_use_log_file_rotation,omitempty" cass-config:"^3.11.x:jvm-options/use_gc_log_file_rotation"`

	// Number of GC log files.
	// Disabled by default. Can only be used when the G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:NumberOfGCLogFiles.
	// +optional
	NumberOfLogFiles *int `json:"gc_print_number_of_log_files,omitempty" cass-config:"^3.11.x:jvm-options/number_of_gc_log_files"`

	// Size of each log file.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Corresponds to: -XX:GCLogFileSize.
	// +optional
	LogFileSize *resource.Quantity `json:"gc_print_log_file_size,omitempty" cass-config:"^3.11.x:jvm-options/gc_log_file_size"`

	// JMX OPTIONS

	// Disabled by default. Defaults to 7199.
	// TODO Make Reaper aware of the JMX port if a non-default port is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// +optional
	JmxPort *int `json:"jmx_port,omitempty" cass-config:"^3.11.x:jvm-options/jmx-port,>=4.x:jvm-server-options/jmx-port"`

	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Possible values for 3.11 include `local-no-auth`, `remote-no-auth`, and `remote-dse-unified-auth`. Defaults to `local-no-auth`.
	// Possible values for 4.0 include `local-no-auth`, `remote-no-auth`. Defaults to `local-no-auth`.
	// +optional
	JmxConnectionType *string `json:"jmx_connection_type,omitempty" cass-config:"^3.11.x:jvm-options/jmx-connection-type,>=4.x:jvm-server-options/jmx-connection-type"`

	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Defaults to false.
	// Valid only when JmxConnectionType is "remote-no-auth", "remote-dse-unified-auth".
	// +optional
	JmxRemoteSsl *bool `json:"jmx_remote_ssl,omitempty" cass-config:"^3.11.x:jvm-options/jmx-remote-ssl,>=4.x:jvm-server-options/jmx-remote-ssl"`

	// Remote SSL options.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// +optional
	JmxRemoteSslOpts *string `json:"jmx_remote_ssl_opts,omitempty" cass-config:"^3.11.x:jvm-options/jmx-remote-ssl-opts,>=4.x:jvm-server-options/jmx-remote-ssl-opts"`

	// Require Client Authentication for remote SSL? Defaults to false.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// +optional
	JmxRemoteSslRequireClientAuth *bool `json:"jmx_remote_ssl_require_client_auth,omitempty" cass-config:">=4.x:jvm-server-options/jmx-remote-ssl-require-client-auth"`

	// DEBUG OPTIONS

	// Unlock commercial features.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UnlockCommercialFeatures.
	// +optional
	UnlockCommercialFeatures *bool `json:"debug_unlock_commercial_features,omitempty" cass-config:"^3.11.x:jvm-options/unlock_commercial_features,>=4.x:jvm-server-options/unlock_commercial_features"`

	// Enable Flight Recorder (Use in production is subject to Oracle licensing).
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+FlightRecorder.
	// +optional
	EnableFlightRecorder *bool `json:"debug_enable_flight_recorder,omitempty" cass-config:"^3.11.x:jvm-options/flight_recorder,>=4.x:jvm-server-options/flight_recorder"`

	// Listen for JVM remote debuggers on port 1414.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=1414".
	// +optional
	ListenForRemoteDebuggers *bool `json:"debug_listen_remote_debuggers,omitempty" cass-config:"^3.11.x:jvm-options/agent_lib_jdwp,>=4.x:jvm-server-options/agent_lib_jdwp"`

	// Disable honoring user code @Contended annotations.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:-RestrictContended.
	// +optional
	DisableContendedAnnotations *bool `json:"debug_disable_contended_annotations,omitempty" cass-config:">=4.x:jvm-server-options/restrict-contended"`

	// Whether the compiler should generate the necessary metadata for the parts of the code not at
	// safe points as well. For use with Flight Recorder.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+DebugNonSafepoints.
	// +optional
	DebugNonSafepoints *bool `json:"debug_non_safepoints,omitempty" cass-config:">=4.x:jvm-server-options/debug-non-safepoints"`

	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UnlockDiagnosticVMOptions.
	// +optional
	UnlockDiagnosticVmOptions *bool `json:"debug_unlock_diagnostic_vm_options,omitempty" cass-config:">=4.x:jvm-server-options/unlock-diagnostic-vm-options"`

	// Make Cassandra JVM log internal method compilation (developers only).
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+LogCompilation.
	// +optional
	LogCompilation *bool `json:"debug_log_compilation,omitempty" cass-config:">=4.x:jvm-server-options/log_compilation"`

	// Preserve Frame Pointer.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+PreserveFramePointer.
	// +optional
	PreserveFramePointer *bool `json:"debug_preserve_frame_pointer,omitempty" cass-config:">=4.x:jvm-server-options/preserve-frame-pointer"`

	// Additional, arbitrary JVM options (advanced).
	// +optional
	AdditionalOptions []string `json:"additionalOptions,omitempty" cass-config:"*:cassandra-env-sh/additional-jvm-opts"`
}

type ParameterizedClass struct {
	ClassName  string             `json:"class_name" cass-config:"*:class_name"`
	Parameters *map[string]string `json:"parameters,omitempty" cass-config:"*:parameters"`
}

type ReplicaFilteringProtectionOptions struct {
	CachedRowsWarnThreshold *int `json:"cached_rows_warn_threshold,omitempty" cass-config:"*:cached_rows_warn_threshold"`
	CachedRowsFailThreshold *int `json:"cached_rows_fail_threshold,omitempty" cass-config:"*:cached_rows_fail_threshold"`
}

type RequestSchedulerOptions struct {
	ThrottleLimit *int            `json:"throttle_limit,omitempty" cass-config:"*:throttle_limit"`
	DefaultWeight *int            `json:"default_weight,omitempty" cass-config:"*:default_weight"`
	Weights       *map[string]int `json:"weights,omitempty" cass-config:"*:weights"`
}

type AuditLogOptions struct {
	Enabled            bool                `json:"enabled" cass-config:"*:enabled;retainzero"`
	Logger             *ParameterizedClass `json:"logger,omitempty" cass-config:"*:logger;recurse"`
	IncludedKeyspaces  *string             `json:"included_keyspaces,omitempty" cass-config:"*:included_keyspaces"`
	ExcludedKeyspaces  *string             `json:"excluded_keyspaces,omitempty" cass-config:"*:excluded_keyspaces"`
	IncludedCategories *string             `json:"included_categories,omitempty" cass-config:"*:included_categories"`
	ExcludedCategories *string             `json:"excluded_categories,omitempty" cass-config:"*:excluded_categories"`
	IncludedUsers      *string             `json:"included_users,omitempty" cass-config:"*:included_users"`
	ExcludedUsers      *string             `json:"excluded_users,omitempty" cass-config:"*:excluded_users"`
	RollCycle          *string             `json:"roll_cycle,omitempty" cass-config:"*:roll_cycle"`
	Block              *bool               `json:"block,omitempty" cass-config:"*:block"`
	MaxQueueWeight     *int                `json:"max_queue_weight,omitempty" cass-config:"*:max_queue_weight"`
	MaxLogSize         *int                `json:"max_log_size,omitempty" cass-config:"*:max_log_size"`
	ArchiveCommand     *string             `json:"archive_command,omitempty" cass-config:"*:archive_command"`
	MaxArchiveRetries  *int                `json:"max_archive_retries,omitempty" cass-config:"*:max_archive_retries"`
}

type FullQueryLoggerOptions struct {
	ArchiveCommand    *string `json:"archive_command,omitempty" cass-config:"*:archive_command"`
	RollCycle         *string `json:"roll_cycle,omitempty" cass-config:"*:roll_cycle"`
	Block             *bool   `json:"block,omitempty" cass-config:"*:block"`
	MaxQueueWeight    *int    `json:"max_queue_weight,omitempty" cass-config:"*:max_queue_weight"`
	MaxLogSize        *int    `json:"max_log_size,omitempty" cass-config:"*:max_log_size"`
	MaxArchiveRetries *int    `json:"max_archive_retries,omitempty" cass-config:"*:max_archive_retries"`
	LogDir            *string `json:"log_dir,omitempty" cass-config:"*:log_dir"`
}

type SubnetGroups struct {
	Subnets []string `json:"subnets" cass-config:"*:subnets"`
}

type TrackWarnings struct {
	Enabled             bool `json:"enabled" cass-config:"*:enabled;retainzero"`
	CoordinatorReadSize *int `json:"coordinator_read_size,omitempty" cass-config:"*:coordinator_read_size"`
	LocalReadSize       *int `json:"local_read_size,omitempty" cass-config:"*:local_read_size"`
	RowIndexSize        *int `json:"row_index_size,omitempty" cass-config:"*:row_index_size"`
}
