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
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// K8ssandraClusterSpec defines the desired state of K8ssandraCluster
type K8ssandraClusterSpec struct {

	// Whether to enable authentication in this cluster. The default is true; it is highly recommended to always leave
	// authentication turned on. When enabled, authentication will be enforced not only on Cassandra nodes, but also on
	// Reaper, Medusa and Stargate nodes, if any.
	// +optional
	// +kubebuilder:default=true
	Auth *bool `json:"auth,omitempty"`

	// Cassandra is a specification of the Cassandra cluster. This includes everything from
	// the number of datacenters, the k8s cluster where each DC should be deployed, node
	// affinity (via racks), individual C* node settings, JVM settings, and more.
	Cassandra *CassandraClusterTemplate `json:"cassandra,omitempty"`

	// Stargate defines the desired deployment characteristics for Stargate in this K8ssandraCluster.
	// If this is non-nil, Stargate will be deployed on every Cassandra datacenter in this K8ssandraCluster.
	// +optional
	Stargate *stargateapi.StargateClusterTemplate `json:"stargate,omitempty"`

	// Reaper defines the desired deployment characteristics for Reaper in this K8ssandraCluster.
	// If this is non-nil, Reaper will be deployed on every Cassandra datacenter in this K8ssandraCluster.
	// +optional
	Reaper *reaperapi.ReaperClusterTemplate `json:"reaper,omitempty"`

	// Medusa defines the desired deployment characteristics for Medusa in this K8ssandraCluster.
	// If this is non-nil, Medusa will be deployed in every Cassandra pod in this K8ssandraCluster.
	// +optional
	Medusa *medusaapi.MedusaClusterTemplate `json:"medusa,omitempty"`
}

func (in K8ssandraClusterSpec) IsAuthEnabled() bool {
	return in.Auth == nil || *in.Auth
}

// K8ssandraClusterStatus defines the observed state of K8ssandraCluster
type K8ssandraClusterStatus struct {
	// +optional
	Conditions []K8ssandraClusterCondition `json:"conditions,omitempty"`

	// Datacenters maps the CassandraDatacenter name to a K8ssandraStatus. The
	// naming is a bit confusing but the mapping makes sense because we have a
	// CassandraDatacenter and then define other components like Stargate and Reaper
	// relative to it. I wanted to inline the field but when I do it won't serialize.
	//
	// TODO Figure out how to inline this field
	Datacenters map[string]K8ssandraStatus `json:"datacenters,omitempty"`
}

type K8ssandraClusterConditionType string

const (
	// CassandraInitialized is set to true when the Cassandra cluster becomes ready for
	// the first time. During the life time of the C* cluster CassandraDatacenters may have
	// their readiness condition change back and forth. Once set, this condition however
	// does not change.
	CassandraInitialized = "CassandraInitialized"
)

type K8ssandraClusterCondition struct {
	Type   K8ssandraClusterConditionType `json:"type"`
	Status corev1.ConditionStatus        `json:"status"`

	// LastTransitionTime is the last time the condition transited from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}

// K8ssandraStatus defines the observed of a k8ssandra instance
type K8ssandraStatus struct {
	Cassandra *cassdcapi.CassandraDatacenterStatus `json:"cassandra,omitempty"`
	Stargate  *stargateapi.StargateStatus          `json:"stargate,omitempty"`
	Reaper    *reaperapi.ReaperStatus              `json:"reaper,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=k8ssandraclusters,shortName=k8c;k8cs

// K8ssandraCluster is the Schema for the k8ssandraclusters API
type K8ssandraCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   K8ssandraClusterSpec   `json:"spec,omitempty"`
	Status K8ssandraClusterStatus `json:"status,omitempty"`
}

// HasStargates returns true if at least one Stargate resource will be created as part of the creation
// of this K8ssandraCluster object.
func (in *K8ssandraCluster) HasStargates() bool {
	if in == nil {
		return false
	} else if in.Spec.Stargate != nil {
		return true
	} else if in.Spec.Cassandra == nil || len(in.Spec.Cassandra.Datacenters) == 0 {
		return false
	}
	for _, dcTemplate := range in.Spec.Cassandra.Datacenters {
		if dcTemplate.Stargate != nil {
			return true
		}
	}
	return false
}

// HasReapers returns true if at least one Reaper resource will be created as part of the creation
// of this K8ssandraCluster object.
func (in *K8ssandraCluster) HasReapers() bool {
	if in == nil {
		return false
	} else if in.Spec.Reaper != nil {
		return true
	} else if in.Spec.Cassandra == nil || len(in.Spec.Cassandra.Datacenters) == 0 {
		return false
	}
	for _, dcTemplate := range in.Spec.Cassandra.Datacenters {
		if dcTemplate.Reaper != nil {
			return true
		}
	}
	return false
}

// +kubebuilder:object:root=true

// K8ssandraClusterList contains a list of K8ssandraCluster
type K8ssandraClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []K8ssandraCluster `json:"items"`
}

type CassandraClusterTemplate struct {
	// Cluster is the name of the cluster. This corresponds to cluster_name in
	// cassandra.yaml.
	// +kubebuilder:validation:MinLength=2
	Cluster string `json:"cluster,omitempty"`

	// The reference to the superuser secret to use for Cassandra. If unspecified, a default secret will be generated
	// with a random password; the generated secret name will be "<cluster_name>-superuser" where <cluster_name> is the
	// Cassandra cluster name specified above.
	// +optional
	SuperuserSecretRef corev1.LocalObjectReference `json:"superuserSecretRef,omitempty"`

	// ServerImage is the image for the cassandra container. Note that this should be a
	// management-api image. If left empty the operator will choose a default image based
	// on ServerVersion.
	// +optional
	ServerImage string `json:"serverImage,omitempty"`

	// ServerVersion is the Cassandra version.
	// +kubebuilder:validation:Pattern=(3\.11\.\d+)|(4\.0\.\d+)
	ServerVersion string `json:"serverVersion,omitempty"`

	// The image to use in each Cassandra pod for the (short-lived) init container that enables JMX remote
	// authentication on Cassandra pods. This is only useful when authentication is enabled in the cluster.
	// The default is "busybox:1.34.1".
	// +optional
	// +kubebuilder:default={name:"busybox",tag:"1.34.1"}
	JmxInitContainerImage *images.Image `json:"jmxInitContainerImage,omitempty"`

	// Resources is the cpu and memory resources for the cassandra container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// SystemLoggerResources is the cpu and memory resources for the server-system-logger
	// container.
	// +optional
	SystemLoggerResources *corev1.ResourceRequirements `json:"systemLoggerResources,omitempty"`

	// CassandraConfig is configuration settings that are applied to cassandra.yaml and
	// jvm-options for 3.11.x or jvm-server-options for 4.x.
	// +optional
	CassandraConfig *CassandraConfig `json:"config,omitempty"`

	// StorageConfig is the persistent storage requirements for each Cassandra pod. This
	// includes everything under /var/lib/cassandra, namely the commit log and data
	// directories.
	// +optional
	StorageConfig *cassdcapi.StorageConfig `json:"storageConfig,omitempty"`

	// Networking enables host networking and configures a NodePort ports.
	// +optional
	Networking *cassdcapi.NetworkingConfig `json:"networking,omitempty"`

	// Racks is a list of named racks. Note that racks are used to create node affinity. //
	// +optional
	Racks []cassdcapi.Rack `json:"racks,omitempty"`

	// Datacenters a list of the DCs in the cluster.
	// +optional
	Datacenters []CassandraDatacenterTemplate `json:"datacenters,omitempty"`

	// CassandraTelemetry defines the desired state for telemetry resources in this K8ssandraCluster.
	// If telemetry configurations are defined, telemetry resources will be deployed to integrate with
	// a user-provided monitoring solution (at present, only support for Prometheus is available).
	// +optional
	CassandraTelemetry *telemetryapi.TelemetrySpec `json:"cassandraTelemetry,omitempty"`

	// MgmtAPIHeap defines the amount of memory devoted to the management
	// api heap.
	// +optional
	MgmtAPIHeap *resource.Quantity `json:"mgmtAPIHeap,omitempty"`
}

// +kubebuilder:pruning:PreserveUnknownFields

type CassandraDatacenterTemplate struct {
	Meta EmbeddedObjectMeta `json:"metadata,omitempty"`

	K8sContext string `json:"k8sContext,omitempty"`

	ServerImage string `json:"serverImage,omitempty"`

	// Size is the number Cassandra pods to deploy in this datacenter.
	// This number does not include Stargate instances.
	// +kubebuilder:validation:Minimum=1
	Size int32 `json:"size"`

	// ServerVersion is the Cassandra version.
	// +kubebuilder:validation:Pattern=(3\.11\.\d+)|(4\.0\.\d+)
	// +optional
	ServerVersion string `json:"serverVersion,omitempty"`

	// The image to use in each Cassandra pod for the (short-lived) init container that enables JMX remote
	// authentication on Cassandra pods. This is only useful when authentication is enabled in the cluster.
	// The default is "busybox:1.34.1".
	// +optional
	// +kubebuilder:default={name:"busybox",tag:"1.34.1"}
	JmxInitContainerImage *images.Image `json:"jmxInitContainerImage,omitempty"`

	// CassandraConfig is configuration settings that are applied to cassandra.yaml and
	// jvm-options for 3.11.x or jvm-server-options for 4.x.
	CassandraConfig *CassandraConfig `json:"config,omitempty"`

	// Resources is the cpu and memory resources for the cassandra container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// SystemLoggerResources is the cpu and memory resources for the server-system-logger
	// container.
	// +optional
	SystemLoggerResources *corev1.ResourceRequirements `json:"systemLoggerResources,omitempty"`

	// +optional
	Racks []cassdcapi.Rack `json:"racks,omitempty"`

	// Networking enables host networking and configures a NodePort ports.
	// +optional
	Networking *cassdcapi.NetworkingConfig `json:"networking,omitempty"`

	// StorageConfig is the persistent storage requirements for each Cassandra pod. This
	// includes everything under /var/lib/cassandra, namely the commit log and data
	// directories.
	// +optional
	StorageConfig *cassdcapi.StorageConfig `json:"storageConfig,omitempty"`

	// Stargate defines the desired deployment characteristics for Stargate in this datacenter. Leave nil to skip
	// deploying Stargate in this datacenter.
	// +optional
	Stargate *stargateapi.StargateDatacenterTemplate `json:"stargate,omitempty"`

	// Reaper defines the desired deployment characteristics for Reaper in this datacenter. Leave nil to skip
	// deploying Reaper in this datacenter.
	// +optional
	Reaper *reaperapi.ReaperDatacenterTemplate `json:"reaper,omitempty"`

	// MgmtAPIHeap defines the amount of memory devoted to the management
	// api heap.
	// +optional
	MgmtAPIHeap *resource.Quantity `json:"mgmtAPIHeap,omitempty"`

	// Telemetry defines the desired state for telemetry resources in this datacenter.
	// If telemetry configurations are defined, telemetry resources will be deployed to integrate with
	// a user-provided monitoring solution (at present, only support for Prometheus is available).
	// +optional
	CassandraTelemetry *telemetryapi.TelemetrySpec `json:"cassandraTelemetry,omitempty"`
}

type EmbeddedObjectMeta struct {
	// +optional
	Namespace string `json:"namespace,omitempty"`

	Name string `json:"name"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TODO Implement Stringer interface. It will helpful for debugging and testing.
type CassandraConfig struct {
	// +optional
	CassandraYaml CassandraYaml `json:"cassandraYaml,omitempty"`

	// +optional
	JvmOptions JvmOptions `json:"jvmOptions,omitempty"`
}

// CassandraYaml defines the contents of the cassandra.yaml file. For more info see:
// https://cassandra.apache.org/doc/latest/cassandra/configuration/cass_yaml_file.html
type CassandraYaml struct {
	// Exists in 3.11, 4.0, trunk
	// +optional
	AllocateTokensForKeyspace *string `json:"allocate_tokens_for_keyspace,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	AllocateTokensForLocalReplicationFactor *int `json:"allocate_tokens_for_local_replication_factor,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	AuditLoggingOptions *AuditLogOptions `json:"audit_logging_options,omitempty"`

	// Exists in trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	AuthReadConsistencyLevel *string `json:"auth_read_consistency_level,omitempty"`

	// Exists in trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	AuthWriteConsistencyLevel *string `json:"auth_write_consistency_level,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	Authenticator *string `json:"authenticator,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	Authorizer *string `json:"authorizer,omitempty"`

	// Exists in trunk
	// +optional
	AutoHintsCleanupEnabled *bool `json:"auto_hints_cleanup_enabled,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	AutoOptimiseFullRepairStreams *bool `json:"auto_optimise_full_repair_streams,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	AutoOptimiseIncRepairStreams *bool `json:"auto_optimise_inc_repair_streams,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	AutoOptimisePreviewRepairStreams *bool `json:"auto_optimise_preview_repair_streams,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	AutoSnapshot *bool `json:"auto_snapshot,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	AutocompactionOnStartupEnabled *bool `json:"autocompaction_on_startup_enabled,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	AutomaticSstableUpgrade *bool `json:"automatic_sstable_upgrade,omitempty"`

	// Exists in trunk
	// +optional
	AvailableProcessors *int `json:"available_processors,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BackPressureEnabled *bool `json:"back_pressure_enabled,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BackPressureStrategy *ParameterizedClass `json:"back_pressure_strategy,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BatchSizeFailThresholdInKb *int `json:"batch_size_fail_threshold_in_kb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BatchSizeWarnThresholdInKb *int `json:"batch_size_warn_threshold_in_kb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BatchlogReplayThrottleInKb *int `json:"batchlog_replay_throttle_in_kb,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	BlockForPeersInRemoteDcs *bool `json:"block_for_peers_in_remote_dcs,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	BlockForPeersTimeoutInSecs *int `json:"block_for_peers_timeout_in_secs,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	BufferPoolUseHeapIfExhausted *bool `json:"buffer_pool_use_heap_if_exhausted,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CasContentionTimeoutInMs *int `json:"cas_contention_timeout_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CdcEnabled *bool `json:"cdc_enabled,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CdcFreeSpaceCheckIntervalMs *int `json:"cdc_free_space_check_interval_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CdcRawDirectory *string `json:"cdc_raw_directory,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CdcTotalSpaceInMb *int `json:"cdc_total_space_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CheckForDuplicateRowsDuringCompaction *bool `json:"check_for_duplicate_rows_during_compaction,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CheckForDuplicateRowsDuringReads *bool `json:"check_for_duplicate_rows_during_reads,omitempty"`

	// Exists in trunk
	// +optional
	ClientErrorReportingExclusions *SubnetGroups `json:"client_error_reporting_exclusions,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ColumnIndexCacheSizeInKb *int `json:"column_index_cache_size_in_kb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ColumnIndexSizeInKb *int `json:"column_index_size_in_kb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogCompression *ParameterizedClass `json:"commitlog_compression,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogMaxCompressionBuffersInPool *int `json:"commitlog_max_compression_buffers_in_pool,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogPeriodicQueueSize *int `json:"commitlog_periodic_queue_size,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogSegmentSizeInMb *int `json:"commitlog_segment_size_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=periodic;batch;group
	// +optional
	CommitlogSync *string `json:"commitlog_sync,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogSyncBatchWindowInMs *string `json:"commitlog_sync_batch_window_in_ms,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	CommitlogSyncGroupWindowInMs *int `json:"commitlog_sync_group_window_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogSyncPeriodInMs *int `json:"commitlog_sync_period_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CommitlogTotalSpaceInMb *int `json:"commitlog_total_space_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CompactionLargePartitionWarningThresholdMb *int `json:"compaction_large_partition_warning_threshold_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CompactionThroughputMbPerSec *int `json:"compaction_throughput_mb_per_sec,omitempty"`

	// Exists in trunk
	// +optional
	CompactionTombstoneWarningThreshold *int `json:"compaction_tombstone_warning_threshold,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentCompactors *int `json:"concurrent_compactors,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentCounterWrites *int `json:"concurrent_counter_writes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	ConcurrentMaterializedViewBuilders *int `json:"concurrent_materialized_view_builders,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentMaterializedViewWrites *int `json:"concurrent_materialized_view_writes,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentReads *int `json:"concurrent_reads,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentReplicates *int `json:"concurrent_replicates,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	ConcurrentValidations *int `json:"concurrent_validations,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ConcurrentWrites *int `json:"concurrent_writes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	ConsecutiveMessageErrorsThreshold *int `json:"consecutive_message_errors_threshold,omitempty"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=disabled;warn;exception
	// +optional
	CorruptedTombstoneStrategy *string `json:"corrupted_tombstone_strategy,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterCacheKeysToSave *int `json:"counter_cache_keys_to_save,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterCacheSavePeriod *int `json:"counter_cache_save_period,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterCacheSizeInMb *int `json:"counter_cache_size_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CounterWriteRequestTimeoutInMs *int `json:"counter_write_request_timeout_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CredentialsCacheMaxEntries *int `json:"credentials_cache_max_entries,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CredentialsUpdateIntervalInMs *int `json:"credentials_update_interval_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CredentialsValidityInMs *int `json:"credentials_validity_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	CrossNodeTimeout *bool `json:"cross_node_timeout,omitempty"`

	// Exists in trunk
	// +optional
	DefaultKeyspaceRf *int `json:"default_keyspace_rf,omitempty"`

	// Exists in trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	DenylistConsistencyLevel *string `json:"denylist_consistency_level,omitempty"`

	// Exists in trunk
	// +optional
	DenylistInitialLoadRetrySeconds *int `json:"denylist_initial_load_retry_seconds,omitempty"`

	// Exists in trunk
	// +optional
	DenylistMaxKeysPerTable *int `json:"denylist_max_keys_per_table,omitempty"`

	// Exists in trunk
	// +optional
	DenylistMaxKeysTotal *int `json:"denylist_max_keys_total,omitempty"`

	// Exists in trunk
	// +optional
	DenylistRefreshSeconds *int `json:"denylist_refresh_seconds,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	DiagnosticEventsEnabled *bool `json:"diagnostic_events_enabled,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=auto;mmap;mmap_index_only;standard
	// +optional
	DiskAccessMode *string `json:"disk_access_mode,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DiskOptimizationEstimatePercentile *string `json:"disk_optimization_estimate_percentile,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DiskOptimizationPageCrossChance *string `json:"disk_optimization_page_cross_chance,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=ssd;spinning
	// +optional
	DiskOptimizationStrategy *string `json:"disk_optimization_strategy,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitch *bool `json:"dynamic_snitch,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitchBadnessThreshold *string `json:"dynamic_snitch_badness_threshold,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitchResetIntervalInMs *int `json:"dynamic_snitch_reset_interval_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	DynamicSnitchUpdateIntervalInMs *int `json:"dynamic_snitch_update_interval_in_ms,omitempty"`

	// Exists in trunk
	// +optional
	EnableDenylistRangeReads *bool `json:"enable_denylist_range_reads,omitempty"`

	// Exists in trunk
	// +optional
	EnableDenylistReads *bool `json:"enable_denylist_reads,omitempty"`

	// Exists in trunk
	// +optional
	EnableDenylistWrites *bool `json:"enable_denylist_writes,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableDropCompactStorage *bool `json:"enable_drop_compact_storage,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableMaterializedViews *bool `json:"enable_materialized_views,omitempty"`

	// Exists in trunk
	// +optional
	EnablePartitionDenylist *bool `json:"enable_partition_denylist,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableSasiIndexes *bool `json:"enable_sasi_indexes,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableScriptedUserDefinedFunctions *bool `json:"enable_scripted_user_defined_functions,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	EnableTransientReplication *bool `json:"enable_transient_replication,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableUserDefinedFunctions *bool `json:"enable_user_defined_functions,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EnableUserDefinedFunctionsThreads *bool `json:"enable_user_defined_functions_threads,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	EndpointSnitch *string `json:"endpoint_snitch,omitempty"`

	// Exists in trunk
	// +optional
	FailureDetector *string `json:"failure_detector,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	FileCacheEnabled *bool `json:"file_cache_enabled,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	FileCacheRoundUp *bool `json:"file_cache_round_up,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	FileCacheSizeInMb *int `json:"file_cache_size_in_mb,omitempty"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=none;fast;table
	// +optional
	FlushCompression *string `json:"flush_compression,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	FullQueryLoggingOptions *FullQueryLoggerOptions `json:"full_query_logging_options,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	GcLogThresholdInMs *int `json:"gc_log_threshold_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	GcWarnThresholdInMs *int `json:"gc_warn_threshold_in_ms,omitempty"`

	// Exists in trunk
	// +optional
	HintWindowPersistentEnabled *bool `json:"hint_window_persistent_enabled,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintedHandoffDisabledDatacenters *[]string `json:"hinted_handoff_disabled_datacenters,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintedHandoffEnabled *bool `json:"hinted_handoff_enabled,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintedHandoffThrottleInKb *int `json:"hinted_handoff_throttle_in_kb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintsCompression *ParameterizedClass `json:"hints_compression,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	HintsFlushPeriodInMs *int `json:"hints_flush_period_in_ms,omitempty"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE;NODE_LOCAL
	// +optional
	IdealConsistencyLevel *string `json:"ideal_consistency_level,omitempty"`

	// Exists in 3.11
	// +optional
	IndexInterval *int `json:"index_interval,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	IndexSummaryCapacityInMb *int `json:"index_summary_capacity_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	IndexSummaryResizeIntervalInMinutes *int `json:"index_summary_resize_interval_in_minutes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InitialRangeTombstoneListAllocationSize *int `json:"initial_range_tombstone_list_allocation_size,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	InterDcStreamThroughputOutboundMegabitsPerSec *int `json:"inter_dc_stream_throughput_outbound_megabits_per_sec,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	InterDcTcpNodelay *bool `json:"inter_dc_tcp_nodelay,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationReceiveQueueCapacityInBytes *int `json:"internode_application_receive_queue_capacity_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationReceiveQueueReserveEndpointCapacityInBytes *int `json:"internode_application_receive_queue_reserve_endpoint_capacity_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationReceiveQueueReserveGlobalCapacityInBytes *int `json:"internode_application_receive_queue_reserve_global_capacity_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationSendQueueCapacityInBytes *int `json:"internode_application_send_queue_capacity_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationSendQueueReserveEndpointCapacityInBytes *int `json:"internode_application_send_queue_reserve_endpoint_capacity_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeApplicationSendQueueReserveGlobalCapacityInBytes *int `json:"internode_application_send_queue_reserve_global_capacity_in_bytes,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	InternodeAuthenticator *string `json:"internode_authenticator,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=all;none;dc
	// +optional
	InternodeCompression *string `json:"internode_compression,omitempty"`

	// Exists in trunk
	// +optional
	InternodeErrorReportingExclusions *SubnetGroups `json:"internode_error_reporting_exclusions,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeMaxMessageSizeInBytes *int `json:"internode_max_message_size_in_bytes,omitempty"`

	// Exists in 3.11
	// +optional
	InternodeRecvBuffSizeInBytes *int `json:"internode_recv_buff_size_in_bytes,omitempty"`

	// Exists in 3.11
	// +optional
	InternodeSendBuffSizeInBytes *int `json:"internode_send_buff_size_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeSocketReceiveBufferSizeInBytes *int `json:"internode_socket_receive_buffer_size_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeSocketSendBufferSizeInBytes *int `json:"internode_socket_send_buffer_size_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeStreamingTcpUserTimeoutInMs *int `json:"internode_streaming_tcp_user_timeout_in_ms,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeTcpConnectTimeoutInMs *int `json:"internode_tcp_connect_timeout_in_ms,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	InternodeTcpUserTimeoutInMs *int `json:"internode_tcp_user_timeout_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	KeyCacheKeysToSave *int `json:"key_cache_keys_to_save,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	KeyCacheMigrateDuringCompaction *bool `json:"key_cache_migrate_during_compaction,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	KeyCacheSavePeriod *int `json:"key_cache_save_period,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	KeyCacheSizeInMb *int `json:"key_cache_size_in_mb,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	KeyspaceCountWarnThreshold *int `json:"keyspace_count_warn_threshold,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	MaxConcurrentAutomaticSstableUpgrades *int `json:"max_concurrent_automatic_sstable_upgrades,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxHintWindowInMs *int `json:"max_hint_window_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxHintsDeliveryThreads *int `json:"max_hints_delivery_threads,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxHintsFileSizeInMb *int `json:"max_hints_file_size_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxMutationSizeInKb *int `json:"max_mutation_size_in_kb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxStreamingRetries *int `json:"max_streaming_retries,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MaxValueSizeInMb *int `json:"max_value_size_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=unslabbed_heap_buffers;unslabbed_heap_buffers_logged;heap_buffers;offheap_buffers;offheap_objects
	// +optional
	MemtableAllocationType *string `json:"memtable_allocation_type,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableCleanupThreshold *string `json:"memtable_cleanup_threshold,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableFlushWriters *int `json:"memtable_flush_writers,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableHeapSpaceInMb *int `json:"memtable_heap_space_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MemtableOffheapSpaceInMb *int `json:"memtable_offheap_space_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	MinFreeSpacePerDriveInMb *int `json:"min_free_space_per_drive_in_mb,omitempty"`

	// Exists in trunk
	// +optional
	MinimumKeyspaceRf *int `json:"minimum_keyspace_rf,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	NativeTransportAllowOlderProtocols *bool `json:"native_transport_allow_older_protocols,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportFlushInBatchesLegacy *bool `json:"native_transport_flush_in_batches_legacy,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	NativeTransportIdleTimeoutInMs *int `json:"native_transport_idle_timeout_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentConnections *int `json:"native_transport_max_concurrent_connections,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentConnectionsPerIp *int `json:"native_transport_max_concurrent_connections_per_ip,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentRequestsInBytes *int `json:"native_transport_max_concurrent_requests_in_bytes,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxConcurrentRequestsInBytesPerIp *int `json:"native_transport_max_concurrent_requests_in_bytes_per_ip,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxFrameSizeInMb *int `json:"native_transport_max_frame_size_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxNegotiableProtocolVersion *int `json:"native_transport_max_negotiable_protocol_version,omitempty"`

	// Exists in trunk
	// +optional
	NativeTransportMaxRequestsPerSecond *int `json:"native_transport_max_requests_per_second,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NativeTransportMaxThreads *int `json:"native_transport_max_threads,omitempty"`

	// Exists in trunk
	// +optional
	NativeTransportRateLimitingEnabled *bool `json:"native_transport_rate_limiting_enabled,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	NativeTransportReceiveQueueCapacityInBytes *int `json:"native_transport_receive_queue_capacity_in_bytes,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	NetworkAuthorizer *string `json:"network_authorizer,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	NetworkingCacheSizeInMb *int `json:"networking_cache_size_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	NumTokens *int `json:"num_tokens,omitempty"`

	// Exists in 3.11
	// +optional
	OtcBacklogExpirationIntervalMs *int `json:"otc_backlog_expiration_interval_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	OtcCoalescingEnoughCoalescedMessages *int `json:"otc_coalescing_enough_coalesced_messages,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	OtcCoalescingStrategy *string `json:"otc_coalescing_strategy,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	OtcCoalescingWindowUs *int `json:"otc_coalescing_window_us,omitempty"`

	// Exists in trunk
	// +optional
	PaxosCacheSizeInMb *int `json:"paxos_cache_size_in_mb,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	PeriodicCommitlogSyncLagBlockInMs *int `json:"periodic_commitlog_sync_lag_block_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PermissionsCacheMaxEntries *int `json:"permissions_cache_max_entries,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PermissionsUpdateIntervalInMs *int `json:"permissions_update_interval_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PermissionsValidityInMs *int `json:"permissions_validity_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PhiConvictThreshold *string `json:"phi_convict_threshold,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	PreparedStatementsCacheSizeMb *int `json:"prepared_statements_cache_size_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RangeRequestTimeoutInMs *int `json:"range_request_timeout_in_ms,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	RangeTombstoneListGrowthFactor *string `json:"range_tombstone_list_growth_factor,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ReadRequestTimeoutInMs *int `json:"read_request_timeout_in_ms,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	RejectRepairCompactionThreshold *int `json:"reject_repair_compaction_threshold,omitempty"`

	// Exists in: 4.0, trunk
	// +kubebuilder:validation:Enum=queue;reject
	// +optional
	RepairCommandPoolFullStrategy *string `json:"repair_command_pool_full_strategy,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	RepairCommandPoolSize *int `json:"repair_command_pool_size,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RepairSessionMaxTreeDepth *int `json:"repair_session_max_tree_depth,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	RepairSessionSpaceInMb *int `json:"repair_session_space_in_mb,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	RepairedDataTrackingForPartitionReadsEnabled *bool `json:"repaired_data_tracking_for_partition_reads_enabled,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	RepairedDataTrackingForRangeReadsEnabled *bool `json:"repaired_data_tracking_for_range_reads_enabled,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	ReplicaFilteringProtection *ReplicaFilteringProtectionOptions `json:"replica_filtering_protection,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	ReportUnconfirmedRepairedDataMismatches *bool `json:"report_unconfirmed_repaired_data_mismatches,omitempty"`

	// Exists in 3.11
	// +optional
	RequestScheduler *string `json:"request_scheduler,omitempty"`

	// Exists in 3.11
	// +kubebuilder:validation:Enum=keyspace
	// +optional
	RequestSchedulerId *string `json:"request_scheduler_id,omitempty"`

	// Exists in 3.11
	// +optional
	RequestSchedulerOptions *RequestSchedulerOptions `json:"request_scheduler_options,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RequestTimeoutInMs *int `json:"request_timeout_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RoleManager *string `json:"role_manager,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RolesCacheMaxEntries *int `json:"roles_cache_max_entries,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RolesUpdateIntervalInMs *int `json:"roles_update_interval_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RolesValidityInMs *int `json:"roles_validity_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheClassName *string `json:"row_cache_class_name,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheKeysToSave *int `json:"row_cache_keys_to_save,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheSavePeriod *int `json:"row_cache_save_period,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	RowCacheSizeInMb *int `json:"row_cache_size_in_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SlowQueryLogTimeoutInMs *int `json:"slow_query_log_timeout_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SnapshotBeforeCompaction *bool `json:"snapshot_before_compaction,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	SnapshotLinksPerSecond *int `json:"snapshot_links_per_second,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SnapshotOnDuplicateRowDetection *bool `json:"snapshot_on_duplicate_row_detection,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	SnapshotOnRepairedDataMismatch *bool `json:"snapshot_on_repaired_data_mismatch,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	SstablePreemptiveOpenIntervalInMb *int `json:"sstable_preemptive_open_interval_in_mb,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	StreamEntireSstables *bool `json:"stream_entire_sstables,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	StreamThroughputOutboundMegabitsPerSec *int `json:"stream_throughput_outbound_megabits_per_sec,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	StreamingConnectionsPerHost *int `json:"streaming_connections_per_host,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	StreamingKeepAlivePeriodInSecs *int `json:"streaming_keep_alive_period_in_secs,omitempty"`

	// Exists in 3.11
	// +optional
	StreamingSocketTimeoutInMs *int `json:"streaming_socket_timeout_in_ms,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	TableCountWarnThreshold *int `json:"table_count_warn_threshold,omitempty"`

	// Exists in 3.11
	// +optional
	ThriftFramedTransportSizeInMb *int `json:"thrift_framed_transport_size_in_mb,omitempty"`

	// Exists in 3.11
	// +optional
	ThriftMaxMessageLengthInMb *int `json:"thrift_max_message_length_in_mb,omitempty"`

	// Exists in 3.11
	// +optional
	ThriftPreparedStatementsCacheSizeMb *int `json:"thrift_prepared_statements_cache_size_mb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TombstoneFailureThreshold *int `json:"tombstone_failure_threshold,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TombstoneWarnThreshold *int `json:"tombstone_warn_threshold,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TracetypeQueryTtl *int `json:"tracetype_query_ttl,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TracetypeRepairTtl *int `json:"tracetype_repair_ttl,omitempty"`

	// Exists in trunk
	// +optional
	TrackWarnings *TrackWarnings `json:"track_warnings,omitempty"`

	// Exists in trunk
	// +optional
	TraverseAuthFromRoot *bool `json:"traverse_auth_from_root,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TrickleFsync *bool `json:"trickle_fsync,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TrickleFsyncIntervalInKb *int `json:"trickle_fsync_interval_in_kb,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	TruncateRequestTimeoutInMs *int `json:"truncate_request_timeout_in_ms,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	UnloggedBatchAcrossPartitionsWarnThreshold *int `json:"unlogged_batch_across_partitions_warn_threshold,omitempty"`

	// Exists in trunk
	// +optional
	UseDeterministicTableId *bool `json:"use_deterministic_table_id,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	UseOffheapMerkleTrees *bool `json:"use_offheap_merkle_trees,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	UserDefinedFunctionFailTimeout *int `json:"user_defined_function_fail_timeout,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	UserDefinedFunctionWarnTimeout *int `json:"user_defined_function_warn_timeout,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +kubebuilder:validation:Enum=ignore;die;die_immediate
	// +optional
	UserFunctionTimeoutPolicy *string `json:"user_function_timeout_policy,omitempty"`

	// Exists in: 4.0, trunk
	// +optional
	ValidationPreviewPurgeHeadStartInSec *int `json:"validation_preview_purge_head_start_in_sec,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	WindowsTimerInterval *int `json:"windows_timer_interval,omitempty"`

	// Exists in 3.11, 4.0, trunk
	// +optional
	WriteRequestTimeoutInMs *int `json:"write_request_timeout_in_ms,omitempty"`
}

type JvmOptions struct {
	// +optional
	HeapSize *resource.Quantity `json:"heapSize,omitempty"`

	// +optional
	HeapNewGenSize *resource.Quantity `json:"heapNewGenSize,omitempty"`

	// +optional
	AdditionalOptions []string `json:"additionalOptions,omitempty"`
}

type ParameterizedClass struct {
	ClassName  string             `json:"class_name"`
	Parameters *map[string]string `json:"parameters,omitempty"`
}

type ReplicaFilteringProtectionOptions struct {
	CachedRowsWarnThreshold *int `json:"cached_rows_warn_threshold,omitempty"`
	CachedRowsFailThreshold *int `json:"cached_rows_fail_threshold,omitempty"`
}

type RequestSchedulerOptions struct {
	ThrottleLimit *int            `json:"throttle_limit,omitempty"`
	DefaultWeight *int            `json:"default_weight,omitempty"`
	Weights       *map[string]int `json:"weights,omitempty"`
}

type AuditLogOptions struct {
	Enabled            bool                `json:"enabled"`
	Logger             *ParameterizedClass `json:"logger,omitempty"`
	IncludedKeyspaces  *string             `json:"included_keyspaces,omitempty"`
	ExcludedKeyspaces  string              `json:"excluded_keyspaces,omitempty"`
	IncludedCategories *string             `json:"included_categories,omitempty"`
	ExcludedCategories string              `json:"excluded_categories,omitempty"`
	IncludedUsers      *string             `json:"included_users,omitempty"`
	ExcludedUsers      *string             `json:"excluded_users,omitempty"`
	RollCycle          *string             `json:"roll_cycle,omitempty"`
	Block              *bool               `json:"block,omitempty"`
	MaxQueueWeight     *int                `json:"max_queue_weight,omitempty"`
	MaxLogSize         *int                `json:"max_log_size,omitempty"`
	ArchiveCommand     *string             `json:"archive_command,omitempty"`
	MaxArchiveRetries  *int                `json:"max_archive_retries,omitempty"`
}

type FullQueryLoggerOptions struct {
	ArchiveCommand    *string `json:"archive_command,omitempty"`
	RollCycle         *string `json:"roll_cycle,omitempty"`
	Block             *bool   `json:"block,omitempty"`
	MaxQueueWeight    *int    `json:"max_queue_weight,omitempty"`
	MaxLogSize        *int    `json:"max_log_size,omitempty"`
	MaxArchiveRetries *int    `json:"max_archive_retries,omitempty"`
	LogDir            *string `json:"log_dir,omitempty"`
}

type SubnetGroups struct {
	Subnets []Group `json:"subnets"`
}

type Group struct {
	Subnet string `json:"subnet"`
}

type TrackWarnings struct {
	Enabled             bool `json:"enabled"`
	CoordinatorReadSize *int `json:"coordinator_read_size,omitempty"`
	LocalReadSize       *int `json:"local_read_size,omitempty"`
	RowIndexSize        *int `json:"row_index_size,omitempty"`
}

func (s *K8ssandraClusterStatus) GetConditionStatus(conditionType K8ssandraClusterConditionType) corev1.ConditionStatus {
	for _, condition := range s.Conditions {
		if condition.Type == conditionType {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func (s *K8ssandraClusterStatus) SetCondition(condition K8ssandraClusterCondition) {
	for i, c := range s.Conditions {
		if c.Type == condition.Type {
			s.Conditions[i] = condition
			return
		}
	}
	s.Conditions = append(s.Conditions, condition)
}

func init() {
	SchemeBuilder.Register(&K8ssandraCluster{}, &K8ssandraClusterList{})
}
