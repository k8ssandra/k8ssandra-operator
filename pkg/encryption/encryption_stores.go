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
	EncryptionSettings `json:",inline" cass-config:"*:;recurse"`
	Enabled            bool `json:"enabled" cass-config:"*:enabled;retainzero"`
	Optional           bool `json:"optional,omitempty" cass-config:"*:optional"`
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type ServerEncryptionOptions struct {
	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	Optional *bool `json:"optional,omitempty" cass-config:">=4.x:optional"`

	// +kubebuilder:validation:Enum=none;dc;rack;all
	// +kubebuilder:default=none
	// +kubebuilder:validation:optional
	InternodeEncryption string `json:"internode_encryption,omitempty" cass-config:"*:internode_encryption"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	RequireEndpointVerification bool `json:"require_endpoint_verification,omitempty" cass-config:"*:require_endpoint_verification"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	EnableLegacySslStoragePort bool `json:"enable_legacy_ssl_storage_port,omitempty" cass-config:"*:enable_legacy_ssl_storage_port"`

	EncryptionSettings `json:",inline" cass-config:"*:;recurse"`
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type EncryptionSettings struct {
	// +kubebuilder:validation:optional
	Protocol string `json:"protocol,omitempty" cass-config:"*:protocol"`

	// +kubebuilder:validation:optional
	AcceptedProtocols []string `json:"accepted_protocols,omitempty" cass-config:"*:accepted_protocols"`

	// +kubebuilder:validation:optional
	Algorithm string `json:"algorithm,omitempty" cass-config:"*:algorithm"`

	// +kubebuilder:validation:optional
	StoreType string `json:"store_type,omitempty" cass-config:"*:store_type"`

	// +kubebuilder:validation:optional
	CipherSuites []string `json:"cipher_suites,omitempty" cass-config:"*:cipher_suites"`

	// +kubebuilder:default=false
	// +kubebuilder:validation:optional
	RequireClientAuth bool `json:"require_client_auth,omitempty" cass-config:"*:require_client_auth"`
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
