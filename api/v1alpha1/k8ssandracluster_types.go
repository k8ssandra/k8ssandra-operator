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
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// K8ssandraClusterSpec defines the desired state of K8ssandraCluster
type K8ssandraClusterSpec struct {
	K8sContextsSecret string `json:"k8sContextsSecret,omitempty"`

	Cassandra *CassandraClusterTemplate `json:"cassandra,omitempty"`

	// Stargate defines the desired deployment characteristics for Stargate in this K8ssandraCluster.
	// If this is non-nil, Stargate will be deployed on every Cassandra datacenter in this K8ssandraCluster.
	// +optional
	Stargate *StargateClusterTemplate `json:"stargate,omitempty"`
}

// K8ssandraClusterStatus defines the observed state of K8ssandraCluster
type K8ssandraClusterStatus struct {
	// Datacenters maps the CassandraDatacenter name to a K8ssandraStatus. The
	// naming is a bit confusing but the mapping makes sense because we have a
	// CassandraDatacenter and then define other components like Stargate and Reaper
	// relative to it. I wanted to inline the field but when I do it won't serialize.
	//
	// TODO Figure out how to inline this field
	Datacenters map[string]K8ssandraStatus `json:"datacenters,omitempty"`
}

// K8ssandraStatus defines the observed of a k8ssandra instance
type K8ssandraStatus struct {
	Cassandra *cassdcapi.CassandraDatacenterStatus `json:"cassandra,omitempty"`
	Stargate  *StargateStatus                      `json:"stargate,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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

	// SuperuserSecretName allows to override the default super user secret
	SuperuserSecretName string `json:"superUserSecret,omitempty"`

	// ServerImage is the image for the cassandra container. Note that this should be a
	// management-api image. If left empty the operator will choose a default image based
	// on ServerVersion.
	ServerImage string `json:"serverImage,omitempty"`

	// ServerVersion is the Cassandra version.
	// +kubebuilder:validation:Pattern=(3\.11\.\d+)|(4\.0\.\d+)
	ServerVersion string `json:"serverVersion,omitempty"`

	// Resources is the cpu and memory resources for the cassandra container.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// SystemLoggerResources is the cpu and memory resources for the server-system-logger
	// container.
	SystemLoggerResources *corev1.ResourceRequirements `json:"systemLoggerResources,omitempty"`

	// CassandraConfig is configuration settings that are applied to cassandra.yaml and
	// jvm-options for 3.11.x or jvm-server-options for 4.x.
	CassandraConfig *CassandraConfig `json:"config,omitempty"`

	// StorageConfig is the persistent storage requirements for each Cassandra pod. This
	// includes everything under /var/lib/cassandra, namely the commit log and data
	// directories.
	StorageConfig *cassdcapi.StorageConfig `json:"storageConfig,omitempty"`

	Racks []cassdcapi.Rack `json:"racks,omitempty"`

	Datacenters []CassandraDatacenterTemplate `json:"datacenters,omitempty"`
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
	ServerVersion string `json:"serverVersion,omitempty"`

	// CassandraConfig is configuration settings that are applied to cassandra.yaml and
	// jvm-options for 3.11.x or jvm-server-options for 4.x.
	CassandraConfig *CassandraConfig `json:"config,omitempty"`

	// Resources is the cpu and memory resources for the cassandra container.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// SystemLoggerResources is the cpu and memory resources for the server-system-logger
	// container.
	SystemLoggerResources *corev1.ResourceRequirements `json:"systemLoggerResources,omitempty"`

	Racks []cassdcapi.Rack `json:"racks,omitempty"`

	// StorageConfig is the persistent storage requirements for each Cassandra pod. This
	// includes everything under /var/lib/cassandra, namely the commit log and data
	// directories.
	StorageConfig *cassdcapi.StorageConfig `json:"storageConfig,omitempty"`

	// Stargate defines the desired deployment characteristics for Stargate in this datacenter. Leave nil to skip
	// deploying Stargate in this datacenter.
	// +optional
	Stargate *StargateDatacenterTemplate `json:"stargate,omitempty"`
}

type EmbeddedObjectMeta struct {
	Namespace string `json:"namespace,omitempty"`

	Name string `json:"name,omitempty"`

	Labels map[string]string `json:"labels,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty"`
}

// TODO Implement Stringer interface. It will helpful for debugging and testing.
type CassandraConfig struct {
	//Auth *Auth `json:"auth,omitempty"`

	CassandraYaml *CassandraYaml `json:"cassandraYaml,omitempty"`

	JvmOptions *JvmOptions `json:"jvmOptions,omitempty"`
}

type Auth struct {
	Enabled bool `json:"enabled,omitempty"`

	CacheValidityPeriodMillis *int64 `json:"cacheValidityPeriodMillis,omitempty"`

	CacheUpdateIntervalMillis *int64 `json:"cacheUpdateIntervalMillis,omitempty"`

	SuperUserSecretName string `json:"SuperUserSecretName,omitempty"`
}

type CassandraYaml struct {
	//Authenticator string `json:"authenticator,omitempty"`
	//
	//Authorizer string `json:"authorizer,omitempty"`
	//
	//RoleManager string `json:"role_manager,omitempty"`
	//
	//RoleValidityMillis *int64 `json:"roles_validity_in_ms,omitempty"`
	//
	//RoleUpdateIntervalMillis *int64 `json:"roles_update_interval_in_ms,omitempty"`
	//
	//PermissionValidityMillis *int64 `json:"permissions_validity_in_ms,omitempty"`

	ConcurrentReads *int `json:"concurrent_reads,omitempty"`

	ConcurrentWrites *int `json:"concurrent_writes,omitempty"`

	ConcurrentCounterWrites *int `json:"concurrent_counter_writes,omitempty"`

	AutoSnapshot *bool `json:"auto_snapshot,omitempty"`

	MemtableFlushWriters *int `json:"memtable_flush_writers,omitempty"`

	CommitLogSegmentSizeMb *int `json:"commitlog_segment_size_in_mb,omitempty"`

	ConcurrentCompactors *int `json:"concurrent_compactors,omitempty"`

	CompactionThroughputMbPerSec *int `json:"compaction_throughput_mb_per_sec,omitempty"`

	SstablePreemptiveOpenIntervalMb *int `json:"sstable_preemptive_open_interval_in_mb,omitempty"`

	KeyCacheSizeMb *int `json:"key_cache_size_in_mb,omitempty"`

	ThriftPreparedStatementCacheSizeMb *int `json:"thrift_prepared_statements_cache_size_mb,omitempty"`

	PreparedStatementsCacheSizeMb *int `json:"prepared_statements_cache_size_mb,omitempty"`

	StartRpc *bool `json:"start_rpc,omitempty"`

	SlowQueryLogTimeoutMs *int `json:"slow_query_log_timeout_in_ms,omitempty"`

	CounterCacheSizeMb *int `json:"counter_cache_size_in_mb,omitempty"`

	FileCacheSizeMb *int `json:"file_cache_size_in_mb,omitempty"`

	RowCacheSizeMb *int `json:"row_cache_size_in_mb,omitempty"`
}

type JvmOptions struct {
	HeapSize *resource.Quantity `json:"heapSize,omitempty"`

	HeapNewGenSize *resource.Quantity `json:"heapNewGenSize,omitempty"`

	AdditionalOptions []string `json:"additionalOptions,omitempty"`
}

func init() {
	SchemeBuilder.Register(&K8ssandraCluster{}, &K8ssandraClusterList{})
}
