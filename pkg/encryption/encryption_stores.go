package encryption

import (
	corev1 "k8s.io/api/core/v1"
)

// +kubebuilder:object:generate=true
type Stores struct {
	// ref to the secret that contains the keystore and optionally its password (which can also be specified through
	//the keystorePasswordSecretRef field)
	// if keys are not specified, "keystore" entry and a "keystore-password" entry will be used
	// +kubebuilder:validation:Required
	KeystoreSecretRef *SecretKeySelector `json:"keystoreSecretRef"`

	// ref to the secret that contains the truststore and optionally its password (which can also be specified through
	//the truststorePasswordSecretRef field)
	// if keys are not specified explicitly, "keystore" entry and a "keystore-password" entry will be used
	// +kubebuilder:validation:Required
	TruststoreSecretRef *SecretKeySelector `json:"truststoreSecretRef"`

	// ref to the secret that contains the keystore password if password stored in different secret than keystoreSecretRef
	// if key isn't specified explicitly, "keystore-password" entry will be used
	// +kubebuilder:validation:Optional
	KeystorePasswordRef *SecretKeySelector `json:"keystorePasswordSecretRef"`

	// ref to the secret that contains the truststore password if password stored in different secret than keystoreSecretRef
	// if key isn't specified  explicitly, "truststore-password" entry will be used
	// +kubebuilder:validation:Optional
	TruststorePasswordSecretRef *SecretKeySelector `json:"truststorePasswordSecretRef"`
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

// A reference to a specific 'key' within a Secret resource.
type SecretKeySelector struct {
	// The name of the Secret resource being referred to.
	corev1.LocalObjectReference `json:",inline"`

	// The key of the entry in the Secret resource's `data` field to be used.
	// +kubebuilder:validation:Optional
	Key string `json:"key,omitempty"`
}

func (s *SecretKeySelector) GetSpecificKeyOrDefault(defaultVal string) (key string) {
	if s.Key != "" {
		return s.Key
	}
	return defaultVal
}
