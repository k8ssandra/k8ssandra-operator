package encryption

import (
	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:object:generate=true
type Stores struct {
	// ref to the secret that contains the keystore and its password
	// the expected format of the secret is a "keystore" entry and a "keystore-password" entry
	KeystoreSecretRef corev1.LocalObjectReference `json:"keystoreSecretRef"`

	// ref to the secret that contains the truststore and its password
	// the expected format of the secret is a "truststore" entry and a "truststore-password" entry
	// +optional
	TruststoreSecretRef *corev1.LocalObjectReference `json:"truststoreSecretRef"`
}

type EncryptionStoresYaml struct {
	Keystore           string `json:"keystore"`
	KeystorePassword   string `json:"keystore_password"`
	Truststore         string `json:"truststore"`
	TruststorePassword string `json:"truststore_password"`
}

type EncryptionStoresPasswords struct {
	ClientKeystorePassword   string
	ClientTruststorePassword string
	ServerKeystorePassword   string
	ServerTruststorePassword string
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type ClientEncryptionOptions struct {
	Enabled bool `json:"enabled"`

	// +optional
	Optional bool `json:"optional,omitempty"`

	EncryptionSettings `json:",inline"`
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type ServerEncryptionOptions struct {
	// +kubebuilder:default=false
	// +optional
	Optional *bool `json:"optional,omitempty"`

	// +kubebuilder:validation:Enum=none;dc;rack;all
	// +kubebuilder:default=none
	// +optional
	InternodeEncryption string `json:"internode_encryption,omitempty"`

	// +kubebuilder:default=false
	// +optional
	RequireEndpointVerification bool `json:"require_endpoint_verification,omitempty"`

	// +kubebuilder:default=false
	// +optional
	EnableLegacySslStoragePort bool `json:"enable_legacy_ssl_storage_port,omitempty"`

	EncryptionSettings `json:",inline"`
}

// See the cassandra.yaml file for explanations on how to set these options: https://github.com/apache/cassandra/blob/cassandra-4.0/conf/cassandra.yaml#L1091-L1183
// +kubebuilder:object:generate=true
type EncryptionSettings struct {
	// +optional
	Protocol string `json:"protocol,omitempty"`

	// +optional
	AcceptedProtocols []string `json:"accepted_protocols,omitempty"`

	// +optional
	Algorithm string `json:"algorithm,omitempty"`

	// +optional
	StoreType string `json:"store_type,omitempty"`

	// +optional
	CipherSuites []string `json:"cipher_suites,omitempty"`

	// +kubebuilder:default=false
	// +optional
	RequireClientAuth bool `json:"require_client_auth,omitempty"`
}
