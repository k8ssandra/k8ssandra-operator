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
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
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

	// During a migration the operator should alter keyspaces replication settings including the following external DCs.
	// This avoids removing replicas from datacenters which are outside of the operator scope (not referenced in the CR).
	// Replication settings changes will only apply to system_* keyspaces as well as reaper_db and data_endpoint_auth (Stargate).
	// +optional
	ExternalDatacenters []string `json:"externalDatacenters,omitempty"`

	// SecretsProvider defines whether the secrets used for credentials and certs will be backed
	// by an external secret backend (e.g. vault). This moves the responsibility of generating and
	// storing secrets from the operators to the user and will rely on a mutating webhook to inject
	// the secrets into the necessary resources
	// +kubebuilder:validation:Enum=internal;external
	// +kubebuilder:default=internal
	SecretsProvider string `json:"secretsProvider,omitempty"`
}

// IsAuthEnabled returns true if auth is not specified by the user (auth by default)
// or if the user has explicilty set Auth to true in the cluster spec
func (in K8ssandraClusterSpec) IsAuthEnabled() bool {
	return in.Auth == nil || *in.Auth
}

// UseExternalSecrets defines whether the user has specified if credentials and
// certs will be backed by an external secrets store
func (in K8ssandraClusterSpec) UseExternalSecrets() bool {
	return in.SecretsProvider != "" && in.SecretsProvider == "external"
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

	// +kubebuilder:default=None
	Error string `json:"error,omitempty"`
}

type K8ssandraClusterConditionType string

type DecommissionProgress string

const (
	// CassandraInitialized is set to true when the Cassandra cluster becomes ready for
	// the first time. During the life time of the C* cluster CassandraDatacenters may have
	// their readiness condition change back and forth. Once set, this condition however
	// does not change.
	CassandraInitialized = "CassandraInitialized"

	DecommNone                DecommissionProgress = ""
	DecommUpdatingReplication DecommissionProgress = "UpdatingReplication"
	DecommDeleting            DecommissionProgress = "Decommissioning"
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
	DecommissionProgress DecommissionProgress                 `json:"decommissionProgress,omitempty"`
	Cassandra            *cassdcapi.CassandraDatacenterStatus `json:"cassandra,omitempty"`
	Stargate             *stargateapi.StargateStatus          `json:"stargate,omitempty"`
	Reaper               *reaperapi.ReaperStatus              `json:"reaper,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=k8ssandraclusters,shortName=k8c;k8cs
// +kubebuilder:printcolumn:name="Error",type=string,JSONPath=".status.error",description="Latest reconcile error"

// K8ssandraCluster is the Schema for the k8ssandraclusters API. The K8ssandraCluster CRD name is also the name of the
// Cassandra cluster (which corresponds to cluster_name in cassandra.yaml).
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

// HasStoppedDatacenters returns true if at least one DC is flagged as stopped.
func (in *K8ssandraCluster) HasStoppedDatacenters() bool {
	if in == nil {
		return false
	} else if in.Spec.Cassandra == nil || len(in.Spec.Cassandra.Datacenters) == 0 {
		return false
	}
	for _, dcTemplate := range in.Spec.Cassandra.Datacenters {
		if dcTemplate.Stopped {
			return true
		}
	}
	return false
}

func (in *K8ssandraCluster) GetInitializedDatacenters() []CassandraDatacenterTemplate {
	datacenters := make([]CassandraDatacenterTemplate, 0)
	if in != nil && in.Spec.Cassandra != nil {
		for _, dc := range in.Spec.Cassandra.Datacenters {
			if status, found := in.Status.Datacenters[dc.Meta.Name]; found && status.Cassandra.GetConditionStatus(cassdcapi.DatacenterInitialized) == corev1.ConditionTrue {
				datacenters = append(datacenters, dc)
			}
		}
	}
	return datacenters
}

// SanitizedName returns a sanitized version of the name returned by CassClusterName()
func (in *K8ssandraCluster) SanitizedName() string {
	return cassdcapi.CleanupForKubernetes(in.CassClusterName())
}

// CassClusterName returns the Cassandra cluster name override if it exists,
// otherwise the k8c object name.
func (in *K8ssandraCluster) CassClusterName() string {
	if in.Spec.Cassandra != nil && in.Spec.Cassandra.ClusterName != "" {
		return in.Spec.Cassandra.ClusterName
	}
	return in.Name
}

// +kubebuilder:object:root=true

// K8ssandraClusterList contains a list of K8ssandraCluster
type K8ssandraClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []K8ssandraCluster `json:"items"`
}

type CassandraClusterTemplate struct {
	DatacenterOptions `json:",inline"`

	// The reference to the superuser secret to use for Cassandra. If unspecified, a default secret will be generated
	// with a random password; the generated secret name will be "<cluster_name>-superuser" where <cluster_name> is the
	// K8ssandraCluster CRD name.
	// +optional
	SuperuserSecretRef corev1.LocalObjectReference `json:"superuserSecretRef,omitempty"`

	// Datacenters a list of the DCs in the cluster.
	// +optional
	Datacenters []CassandraDatacenterTemplate `json:"datacenters,omitempty"`

	// Add annotations to generated pod spec.
	// +optional
	AdditionalPodAnnotations map[string]string `json:"additionalPodAnnotations,omitempty"`

	// AdditionalSeeds specifies Cassandra node IPs for an existing datacenter. This is
	// primarily intended for migrations from an existing Cassandra cluster that is not
	// managed by k8ssandra-operator. Note that this property should NOT be used to set
	// seeds for a DC that is or will be managed by k8ssandra-operator. k8ssandra-operator
	// already manages seeds for DCs that it manages. If you have DNS set up such that you
	// can resolve hostnames for the remote Cassandra cluster, then you can specify hostnames
	// here; otherwise, use IP addresses.
	AdditionalSeeds []string `json:"additionalSeeds,omitempty"`

	// Internode encryption stores which are used by Cassandra and Stargate.
	// +optional
	ServerEncryptionStores *encryption.Stores `json:"serverEncryptionStores,omitempty"`

	// Client encryption stores which are used by Cassandra and Reaper.
	// +optional
	ClientEncryptionStores *encryption.Stores `json:"clientEncryptionStores,omitempty"`

	// Override the Cassandra cluster name. If unspecified, the cluster name will be the same as the K8ssandraCluster
	// CRD name.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`

	// Server type: "cassandra" or "dse".
	// +kubebuilder:validation:Enum=cassandra;dse
	// +kubebuilder:default=cassandra
	ServerType ServerDistribution `json:"serverType,omitempty"`
}

type CassandraDatacenterTemplate struct {
	Meta EmbeddedObjectMeta `json:"metadata,omitempty"`

	K8sContext string `json:"k8sContext,omitempty"`

	DatacenterOptions `json:",inline"`

	// Size is the number Cassandra pods to deploy in this datacenter.
	// This number does not include Stargate instances.
	// +kubebuilder:validation:Minimum=1
	Size int32 `json:"size"`

	// Stopped means that the datacenter will be stopped. Use this for maintenance or for cost saving. A stopped
	// CassandraDatacenter will have no running server pods, like using "stop" with  traditional System V init scripts.
	// Other Kubernetes resources will be left intact, and volumes will re-attach when the CassandraDatacenter
	// workload is resumed.
	// +optional
	// +kubebuilder:default=false
	Stopped bool `json:"stopped,omitempty"`

	// Stargate defines the desired deployment characteristics for Stargate in this datacenter. Leave nil to skip
	// deploying Stargate in this datacenter.
	// +optional
	Stargate *stargateapi.StargateDatacenterTemplate `json:"stargate,omitempty"`

	// PerNodeConfigMapRef is a reference to a ConfigMap that contains per-node configuration for
	// this DC. The ConfigMap is expected to have entries in the following form:
	// <pod-name>_<file-name>.yaml, where <pod-name> is the name of the pod and <file-name> is the
	// name of a configuration file (typically, cassandra.yaml). The value of the entry is expected
	// to be a YAML fragment that contains the per-node configuration for each pod. When the pod is
	// started, the per-node ConfigMap is mounted and the contents of each entry corresponding to
	// the pod are merged into their respective configuration files.
	// +optional
	PerNodeConfigMapRef corev1.LocalObjectReference `json:"perNodeConfigMapRef,omitempty"`
}

// DatacenterOptions are configuration settings that are can be set at the Cluster level and overridden for a single DC
type DatacenterOptions struct {
	// ServerVersion is the Cassandra or DSE version. The following versions are supported:
	// - Cassandra: 3.11.X and 4.0.X
	// - DSE: 6.8.X
	// +kubebuilder:validation:Pattern=(6\.8\.\d+)|(3\.11\.\d+)|(4\.0\.\d+)
	ServerVersion string `json:"serverVersion,omitempty"`

	// ServerImage is the image for the cassandra container. Note that this should be a
	// management-api image. If left empty the operator will choose a default image based
	// on ServerVersion.
	// +optional
	ServerImage string `json:"serverImage,omitempty"`

	// CassandraConfig contains configuration settings that are applied to cassandra.yaml, dse.yaml
	// and the various jvm*.options files.
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

	// Resources is the cpu and memory resources for the cassandra container.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Racks is a list of named racks. Note that racks are used to create node affinity. //
	// +optional
	Racks []cassdcapi.Rack `json:"racks,omitempty"`

	// The image to use in each Cassandra pod for the (short-lived) init container that enables JMX remote
	// authentication on Cassandra pods. This is only useful when authentication is enabled in the cluster.
	// The default is "busybox:1.34.1".
	// +optional
	// +kubebuilder:default={name:"busybox",tag:"1.34.1"}
	JmxInitContainerImage *images.Image `json:"jmxInitContainerImage,omitempty"`

	// SoftPodAntiAffinity sets whether multiple Cassandra instances can be scheduled on the same node.
	// This should normally be false to ensure cluster resilience but may be set true for test/dev scenarios to minimise
	// the number of nodes required.
	SoftPodAntiAffinity *bool `json:"softPodAntiAffinity,omitempty"`

	// Tolerations applied to every Cassandra pod.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// MgmtAPIHeap defines the amount of memory devoted to the management
	// api heap.
	// +optional
	MgmtAPIHeap *resource.Quantity `json:"mgmtAPIHeap,omitempty"`

	// Telemetry defines the desired state for telemetry resources in this datacenter.
	// If telemetry configurations are defined, telemetry resources will be deployed to integrate with
	// a user-provided monitoring solution (at present, only support for Prometheus is available).
	// +optional
	Telemetry *telemetryapi.TelemetrySpec `json:"telemetry,omitempty"`

	// CDC defines the desired state for CDC integrations. It can be used to feed mutation events from Cassandra into an Apache Pulsar cluster,
	// from where they can be expored to external systems.
	// +optional
	CDC *cassdcapi.CDCConfiguration `json:"cdc,omitempty"`

	// Containers defines containers to be deployed in each Cassandra pod.
	// K8ssandra-operator and cass-operator will create their own containers, which can be referenced here to override specific settings,
	// such as mounts or resources request/limits for example.
	// Example:
	//  containers:
	//  - name: server-system-logger
	//  - name: custom-container
	//    image: busybox
	//  - name: cassandra
	// +optional
	Containers []corev1.Container `json:"containers,omitempty"`

	// InitContainers defines init-containers to be deployed in each Cassandra pod.
	// K8ssandra-operator and cass-operator will create their own init-containers, which can be referenced here to control ordering or override specific settings
	// such as mounts or resources request/limits for example.
	// Only the name of the k8ssandra-operator/cass-operator init-containers is required, not their full definition which will be created by the operators.
	// Example:
	//  initContainers:
	//  - name: server-config-init
	//  - name: custom-init-container
	//    image: busybox
	//  - name: medusa-restore
	// Otherwise init-containers referenced here will be injected in first position
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Add annotations to generated pod spec.
	// +optional
	AdditionalPodAnnotations map[string]string `json:"additionalPodAnnotations,omitempty"`

	// Volumes defines additional volumes to be added to each Cassandra pod.
	// If the volume uses a PersistentVolumeClaim, the PVC will be managed by the statefulset.
	// +optional
	ExtraVolumes *K8ssandraVolumes `json:"extraVolumes,omitempty"`

	// +optional
	DseWorkloads *cassdcapi.DseWorkloads `json:"dseWorkloads,omitempty"`

	// PodSecurityContext defines the security context for the Cassandra pods.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// ManagementApiAuth defines the authentication settings for the management API in the Cassandra pods.
	// +optional
	ManagementApiAuth *cassdcapi.ManagementApiAuthConfig `json:"managementApiAuth,omitempty"`
}

type K8ssandraVolumes struct {
	// Volumes defines volumes to be added to each Cassandra pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// PVCs defines additional volumes to be added to each Cassandra pod and managed by the statefulset.
	// Such volumes are automatically mounted by cass-operator into the cassandra containers.
	// +optional
	PVCs []cassdcapi.AdditionalVolumes `json:"pvcs,omitempty"`
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

func (in *EmbeddedObjectMeta) GetAnnotations() map[string]string {
	return in.Annotations
}

func (in *EmbeddedObjectMeta) SetAnnotations(annotations map[string]string) {
	in.Annotations = annotations
}

func (in *EmbeddedObjectMeta) GetLabels() map[string]string {
	return in.Labels
}

func (in *EmbeddedObjectMeta) SetLabels(labels map[string]string) {
	in.Labels = labels
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

type ServerDistribution string

const (
	ServerDistributionCassandra = ServerDistribution("cassandra")
	ServerDistributionDse       = ServerDistribution("dse")
)
