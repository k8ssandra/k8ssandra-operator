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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Storage struct {
	// The storage backend to use for the backups.
	// Should be either of "local", "google_storage", "azure_blobs" or "s3".
	StorageProvider string `json:"storageProvider,omitempty"`

	// Kubernetes Secret that stores the key file for the storage provider's API.
	// If using 'local' storage, this value is ignored.
	// +optional
	StorageSecretRef string `json:"storageSecretRef,omitempty"`

	// The name of the bucket to use for the backups.
	BucketName string `json:"bucketName,omitempty"`

	// Name of the top level folder in the backup bucket.
	// If empty, the cluster name will be used.
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// Maximum backup age that the purge process should observe.
	// +optional
	MaxBackupAge int `json:"maxBackupAge,omitempty"`

	// Maximum number of backups to keep (used by the purge process).
	// Default is unlimited.
	// +optional
	MaxBackupCount int `json:"maxBackupCount,omitempty"`

	// AWS Profile to use for authentication.
	// +optional
	ApiProfile string `json:"apiProfile,omitempty"`

	// Max upload bandwidth in MB/s.
	// Defaults to 50 MB/s.
	// +optional
	TransferMaxBandwidth int `json:"transferMaxBandwidth,omitempty"`

	// Number of concurrent uploads.
	// Helps maximizing the speed of uploads but puts more pressure on the network.
	// Defaults to 1.
	// +optional
	ConcurrentTransfers int `json:"concurrentTransfers,omitempty"`

	// File size over which cloud specific cli tools are used for transfer.
	// Defaults to 100 MB.
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
	Port int `json:"port,omitempty"`

	// Whether to use SSL for the storage backend.
	Secure bool `json:"secure,omitempty"`

	// Age after which orphan sstables can be deleted from the storage backend.
	// Protects from race conditions between purge and ongoing backups.
	// Defaults to 10 days.
	// +optional
	BackupGracePeriodInDays int `json:"backupGracePeriodInDays,omitempty"`
}

type ContainerImage struct {

	// +kubebuilder:default="docker.io"
	// +optional
	Registry *string `json:"registry,omitempty"`

	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// +kubebuilder:default="latest"
	// +optional
	Tag *string `json:"tag,omitempty"`

	// +kubebuilder:default="IfNotPresent"
	// +optional
	PullPolicy *corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// +optional
	PullSecret corev1.LocalObjectReference `json:"imagePullSecret,omitempty"`
}

type MedusaClusterTemplate struct {
	// MedusaContainerImage is the image characteristics to use for Medusa containers. Leave nil
	// to use a default image.
	// +optional
	Image ContainerImage `json:"medusaContainerImage,omitempty"`

	// SecurityContext applied to the Medusa containers.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// Defines the username and password that Medusa will use to authenticate CQL connections to Cassandra clusters.
	// These credentials will be automatically turned into CQL roles by cass-operator when bootstrapping the datacenter,
	// then passed to the Medusa instances, so that it can authenticate against nodes in the datacenter using CQL.
	// The secret must be in the same namespace as Reaper itself and must contain two keys: "username" and "password".
	// +optional
	CassandraUserSecretRef string `json:"cassandraUserSecretRef,omitempty"`

	// Provides all storage backend related properties for backups.
	StorageProperties Storage `json:"storageProperties,omitempty"`
}

// MedusaSpec defines the desired state of Medusa.
type MedusaSpec struct {
	MedusaClusterTemplate `json:",inline"`
}

type Medusa struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MedusaSpec `json:"spec,omitempty"`
}
