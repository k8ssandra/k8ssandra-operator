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
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"k8s.io/apimachinery/pkg/api/resource"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type CassandraConfig struct {
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	CassandraYaml unstructured.Unstructured `json:"cassandraYaml,omitempty"`

	// +optional
	JvmOptions JvmOptions `json:"jvmOptions,omitempty"`

	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	DseYaml unstructured.Unstructured `json:"dseYaml,omitempty"`
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
	InitialHeapSize *resource.Quantity `json:"heap_initial_size,omitempty" cass-config:"^3.11.x:jvm-options/initial_heap_size;>=4.x,dse@>=6.8.x:jvm-server-options/initial_heap_size"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Xmx.
	// +optional
	MaxHeapSize *resource.Quantity `json:"heap_max_size,omitempty" cass-config:"^3.11.x:jvm-options/max_heap_size;>=4.x,dse@>=6.8.x:jvm-server-options/max_heap_size"`

	// GENERAL VM OPTIONS

	// Enable assertions.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -ea.
	// +optional
	EnableAssertions *bool `json:"vm_enable_assertions,omitempty" cass-config:"^3.11.x:jvm-options/enable_assertions;>=4.x,dse@>=6.8.x:jvm-server-options/enable_assertions"`

	// Enable thread priorities.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UseThreadPriorities.
	// +optional
	EnableThreadPriorities *bool `json:"vm_enable_thread_priorities,omitempty" cass-config:"^3.11.x:jvm-options/use_thread_priorities;>=4.x,dse@>=6.8.x:jvm-server-options/use_thread_priorities"`

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
	HeapDumpOnOutOfMemoryError *bool `json:"vm_heap_dump_on_out_of_memory_error,omitempty" cass-config:"^3.11.x:jvm-options/heap_dump_on_out_of_memory_error;>=4.x,dse@>=6.8.x:jvm-server-options/heap_dump_on_out_of_memory_error"`

	// Per-thread stack size.
	// Defaults to 256Ki.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Xss.
	// +optional
	PerThreadStackSize *resource.Quantity `json:"vm_per_thread_stack_size,omitempty" cass-config:"^3.11.x:jvm-options/per_thread_stack_size;>=4.x,dse@>=6.8.x:jvm-server-options/per_thread_stack_size"`

	// The size of interned string table. Larger sizes are beneficial to gossip.
	// Defaults to 1000003.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:StringTableSize.
	// +optional
	StringTableSize *resource.Quantity `json:"vm_string_table_size,omitempty" cass-config:"^3.11.x:jvm-options/string_table_size;>=4.x,dse@>=6.8.x:jvm-server-options/string_table_size"`

	// Ensure all memory is faulted and zeroed on startup.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+AlwaysPreTouch.
	// +optional
	AlwaysPreTouch *bool `json:"vm_always_pre_touch,omitempty" cass-config:"^3.11.x:jvm-options/always_pre_touch;>=4.x,dse@>=6.8.x:jvm-server-options/always_pre_touch"`

	// Disable biased locking to avoid biased lock revocation pauses.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:-UseBiasedLocking.
	// Note: the Cass Config Builder option is named use_biased_locking, but setting it to true
	// disables biased locking.
	// +optional
	DisableBiasedLocking *bool `json:"vm_disable_biased_locking,omitempty" cass-config:"^3.11.x:jvm-options/use_biased_locking;>=4.x,dse@>=6.8.x:jvm-server-options/use-biased-locking"`

	// Enable thread-local allocation blocks.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UseTLAB.
	// +optional
	UseTlab *bool `json:"vm_use_tlab,omitempty" cass-config:"^3.11.x:jvm-options/use_tlb;>=4.x,dse@>=6.8.x:jvm-server-options/use_tlb"`

	// Allow resizing of thread-local allocation blocks.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+ResizeTLAB.
	// +optional
	ResizeTlab *bool `json:"vm_resize_tlab,omitempty" cass-config:"^3.11.x:jvm-options/resize_tlb;>=4.x,dse@>=6.8.x:jvm-server-options/resize_tlb"`

	// Disable hsperfdata mmap'ed file.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+PerfDisableSharedMem.
	// +optional
	DisablePerfSharedMem *bool `json:"vm_disable_perf_shared_mem,omitempty" cass-config:"^3.11.x:jvm-options/perf_disable_shared_mem;>=4.x,dse@>=6.8.x:jvm-server-options/perf_disable_shared_mem"`

	// Prefer binding to IPv4 network interfaces.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Djava.net.preferIPv4Stack=true.
	// +optional
	PreferIpv4 *bool `json:"vm_prefer_ipv4,omitempty" cass-config:"^3.11.x:jvm-options/java_net_prefer_ipv4_stack;>=4.x,dse@>=6.8.x:jvm-server-options/java_net_prefer_ipv4_stack"`

	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UseNUMA.
	// +optional
	UseNuma *bool `json:"vm_use_numa,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/use_numa"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.printHeapHistogramOnOutOfMemoryError.
	// +optional
	PrintHeapHistogramOnOutOfMemoryError *bool `json:"vm_print_heap_histogram_on_out_of_memory_error,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/print_heap_histogram_on_out_of_memory_error"`

	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+ExitOnOutOfMemoryError.
	// +optional
	ExitOnOutOfMemoryError *bool `json:"vm_exit_on_out_of_memory_error,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/exit_on_out_of_memory_error"`

	// Disabled by default. Requires `exit_on_out_of_memory_error` to be disabled..
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+CrashOnOutOfMemoryError.
	// +optional
	CrashOnOutOfMemoryError *bool `json:"vm_crash_on_out_of_memory_error,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/crash_on_out_of_memory_error"`

	// Defaults to 300000 milliseconds.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:GuaranteedSafepointInterval.
	// +optional
	GuaranteedSafepointIntervalMs *int `json:"vm_guaranteed_safepoint_interval_ms,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/guaranteed-safepoint-interval"`

	// Defaults to 65536.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dio.netty.eventLoop.maxPendingTasks.
	// +optional
	NettyEventloopMaxPendingTasks *int `json:"netty_eventloop_maxpendingtasks,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/io_netty_eventloop_maxpendingtasks"`

	// Netty setting `io.netty.tryReflectionSetAccessible`.
	// Defaults to true.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -Dio.netty.tryReflectionSetAccessible=true.
	// +optional
	NettyTryReflectionSetAccessible *bool `json:"netty_try_reflection_set_accessible,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm11-server-options/io_netty_try_reflection_set_accessible"`

	// Defaults to 1048576.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Djdk.nio.maxCachedBufferSize.
	// +optional
	NioMaxCachedBufferSize *resource.Quantity `json:"nio_maxcachedbuffersize,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/jdk_nio_maxcachedbuffersize"`

	// Align direct memory allocations on page boundaries.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dsun.nio.PageAlignDirectMemory=true.
	// +optional
	NioAlignDirectMemory *bool `json:"nio_align_direct_memory,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/page-align-direct-memory"`

	// Allow the current VM to attach to itself.
	// Defaults to true.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -Djdk.attach.allowAttachSelf=true.
	// +optional
	JdkAllowAttachSelf *bool `json:"jdk_allow_attach_self,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm11-server-options/jdk_attach_allow_attach_self"`

	// CASSANDRA STARTUP OPTIONS

	// Available CPU processors.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.available_processors.
	// +optional
	AvailableProcessors *int `json:"cassandra_available_processors,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_available_processors;>=4.x,dse@>=6.8.x:jvm-server-options/cassandra_available_processors"`

	// Enable pluggable metrics reporter.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.metricsReporterConfigFile.
	// +optional
	// TODO mountable directory
	MetricsReporterConfigFile *string `json:"cassandra_metrics_reporter_config_file,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_metrics_reporter_config_file;>=4.x,dse@>=6.8.x:jvm-server-options/cassandra_metrics_reporter_config_file"`

	// Amount of time in milliseconds that a node waits before joining the ring.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.ring_delay_ms.
	// +optional
	RingDelayMs *int `json:"cassandra_ring_delay_ms,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_ring_delay_ms;>=4.x,dse@>=6.8.x:jvm-server-options/cassandra_ring_delay_ms"`

	// Default location for the trigger JARs.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.triggers_dir.
	// +optional
	// TODO mountable directory
	TriggersDirectory *string `json:"cassandra_triggers_directory,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_triggers_dir;>=4.x,dse@>=6.8.x:jvm-server-options/cassandra_triggers_dir"`

	// For testing new compaction and compression strategies.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.write_survey.
	// +optional
	WriteSurvey *bool `json:"cassandra_write_survey,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_write_survey;>=4.x,dse@>=6.8.x:jvm-server-options/cassandra_write_survey"`

	// Disable remote configuration via JMX of auth caches.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.disable_auth_caches_remote_configuration.
	// +optional
	DisableAuthCachesRemoteConfiguration *bool `json:"cassandra_disable_auth_caches_remote_configuration,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_disable_auth_caches_remote_configuration;>=4.x,dse@>=6.8.x:jvm-server-options/cassandra_disable_auth_caches_remote_configuration"`

	// Disable dynamic calculation of the page size used when indexing an entire partition (during
	// initial index build/rebuild).
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.force_default_indexing_page_size.
	// +optional
	ForceDefaultIndexingPageSize *bool `json:"cassandra_force_default_indexing_page_size,omitempty" cass-config:"^3.11.x:jvm-options/cassandra_force_default_indexing_page_size;>=4.x,dse@>=6.8.x:jvm-server-options/cassandra_force_default_indexing_page_size"`

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
	ExpirationDateOverflowPolicy *string `json:"cassandra_expiration_date_overflow_policy,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/cassandra_expiration_date_overflow_policy"`

	// Imposes an upper bound on hint lifetime below the normal min gc_grace_seconds.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -Dcassandra.maxHintTTL.
	// +optional
	MaxHintTtlSeconds *int `json:"cassandra_max_hint_ttl_seconds,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/cassandra_max_hint_ttl"`

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
	GarbageCollector *string `json:"gc,omitempty" cass-config:"^3.11.x:jvm-options/garbage_collector;>=4.x,dse@>=6.8.x:jvm11-server-options/garbage_collector"`

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
	G1RSetUpdatingPauseTimePercent *int `json:"gc_g1_rset_updating_pause_time_percent,omitempty" cass-config:"^3.11.x:jvm-options/g1r_set_updating_pause_time_percent;>=4.x,dse@>=6.8.x:jvm11-server-options/g1r_set_updating_pause_time_percent"`

	// G1GC Max GC Pause in milliseconds. Defaults to 500. Can only be used when G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:MaxGCPauseMillis.
	// +optional
	G1MaxGcPauseMs *int `json:"gc_g1_max_gc_pause_ms,omitempty" cass-config:"^3.11.x:jvm-options/max_gc_pause_millis;>=4.x,dse@>=6.8.x:jvm11-server-options/max_gc_pause_millis"`

	// Initiating Heap Occupancy Percentage. Can only be used when G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:InitiatingHeapOccupancyPercent.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	G1InitiatingHeapOccupancyPercent *int `json:"gc_g1_initiating_heap_occupancy_percent,omitempty" cass-config:"^3.11.x:jvm-options/initiating_heap_occupancy_percent;>=4.x,dse@>=6.8.x:jvm11-server-options/initiating_heap_occupancy_percent"`

	// Parallel GC Threads. Can only be used when G1 garbage collector is used.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:ParallelGCThreads.
	// +optional
	G1ParallelGcThreads *int `json:"gc_g1_parallel_threads,omitempty" cass-config:"^3.11.x:jvm-options/parallel_gc_threads;>=4.x,dse@>=6.8.x:jvm11-server-options/parallel_gc_threads"`

	// Concurrent GC Threads. Can only be used when G1 garbage collector is used.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm11-server.options.
	// Corresponds to: -XX:ConcGCThreads.
	// +optional
	G1ConcGcThreads *int `json:"gc_g1_conc_threads,omitempty" cass-config:"^3.11.x:jvm-options/conc_gc_threads;>=4.x,dse@>=6.8.x:jvm11-server-options/conc_gc_threads"`

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
	JmxPort *int `json:"jmx_port,omitempty" cass-config:"^3.11.x:jvm-options/jmx-port;>=4.x,dse@>=6.8.x:jvm-server-options/jmx-port"`

	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Possible values for 3.11 include `local-no-auth`, `remote-no-auth`, and `remote-dse-unified-auth`. Defaults to `local-no-auth`.
	// Possible values for 4.0 include `local-no-auth`, `remote-no-auth`. Defaults to `local-no-auth`.
	// +optional
	JmxConnectionType *string `json:"jmx_connection_type,omitempty" cass-config:"^3.11.x:jvm-options/jmx-connection-type;>=4.x,dse@>=6.8.x:jvm-server-options/jmx-connection-type"`

	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Defaults to false.
	// Valid only when JmxConnectionType is "remote-no-auth", "remote-dse-unified-auth".
	// +optional
	JmxRemoteSsl *bool `json:"jmx_remote_ssl,omitempty" cass-config:"^3.11.x:jvm-options/jmx-remote-ssl;>=4.x,dse@>=6.8.x:jvm-server-options/jmx-remote-ssl"`

	// Remote SSL options.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// +optional
	JmxRemoteSslOpts *string `json:"jmx_remote_ssl_opts,omitempty" cass-config:"^3.11.x:jvm-options/jmx-remote-ssl-opts;>=4.x,dse@>=6.8.x:jvm-server-options/jmx-remote-ssl-opts"`

	// Require Client Authentication for remote SSL? Defaults to false.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// +optional
	JmxRemoteSslRequireClientAuth *bool `json:"jmx_remote_ssl_require_client_auth,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/jmx-remote-ssl-require-client-auth"`

	// DEBUG OPTIONS

	// Unlock commercial features.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UnlockCommercialFeatures.
	// +optional
	UnlockCommercialFeatures *bool `json:"debug_unlock_commercial_features,omitempty" cass-config:"^3.11.x:jvm-options/unlock_commercial_features;>=4.x,dse@>=6.8.x:jvm-server-options/unlock_commercial_features"`

	// Enable Flight Recorder (Use in production is subject to Oracle licensing).
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+FlightRecorder.
	// +optional
	EnableFlightRecorder *bool `json:"debug_enable_flight_recorder,omitempty" cass-config:"^3.11.x:jvm-options/flight_recorder;>=4.x,dse@>=6.8.x:jvm-server-options/flight_recorder"`

	// Listen for JVM remote debuggers on port 1414.
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 3.11 in jvm.options.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=1414".
	// +optional
	ListenForRemoteDebuggers *bool `json:"debug_listen_remote_debuggers,omitempty" cass-config:"^3.11.x:jvm-options/agent_lib_jdwp;>=4.x,dse@>=6.8.x:jvm-server-options/agent_lib_jdwp"`

	// Disable honoring user code @Contended annotations.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:-RestrictContended.
	// +optional
	DisableContendedAnnotations *bool `json:"debug_disable_contended_annotations,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/restrict-contended"`

	// Whether the compiler should generate the necessary metadata for the parts of the code not at
	// safe points as well. For use with Flight Recorder.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+DebugNonSafepoints.
	// +optional
	DebugNonSafepoints *bool `json:"debug_non_safepoints,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/debug-non-safepoints"`

	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+UnlockDiagnosticVMOptions.
	// +optional
	UnlockDiagnosticVmOptions *bool `json:"debug_unlock_diagnostic_vm_options,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/unlock-diagnostic-vm-options"`

	// Make Cassandra JVM log internal method compilation (developers only).
	// Disabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+LogCompilation.
	// +optional
	LogCompilation *bool `json:"debug_log_compilation,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/log_compilation"`

	// Preserve Frame Pointer.
	// Enabled by default.
	// Cass Config Builder: supported for Cassandra 4.0 in jvm-server.options.
	// Corresponds to: -XX:+PreserveFramePointer.
	// +optional
	PreserveFramePointer *bool `json:"debug_preserve_frame_pointer,omitempty" cass-config:">=4.x,dse@>=6.8.x:jvm-server-options/preserve-frame-pointer"`

	// Additional, arbitrary JVM options which are written into the cassandra-env.sh file.
	// +optional
	AdditionalOptions []string `json:"additionalOptions,omitempty" cass-config:"cassandra-env-sh/additional-jvm-opts"`

	// Jvm11ServerOptions are additional options that will be passed on to the jvm11-server-options file.
	// +optional
	AdditionalJvm11ServerOptions []string `json:"additionalJvm11ServerOptions,omitempty" cass-config:"jvm11-server-options/additional-jvm-opts"`

	// Jvm8ServerOptions are additional options that will be passed on to the jvm8-server-options file.
	// +optional
	AdditionalJvm8ServerOptions []string `json:"additionalJvm8ServerOptions,omitempty" cass-config:"jvm8-server-options/additional-jvm-opts"`

	// JvmServerOptions are additional options that will be passed on to the jvm-server-options file.
	// +optional
	AdditionalJvmServerOptions []string `json:"additionalJvmServerOptions,omitempty" cass-config:"jvm-server-options/additional-jvm-opts"`
}

type ParameterizedClass struct {
	ClassName  string             `json:"class_name" cass-config:"class_name"`
	Parameters *map[string]string `json:"parameters,omitempty" cass-config:"parameters"`
}

type ReplicaFilteringProtectionOptions struct {
	CachedRowsWarnThreshold *int `json:"cached_rows_warn_threshold,omitempty" cass-config:"cached_rows_warn_threshold"`
	CachedRowsFailThreshold *int `json:"cached_rows_fail_threshold,omitempty" cass-config:"cached_rows_fail_threshold"`
}

type RequestSchedulerOptions struct {
	ThrottleLimit *int            `json:"throttle_limit,omitempty" cass-config:"throttle_limit"`
	DefaultWeight *int            `json:"default_weight,omitempty" cass-config:"default_weight"`
	Weights       *map[string]int `json:"weights,omitempty" cass-config:"weights"`
}

type AuditLogOptions struct {
	Enabled            bool                `json:"enabled" cass-config:"enabled;retainzero"`
	Logger             *ParameterizedClass `json:"logger,omitempty" cass-config:"logger;recurse"`
	IncludedKeyspaces  *string             `json:"included_keyspaces,omitempty" cass-config:"included_keyspaces"`
	ExcludedKeyspaces  *string             `json:"excluded_keyspaces,omitempty" cass-config:"excluded_keyspaces"`
	IncludedCategories *string             `json:"included_categories,omitempty" cass-config:"included_categories"`
	ExcludedCategories *string             `json:"excluded_categories,omitempty" cass-config:"excluded_categories"`
	IncludedUsers      *string             `json:"included_users,omitempty" cass-config:"included_users"`
	ExcludedUsers      *string             `json:"excluded_users,omitempty" cass-config:"excluded_users"`
	RollCycle          *string             `json:"roll_cycle,omitempty" cass-config:"roll_cycle"`
	Block              *bool               `json:"block,omitempty" cass-config:"block"`
	MaxQueueWeight     *int                `json:"max_queue_weight,omitempty" cass-config:"max_queue_weight"`
	MaxLogSize         *int                `json:"max_log_size,omitempty" cass-config:"max_log_size"`
	ArchiveCommand     *string             `json:"archive_command,omitempty" cass-config:"archive_command"`
	MaxArchiveRetries  *int                `json:"max_archive_retries,omitempty" cass-config:"max_archive_retries"`
}

type FullQueryLoggerOptions struct {
	ArchiveCommand    *string `json:"archive_command,omitempty" cass-config:"archive_command"`
	RollCycle         *string `json:"roll_cycle,omitempty" cass-config:"roll_cycle"`
	Block             *bool   `json:"block,omitempty" cass-config:"block"`
	MaxQueueWeight    *int    `json:"max_queue_weight,omitempty" cass-config:"max_queue_weight"`
	MaxLogSize        *int    `json:"max_log_size,omitempty" cass-config:"max_log_size"`
	MaxArchiveRetries *int    `json:"max_archive_retries,omitempty" cass-config:"max_archive_retries"`
	LogDir            *string `json:"log_dir,omitempty" cass-config:"log_dir"`
}

type SubnetGroups struct {
	Subnets []string `json:"subnets" cass-config:"subnets"`
}

type TrackWarnings struct {
	Enabled             bool `json:"enabled" cass-config:"enabled;retainzero"`
	CoordinatorReadSize *int `json:"coordinator_read_size,omitempty" cass-config:"coordinator_read_size"`
	LocalReadSize       *int `json:"local_read_size,omitempty" cass-config:"local_read_size"`
	RowIndexSize        *int `json:"row_index_size,omitempty" cass-config:"row_index_size"`
}
