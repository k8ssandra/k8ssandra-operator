package encryption

import (
	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:object:generate=true
type Stores struct {
	// ref to the secret that contains the keystore and its password
	// the expected format of the secret is a "keystore" entry and a "keystore-password" entry
	// +kubebuilder:validation:Required
	KeystoreSecretRef corev1.LocalObjectReference `json:"keystoreSecretRef"`

	// ref to the secret that contains the truststore and its password
	// the expected format of the secret is a "truststore" entry and a "truststore-password" entry
	// +kubebuilder:validation:Required
	TruststoreSecretRef corev1.LocalObjectReference `json:"truststoreSecretRef"`
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type ClientEncryptionOptions struct {
	EncryptionSettings `json:",inline"`
	Enabled            bool `json:"enabled"`
	Optional           bool `json:"optional,omitempty"`
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type ServerEncryptionOptions struct {
	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	Optional *bool `json:"optional,omitempty"`

	// +kubebuilder:validation:Enum=none;dc;rack;all
	// +kubebuilder:default=none
	// +kubebuilder:validation:optional
	InternodeEncryption string `json:"internode_encryption,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	RequireEndpointVerification bool `json:"require_endpoint_verification,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	EnableLegacySslStoragePort bool `json:"enable_legacy_ssl_storage_port,omitempty"`

	EncryptionSettings `json:",inline"`
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type EncryptionSettings struct {
	// +kubebuilder:validation:optional
	Protocol string `json:"protocol,omitempty"`

	// +kubebuilder:validation:optional
	AcceptedProtocols []string `json:"accepted_protocols,omitempty"`

	// +kubebuilder:validation:optional
	Algorithm string `json:"algorithm,omitempty"`

	// +kubebuilder:validation:optional
	StoreType string `json:"store_type,omitempty"`

	// +kubebuilder:validation:optional
	CipherSuites []string `json:"cipher_suites,omitempty"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	RequireClientAuth bool `json:"require_client_auth,omitempty"`
}

type StoreType string

const (
	StoreTypeClient = StoreType("client")
	StoreTypeServer = StoreType("server")
)

type StoreName string

const (
	StoreNameKeystore   = StoreName("keystore")
	StoreNameTruststore = StoreName("truststore")
)
