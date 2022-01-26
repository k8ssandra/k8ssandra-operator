package encryption

import (
	corev1 "k8s.io/api/core/v1"
)

type Stores struct {
	KeystoreSecretRef corev1.LocalObjectReference `json:"keystore_secret_ref"`

	KeystorePasswordSecretRef corev1.LocalObjectReference `json:"keystore_password_secret_ref"`

	TruststoreSecretRef corev1.LocalObjectReference `json:"truststore_secret_ref"`

	TruststorePasswordSecretRef corev1.LocalObjectReference `json:"truststore_password_secret_ref"`
}
