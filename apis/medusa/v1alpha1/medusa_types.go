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
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Storage struct {
	// The storage backend to use for the backups.
	// +kubebuilder:validation:Enum=local;google_storage;azure_blobs;s3;s3_compatible;s3_rgw;ibm_storage
	// +kubebuilder:validation:Required
	StorageProvider string `json:"storageProvider,omitempty"`

	// Kubernetes Secret that stores the key file for the storage provider's API.
	// If using 'local' storage, this value is ignored.
	// +optional
	StorageSecretRef corev1.LocalObjectReference `json:"storageSecretRef,omitempty"`

	// The name of the bucket to use for the backups.
	// +kubebuilder:validation:Required
	BucketName string `json:"bucketName,omitempty"`

	// Name of the top level folder in the backup bucket.
	// If empty, the cluster name will be used.
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// Maximum backup age that the purge process should observe.
	// +kubebuilder:default=0
	// +optional
	MaxBackupAge int `json:"maxBackupAge,omitempty"`

	// Maximum number of backups to keep (used by the purge process).
	// Default is unlimited.
	// +kubebuilder:default=0
	// +optional
	MaxBackupCount int `json:"maxBackupCount,omitempty"`

	// AWS Profile to use for authentication.
	// +optional
	ApiProfile string `json:"apiProfile,omitempty"`

	// Max upload bandwidth in MB/s.
	// Defaults to 50 MB/s.
	// +kubebuilder:default="50MB/s"
	// +optional
	TransferMaxBandwidth string `json:"transferMaxBandwidth,omitempty"`

	// Number of concurrent uploads.
	// Helps maximizing the speed of uploads but puts more pressure on the network.
	// Defaults to 1.
	// +kubebuilder:default=1
	// +optional
	ConcurrentTransfers int `json:"concurrentTransfers,omitempty"`

	// File size over which cloud specific cli tools are used for transfer.
	// Defaults to 100 MB.
	// +kubebuilder:default=104857600
	// +optional
	MultiPartUploadThreshold int `json:"multiPartUploadThreshold,omitempty"`

	// Host to connect to for the storage backend.
	// +optional
	Host string `json:"host,omitempty"`

	// Region of the storage bucket.
	// Defaults to "default".
	// +optional
	Region string `json:"region,omitempty"`

	// Port to connect to for the storage backend.
	// +optional
	Port int `json:"port,omitempty"`

	// Whether to use SSL for the storage backend.
	// +optional
	Secure bool `json:"secure,omitempty"`

	// Age after which orphan sstables can be deleted from the storage backend.
	// Protects from race conditions between purge and ongoing backups.
	// Defaults to 10 days.
	// +optional
	BackupGracePeriodInDays int `json:"backupGracePeriodInDays,omitempty"`

	// Pod storage settings for the local storage provider
	// +optional
	PodStorage *PodStorageSettings `json:"podStorage,omitempty"`
}

type PodStorageSettings struct {
	// Settings for the pod's storage when backups use the local storage provider.

	// Storage class name to use for the pod's storage.
	StorageClassName string `json:"storageClassName,omitempty"`

	// Size of the pod's storage in bytes.
	// Defaults to 10 GB.
	// +kubebuilder:default="10Gi"
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// Pod local storage access modes
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

type MedusaClusterTemplate struct {
	// MedusaContainerImage is the image characteristics to use for Medusa containers. Leave nil
	// to use a default image.
	// +optional
	ContainerImage *images.Image `json:"containerImage,omitempty"`

	// SecurityContext applied to the Medusa containers.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// Defines the username and password that Medusa will use to authenticate CQL connections to Cassandra clusters.
	// These credentials will be automatically turned into CQL roles by cass-operator when bootstrapping the datacenter,
	// then passed to the Medusa instances, so that it can authenticate against nodes in the datacenter using CQL.
	// The secret must be in the same namespace as Cassandra and must contain two keys: "username" and "password".
	// +optional
	CassandraUserSecretRef corev1.LocalObjectReference `json:"cassandraUserSecretRef,omitempty"`

	// Provides all storage backend related properties for backups.
	StorageProperties Storage `json:"storageProperties,omitempty"`

	// Certificates for Medusa if client encryption is enabled in Cassandra.
	// The secret must be in the same namespace as Cassandra and must contain three keys: "rootca.crt", "client.crt_signed" and "client.key".
	// See https://docs.datastax.com/en/developer/python-driver/latest/security/ for more information on the required files.
	// +optional
	CertificatesSecretRef corev1.LocalObjectReference `json:"certificatesSecretRef,omitempty"`

	// Define the readiness probe settings to use for the Medusa containers.
	// +optional
	ReadinessProbeSettings *ProbeSettings `json:"readinessProbeSettings,omitempty"`

	// Define the liveness probe settings to use for the Medusa containers.
	// +optional
	LivenessProbeSettings *ProbeSettings `json:"livenessProbeSettings,omitempty"`

	// Define resource limits for the Medusa containers.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ProbeSettings struct {
	// The initial delay for the probe after the container has started.
	// +kubebuilder:default=10
	// +optional
	InitialDelaySeconds int `json:"initialDelaySeconds,omitempty"`

	// The period after which the probe will be invoked.
	// +kubebuilder:default=10
	// +optional
	PeriodSeconds int `json:"periodSeconds,omitempty"`

	// The number of failed probes after which the container will be considered unhealthy.
	// +kubebuilder:default=10
	// +optional
	FailureThreshold int `json:"failureThreshold,omitempty"`

	// The number of successful probes after which the container will be considered healthy.
	// +kubebuilder:default=1
	// +optional
	SuccessThreshold int `json:"successThreshold,omitempty"`

	// The number of seconds after which the probe will be considered failed.
	// +kubebuilder:default=1
	// +optional
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}
