package encryption

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestPasswordSecretKeySelector(t *testing.T) {
	selector := func(name, key string) *SecretKeySelector {
		return &SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
		}
	}

	tests := []struct {
		name      string
		stores    *Stores
		storeName StoreName
		expected  *corev1.SecretKeySelector
	}{
		{
			name:      "nil stores",
			storeName: StoreNameKeystore,
		},
		{
			name:      "unsupported store",
			stores:    &Stores{},
			storeName: StoreName("unsupported"),
		},
		{
			name: "dedicated keystore password with custom key",
			stores: &Stores{
				KeystorePasswordRef: selector("passwords", "custom-keystore-password"),
			},
			storeName: StoreNameKeystore,
			expected: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "passwords"},
				Key:                  "custom-keystore-password",
			},
		},
		{
			name: "dedicated keystore password with default key",
			stores: &Stores{
				KeystorePasswordRef: selector("passwords", ""),
			},
			storeName: StoreNameKeystore,
			expected: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "passwords"},
				Key:                  "keystore-password",
			},
		},
		{
			name: "dedicated truststore password with custom key",
			stores: &Stores{
				TruststorePasswordSecretRef: selector("truststore-passwords", "custom-truststore-password"),
			},
			storeName: StoreNameTruststore,
			expected: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "truststore-passwords"},
				Key:                  "custom-truststore-password",
			},
		},
		{
			name: "dedicated truststore password with default key",
			stores: &Stores{
				TruststorePasswordSecretRef: selector("truststore-passwords", ""),
			},
			storeName: StoreNameTruststore,
			expected: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "truststore-passwords"},
				Key:                  "truststore-password",
			},
		},
		{
			name: "keystore secret fallback",
			stores: &Stores{
				KeystoreSecretRef: selector("stores", "custom-keystore"),
			},
			storeName: StoreNameKeystore,
			expected: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "stores"},
				Key:                  "keystore-password",
			},
		},
		{
			name: "truststore secret fallback",
			stores: &Stores{
				TruststoreSecretRef: selector("stores", "custom-truststore"),
			},
			storeName: StoreNameTruststore,
			expected: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "stores"},
				Key:                  "truststore-password",
			},
		},
		{
			name:      "missing store and password references",
			stores:    &Stores{},
			storeName: StoreNameKeystore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.stores.PasswordSecretKeySelector(tt.storeName))
		})
	}
}
