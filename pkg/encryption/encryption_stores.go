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
