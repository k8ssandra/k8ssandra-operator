package encryption

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestPasswordSecretKeySelector(t *testing.T) {
	stores := &Stores{
		KeystoreSecretRef: &SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "stores"},
		},
		TruststoreSecretRef: &SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "stores"},
			Key:                  "custom-truststore",
		},
		KeystorePasswordRef: &SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "passwords"},
			Key:                  "custom-keystore-password",
		},
	}

	require.Equal(t, &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "passwords"},
		Key:                  "custom-keystore-password",
	}, stores.PasswordSecretKeySelector(StoreNameKeystore))
	require.Equal(t, &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "stores"},
		Key:                  "truststore-password",
	}, stores.PasswordSecretKeySelector(StoreNameTruststore))
	require.Nil(t, stores.PasswordSecretKeySelector(StoreName("unsupported")))
}
