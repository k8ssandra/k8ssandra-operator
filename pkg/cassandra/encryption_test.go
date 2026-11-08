package cassandra

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCheckMandatoryEncryptionFields(t *testing.T) {
	assert := assert.New(t)

	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": true,
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "all",
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			}},
		},
		ServerEncryptionStores: &encryption.Stores{
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			}},
		},
	}

	assert.NoError(checkMandatoryEncryptionFields(dcConfig.ClientEncryptionStores))
	assert.Error(checkMandatoryEncryptionFields(dcConfig.ServerEncryptionStores))

	assert.True(ServerEncryptionEnabled(dcConfig))
	assert.True(ClientEncryptionEnabled(dcConfig))
}

func TestOverrideOfMandatoryEncryptionCheck(t *testing.T) {
	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": true,
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "all",
				},
			},
		},
	}

	assert.False(t, ServerEncryptionEnabled(dcConfig))
	assert.False(t, ClientEncryptionEnabled(dcConfig))
}

func TestAddEncryptionMountToCassandra(t *testing.T) {
	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": true,
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "all",
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			}},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			}},
		},
	}

	clientKeystoreVolume := corev1.Volume{
		Name: "client-keystore",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "client-keystore-secret",
				},
			},
		},
	}

	clientTruststoreVolume := corev1.Volume{
		Name: "client-truststore",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "client-truststore-secret",
				},
			},
		},
	}

	addEncryptionMountToCassandra(dcConfig, &clientKeystoreVolume, &clientTruststoreVolume, encryption.StoreTypeClient)
	assert.Equal(t, 1, len(dcConfig.PodTemplateSpec.Spec.Containers))
	assert.Equal(t, "cassandra", dcConfig.PodTemplateSpec.Spec.Containers[0].Name)
	assert.Equal(t, 2, len(dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, "client-keystore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[0].Name)
	assert.Equal(t, "client-truststore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[1].Name)

	serverKeystoreVolume := corev1.Volume{
		Name: "server-keystore",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "server-keystore-secret",
				},
			},
		},
	}

	serverTruststoreVolume := corev1.Volume{
		Name: "server-truststore",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "server-truststore-secret",
				},
			},
		},
	}

	addEncryptionMountToCassandra(dcConfig, &serverKeystoreVolume, &serverTruststoreVolume, encryption.StoreTypeServer)
	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, "server-keystore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[2].Name)
	assert.Equal(t, "server-truststore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[3].Name)
}

func TestAddVolumesForEncryption(t *testing.T) {
	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": true,
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "all",
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			}},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			}, Key: "specific-key-from-keystoresecret"},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			}, Key: "specific-key-from-truststoresecret"},
		},
	}

	addVolumesForEncryption(dcConfig, encryption.StoreTypeClient, *dcConfig.ClientEncryptionStores)
	assert.Equal(t, 2, len(dcConfig.PodTemplateSpec.Spec.Volumes))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore", "client-keystore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore", "client-truststore-secret"))
	assert.True(t, keyToPathElemVolumeExist(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore", string(encryption.StoreNameKeystore)))
	assert.True(t, keyToPathElemVolumeExist(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore", string(encryption.StoreNameTruststore)))

	addVolumesForEncryption(dcConfig, encryption.StoreTypeServer, *dcConfig.ServerEncryptionStores)
	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Volumes))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore", "server-truststore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore", "server-keystore-secret"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore", "server-keystore-secret"))
	assert.True(t, keyToPathElemVolumeExist(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore", "specific-key-from-keystoresecret"))
	assert.True(t, keyToPathElemVolumeExist(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore", "specific-key-from-truststoresecret"))
}

func TestHandleEncryptionOptions(t *testing.T) {
	// Test a succeeding case with both client and server encryption turned on
	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled":             true,
					"keystore_password":   "plaintext-password",
					"truststore_password": "plaintext-password",
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "all",
					"keystore_password":    "plaintext-password",
					"truststore_password":  "plaintext-password",
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			}},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			}},
		},
	}

	err := HandleEncryptionOptions(dcConfig, "client-image")
	require.NoError(t, err)
	assert.Equal(t, 5, len(dcConfig.PodTemplateSpec.Spec.Volumes))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore", "client-keystore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore", "client-truststore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore", "server-truststore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore", "server-keystore-secret"))
	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, "client-keystore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[0].Name)
	assert.Equal(t, "client-truststore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[1].Name)
	assert.Equal(t, "server-keystore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[2].Name)
	assert.Equal(t, "server-truststore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[3].Name)
	for _, jvmOption := range []string{
		"-Dcom.sun.management.jmxremote.ssl=true",
		"-Dcom.sun.management.jmxremote.ssl.need.client.auth=true",
		fmt.Sprintf("-Djavax.net.ssl.keyStore=%s/keystore", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameKeystore)),
		fmt.Sprintf("-Djavax.net.ssl.trustStore=%s/truststore", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameTruststore)),
	} {
		assert.True(t, utils.SliceContains(dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, jvmOption), fmt.Sprintf("JVM option %s not found", jvmOption))
	}
	expectedCassandraYaml := unstructured.Unstructured{
		"client_encryption_options": map[string]interface{}{
			"enabled":    true,
			"keystore":   "/mnt/client-keystore/keystore",
			"truststore": "/mnt/client-truststore/truststore",
		},
		"server_encryption_options": map[string]interface{}{
			"internode_encryption": "all",
			"keystore":             "/mnt/server-keystore/keystore",
			"truststore":           "/mnt/server-truststore/truststore",
		},
	}
	assert.Equal(t, expectedCassandraYaml, dcConfig.CassandraConfig.CassandraYaml)
	require.Len(t, dcConfig.PodTemplateSpec.Spec.InitContainers, 2)
	assert.Equal(t, reconciliation.ServerConfigContainerName, dcConfig.PodTemplateSpec.Spec.InitContainers[0].Name)
	passwordInit := dcConfig.PodTemplateSpec.Spec.InitContainers[1]
	assert.Equal(t, encryptionPasswordInitContainer, passwordInit.Name)
	assert.Equal(t, "client-image", passwordInit.Image)
	assert.NotContains(t, passwordInit.Command[2], "test")
	assert.Contains(t, passwordInit.Command[2], "load_str")
	assert.Contains(t, passwordInit.VolumeMounts, corev1.VolumeMount{Name: "tmp", MountPath: "/tmp"})
	assert.Contains(t, dcConfig.PodTemplateSpec.Spec.Containers[0].Env, corev1.EnvVar{
		Name: clientKeystorePasswordEnvName,
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "client-keystore-secret"},
			Key:                  "keystore-password",
		}},
	})
	assert.Contains(t, dcConfig.PodTemplateSpec.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "JAVA_TOOL_OPTIONS",
		Value: "-Djavax.net.ssl.keyStorePassword=$(K8SSANDRA_CLIENT_KEYSTORE_PASSWORD) -Djavax.net.ssl.trustStorePassword=$(K8SSANDRA_CLIENT_TRUSTSTORE_PASSWORD)",
	})
	renderedConfig, err := json.Marshal(dcConfig)
	require.NoError(t, err)
	assert.NotContains(t, string(renderedConfig), "plaintext-password")
}

func TestHandleEncryptionOptionsWithExistingContainers(t *testing.T) {
	// Test a succeeding case with both client and server encryption turned on
	dcConfig := &DatacenterConfig{
		PodTemplateSpec: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "cassandra",
					},
					{
						Name: "bogus",
					},
				},
			},
		},
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": true,
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "all",
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			}},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			}},
			TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			}},
		},
	}

	err := HandleEncryptionOptions(dcConfig, "client-image")
	require.NoError(t, err)
	assert.Equal(t, 5, len(dcConfig.PodTemplateSpec.Spec.Volumes))

	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore", "client-keystore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore", "client-truststore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore", "server-truststore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore", "server-keystore-secret"))

	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, "client-keystore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[0].Name)
	assert.Equal(t, "client-truststore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[1].Name)
	assert.Equal(t, "server-keystore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[2].Name)
	assert.Equal(t, "server-truststore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[3].Name)
	assert.Equal(t, 2, len(dcConfig.PodTemplateSpec.Spec.Containers))
}

func TestHandleNoEncryptionOptions(t *testing.T) {
	// Test a succeeding case with disabled encryption
	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": false,
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "none",
				},
			},
		},
	}

	err := HandleEncryptionOptions(dcConfig, "")
	require.NoError(t, err)
	assert.Equal(t, 0, len(dcConfig.PodTemplateSpec.Spec.Volumes))
	assert.Equal(t, 0, len(dcConfig.PodTemplateSpec.Spec.Containers))
	expectedCassandraYaml := unstructured.Unstructured{
		"client_encryption_options": map[string]interface{}{
			"enabled": false,
		},
		"server_encryption_options": map[string]interface{}{
			"internode_encryption": "none",
		},
	}
	assert.Equal(t, expectedCassandraYaml, dcConfig.CassandraConfig.CassandraYaml)
}

func TestHandleFailedEncryptionOptions(t *testing.T) {
	// Test a succeeding case with missing encryption options
	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": true,
				},
				"server_encryption_options": map[string]interface{}{
					"internode_encryption": "all",
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{},
	}

	err := HandleEncryptionOptions(dcConfig, "client-image")
	assert.Error(t, err)
}

func volumeExists(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func volumeHasSecretSource(volumes []corev1.Volume, name, secretName string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			if volume.Secret != nil {
				if volume.Secret.SecretName == secretName {
					return true
				}
			}
		}
	}
	return false
}

func keyToPathElemVolumeExist(volumes []corev1.Volume, name, key string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			if volume.Secret != nil {
				for _, mapping := range volume.Secret.Items {
					if mapping.Key == key {
						return true
					}
				}
			}
		}
	}
	return false
}

func TestValidateEncryptionStorePassword(t *testing.T) {
	// Test case when KeystorePasswordRef,TruststorePasswordSecretRef is configured
	keystoreSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-keystore-password-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{"client-keystore-password-key": []byte("test-keystore-password")},
	}
	truststoreSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}, ObjectMeta: metav1.ObjectMeta{
			Name:      "client-truststore-secret",
			Namespace: "default",
		}, Data: map[string][]byte{"client-truststore-password-key": []byte("test-truststore-password")},
	}

	client := fake.NewClientBuilder().WithObjects(keystoreSecret, truststoreSecret).Build()
	ClientEncryptionStores := &encryption.Stores{
		KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "client-keystore-secret",
		}},
		KeystorePasswordRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "client-keystore-password-secret",
		}, Key: "client-keystore-password-key"},
		TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "client-truststore-secret",
		}, Key: "client-truststore-key"},
		TruststorePasswordSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "client-truststore-secret",
		}, Key: "client-truststore-password-key"},
	}
	assert.NoError(t, validateEncryptionStorePassword(context.Background(), "default", client, ClientEncryptionStores, encryption.StoreNameKeystore))
	assert.NoError(t, validateEncryptionStorePassword(context.Background(), "default", client, ClientEncryptionStores, encryption.StoreNameTruststore))
}

func TestCheckMandatoryEncryptionFieldsBranches(t *testing.T) {
	selector := &encryption.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "store"},
	}
	tests := []struct {
		name          string
		stores        *encryption.Stores
		expectedError string
	}{
		{
			name:          "nil stores",
			expectedError: "EncryptionStores is required to set up encryption",
		},
		{
			name:          "missing keystore reference",
			stores:        &encryption.Stores{},
			expectedError: "keystore secret ref is required when EncryptionStores is used",
		},
		{
			name: "missing truststore reference",
			stores: &encryption.Stores{
				KeystoreSecretRef: selector,
			},
			expectedError: "truststore secret ref is required when EncryptionStores is used",
		},
		{
			name: "valid stores",
			stores: &encryption.Stores{
				KeystoreSecretRef:   selector,
				TruststoreSecretRef: selector,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkMandatoryEncryptionFields(tt.stores)
			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}
		})
	}
}

func TestRemoveEncryptionPasswordFields(t *testing.T) {
	tests := []struct {
		name     string
		input    unstructured.Unstructured
		expected unstructured.Unstructured
	}{
		{
			name:     "missing section",
			input:    unstructured.Unstructured{},
			expected: unstructured.Unstructured{},
		},
		{
			name: "non-map section",
			input: unstructured.Unstructured{
				"client_encryption_options": "invalid",
			},
			expected: unstructured.Unstructured{
				"client_encryption_options": "invalid",
			},
		},
		{
			name: "removes only password fields",
			input: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled":             true,
					"keystore_password":   "keystore-password",
					"truststore_password": "truststore-password",
				},
			},
			expected: unstructured.Unstructured{
				"client_encryption_options": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{CassandraYaml: *tt.input.DeepCopy()},
			}

			removeEncryptionPasswordFields(template, "client_encryption_options")

			require.Equal(t, tt.expected, template.CassandraConfig.CassandraYaml)
		})
	}
}

func TestHandleEncryptionOptionsModes(t *testing.T) {
	tests := []struct {
		name                   string
		clientEncryption       bool
		serverEncryption       bool
		externalSecrets        bool
		expectedPasswordFiles  []string
		expectCassandraEnvVars bool
	}{
		{
			name:                   "client encryption",
			clientEncryption:       true,
			expectedPasswordFiles:  []string{clientKeystorePasswordFileName, clientTruststorePasswordFileName},
			expectCassandraEnvVars: true,
		},
		{
			name:                  "server encryption",
			serverEncryption:      true,
			expectedPasswordFiles: []string{serverKeystorePasswordFileName, serverTruststorePasswordFileName},
		},
		{
			name:                   "client and server encryption",
			clientEncryption:       true,
			serverEncryption:       true,
			expectedPasswordFiles:  []string{clientKeystorePasswordFileName, clientTruststorePasswordFileName, serverKeystorePasswordFileName, serverTruststorePasswordFileName},
			expectCassandraEnvVars: true,
		},
		{
			name:             "external secrets bypass",
			clientEncryption: true,
			serverEncryption: true,
			externalSecrets:  true,
		},
		{
			name: "no encryption",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := newTestEncryptionDatacenterConfig(tt.clientEncryption, tt.serverEncryption)
			template.ExternalSecrets = tt.externalSecrets

			err := HandleEncryptionOptions(template, "client-image")

			require.NoError(t, err)
			if tt.externalSecrets || (!tt.clientEncryption && !tt.serverEncryption) {
				require.Empty(t, template.PodTemplateSpec.Spec.Volumes)
				require.Empty(t, template.PodTemplateSpec.Spec.InitContainers)
				if tt.externalSecrets {
					require.Contains(t, template.CassandraConfig.JvmOptions.AdditionalOptions, "-Dcom.sun.management.jmxremote.ssl=true")
				}
				return
			}

			passwordVolume := requireVolume(t, template.PodTemplateSpec.Spec.Volumes, encryptionPasswordsVolumeName)
			require.NotNil(t, passwordVolume.Projected)
			require.Len(t, passwordVolume.Projected.Sources, len(tt.expectedPasswordFiles))
			actualPasswordFiles := make([]string, 0, len(passwordVolume.Projected.Sources))
			for _, source := range passwordVolume.Projected.Sources {
				require.NotNil(t, source.Secret)
				require.Len(t, source.Secret.Items, 1)
				actualPasswordFiles = append(actualPasswordFiles, source.Secret.Items[0].Path)
			}
			require.ElementsMatch(t, tt.expectedPasswordFiles, actualPasswordFiles)

			require.Len(t, template.PodTemplateSpec.Spec.InitContainers, 2)
			require.Equal(t, reconciliation.ServerConfigContainerName, template.PodTemplateSpec.Spec.InitContainers[0].Name)
			passwordInit := template.PodTemplateSpec.Spec.InitContainers[1]
			require.Equal(t, encryptionPasswordInitContainer, passwordInit.Name)
			require.Equal(t, "client-image", passwordInit.Image)
			require.Equal(t, resource.MustParse("10m"), *passwordInit.Resources.Requests.Cpu())
			require.Equal(t, resource.MustParse("16Mi"), *passwordInit.Resources.Requests.Memory())
			require.Equal(t, resource.MustParse("100m"), *passwordInit.Resources.Limits.Cpu())
			require.Equal(t, resource.MustParse("64Mi"), *passwordInit.Resources.Limits.Memory())
			require.Contains(t, passwordInit.VolumeMounts, corev1.VolumeMount{
				Name:      encryptionPasswordsVolumeName,
				MountPath: encryptionPasswordsMountPath,
				ReadOnly:  true,
			})
			for _, passwordFile := range tt.expectedPasswordFiles {
				require.Contains(t, passwordInit.Command[2], encryptionPasswordsMountPath+"/"+passwordFile)
			}

			cassandraContainer := requireContainer(t, template.PodTemplateSpec.Spec.Containers, reconciliation.CassandraContainerName)
			if tt.expectCassandraEnvVars {
				require.NotNil(t, requireEnvVar(t, cassandraContainer.Env, clientKeystorePasswordEnvName).ValueFrom)
				require.NotNil(t, requireEnvVar(t, cassandraContainer.Env, clientTruststorePasswordEnvName).ValueFrom)
			} else {
				require.Empty(t, cassandraContainer.Env)
			}

			renderedConfig, err := json.Marshal(template)
			require.NoError(t, err)
			require.NotContains(t, string(renderedConfig), "plaintext-password")
		})
	}
}

func TestHandleEncryptionOptionsRequiresPasswordInitImage(t *testing.T) {
	template := newTestEncryptionDatacenterConfig(true, false)

	err := HandleEncryptionOptions(template, "")

	require.EqualError(t, err, "encryption password init container image must not be empty")
}

func TestHandleEncryptionOptionsRejectsInvalidServerStores(t *testing.T) {
	template := newTestEncryptionDatacenterConfig(false, true)
	template.ServerEncryptionStores.TruststoreSecretRef = nil

	err := HandleEncryptionOptions(template, "client-image")

	require.EqualError(t, err, "truststore secret ref is required when EncryptionStores is used")
}

func TestHandleEncryptionOptionsIsIdempotent(t *testing.T) {
	template := newTestEncryptionDatacenterConfig(true, true)

	require.NoError(t, HandleEncryptionOptions(template, "client-image"))
	require.NoError(t, HandleEncryptionOptions(template, "client-image"))

	require.Len(t, template.PodTemplateSpec.Spec.Volumes, 5)
	require.Len(t, template.PodTemplateSpec.Spec.InitContainers, 2)
	cassandraContainer := requireContainer(t, template.PodTemplateSpec.Spec.Containers, reconciliation.CassandraContainerName)
	require.Equal(t, 1, countEnvVars(cassandraContainer.Env, clientKeystorePasswordEnvName))
	require.Equal(t, 1, countEnvVars(cassandraContainer.Env, clientTruststorePasswordEnvName))
	require.Equal(t, 1, countEnvVars(cassandraContainer.Env, "JAVA_TOOL_OPTIONS"))
	require.Equal(t, 1, countStrings(template.CassandraConfig.JvmOptions.AdditionalOptions, "-Dcom.sun.management.jmxremote.ssl=true"))
	require.Equal(t, 1, countStrings(template.CassandraConfig.JvmOptions.AdditionalOptions, "-Dcom.sun.management.jmxremote.ssl.need.client.auth=true"))
}

func TestHandleEncryptionOptionsPreservesJavaToolOptionsAndReplacesPasswordEnv(t *testing.T) {
	template := newTestEncryptionDatacenterConfig(true, false)
	template.PodTemplateSpec.Spec.Containers = []corev1.Container{{
		Name: reconciliation.CassandraContainerName,
		Env: []corev1.EnvVar{
			{Name: "EXISTING", Value: "value"},
			{Name: clientKeystorePasswordEnvName, Value: "old-keystore-password"},
			{Name: clientTruststorePasswordEnvName, Value: "old-truststore-password"},
			{Name: "JAVA_TOOL_OPTIONS", Value: " -Dfoo=bar "},
		},
	}}

	require.NoError(t, HandleEncryptionOptions(template, "client-image"))

	cassandraContainer := requireContainer(t, template.PodTemplateSpec.Spec.Containers, reconciliation.CassandraContainerName)
	require.Contains(t, cassandraContainer.Env, corev1.EnvVar{Name: "EXISTING", Value: "value"})
	require.Equal(t, 1, countEnvVars(cassandraContainer.Env, clientKeystorePasswordEnvName))
	require.Equal(t, 1, countEnvVars(cassandraContainer.Env, clientTruststorePasswordEnvName))
	javaToolOptions := requireEnvVar(t, cassandraContainer.Env, "JAVA_TOOL_OPTIONS")
	require.Equal(
		t,
		"-Dfoo=bar -Djavax.net.ssl.keyStorePassword=$(K8SSANDRA_CLIENT_KEYSTORE_PASSWORD) -Djavax.net.ssl.trustStorePassword=$(K8SSANDRA_CLIENT_TRUSTSTORE_PASSWORD)",
		javaToolOptions.Value,
	)
}

func TestHandleEncryptionOptionsRejectsJavaToolOptionsValueFrom(t *testing.T) {
	template := newTestEncryptionDatacenterConfig(true, false)
	template.PodTemplateSpec.Spec.Containers = []corev1.Container{{
		Name: reconciliation.CassandraContainerName,
		Env: []corev1.EnvVar{{
			Name: "JAVA_TOOL_OPTIONS",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "java-options"},
					Key:                  "options",
				},
			},
		}},
	}}

	err := HandleEncryptionOptions(template, "client-image")

	require.EqualError(t, err, "cassandra container JAVA_TOOL_OPTIONS cannot use valueFrom when client encryption is enabled")
}

func TestEncryptionPasswordHelpersRejectMissingSelectors(t *testing.T) {
	template := &DatacenterConfig{ClientEncryptionStores: &encryption.Stores{}}

	err := addEncryptionPasswordInjection(template, "client-image", true, false)
	require.EqualError(t, err, "password Secret reference for client-keystore-password is required")

	err = addJmxPasswordEnvironment(template)
	require.EqualError(t, err, "client encryption password Secret references are required")
}

func TestValidateEncryptionStoresSecrets(t *testing.T) {
	ctx := context.Background()
	datacenterKey := types.NamespacedName{Namespace: "test", Name: "dc1"}
	logger := logr.Discard()

	t.Run("external secrets bypass", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(true, true)
		template.ExternalSecrets = true

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, fake.NewClientBuilder().Build(), logger)

		require.NoError(t, err)
	})

	t.Run("client encryption succeeds", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(true, false)
		client := fake.NewClientBuilder().WithObjects(
			newEncryptionPasswordSecret("client-keystore-secret", "keystore-password"),
			newEncryptionPasswordSecret("client-truststore-secret", "truststore-password"),
		).Build()

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, client, logger)

		require.NoError(t, err)
	})

	t.Run("client keystore secret is missing", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(true, false)

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, fake.NewClientBuilder().Build(), logger)

		require.ErrorContains(t, err, "client-keystore-secret")
	})

	t.Run("client truststore secret is missing", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(true, false)
		client := fake.NewClientBuilder().WithObjects(
			newEncryptionPasswordSecret("client-keystore-secret", "keystore-password"),
		).Build()

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, client, logger)

		require.ErrorContains(t, err, "client-truststore-secret")
	})

	t.Run("client password key is missing", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(true, false)
		client := fake.NewClientBuilder().WithObjects(
			newEncryptionPasswordSecret("client-keystore-secret", "wrong-key"),
		).Build()

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, client, logger)

		require.ErrorContains(t, err, `missing required key "keystore-password"`)
	})

	t.Run("client stores are invalid", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(true, false)
		template.ClientEncryptionStores.TruststoreSecretRef = nil

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, fake.NewClientBuilder().Build(), logger)

		require.EqualError(t, err, "truststore secret ref is required when EncryptionStores is used")
	})

	t.Run("server encryption succeeds", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(false, true)
		client := fake.NewClientBuilder().WithObjects(
			newEncryptionPasswordSecret("server-keystore-secret", "keystore-password"),
			newEncryptionPasswordSecret("server-truststore-secret", "truststore-password"),
		).Build()

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, client, logger)

		require.NoError(t, err)
	})

	t.Run("server keystore secret is missing", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(false, true)

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, fake.NewClientBuilder().Build(), logger)

		require.ErrorContains(t, err, "server-keystore-secret")
	})

	t.Run("server truststore secret is missing", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(false, true)
		client := fake.NewClientBuilder().WithObjects(
			newEncryptionPasswordSecret("server-keystore-secret", "keystore-password"),
		).Build()

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, client, logger)

		require.ErrorContains(t, err, "server-truststore-secret")
	})

	t.Run("server stores are invalid", func(t *testing.T) {
		template := newTestEncryptionDatacenterConfig(false, true)
		template.ServerEncryptionStores.TruststoreSecretRef = nil

		err := ValidateEncryptionStoresSecrets(ctx, datacenterKey, template, fake.NewClientBuilder().Build(), logger)

		require.EqualError(t, err, "truststore secret ref is required when EncryptionStores is used")
	})
}

func TestValidateEncryptionStorePasswordRejectsMissingSelector(t *testing.T) {
	err := validateEncryptionStorePassword(
		context.Background(),
		"default",
		fake.NewClientBuilder().Build(),
		&encryption.Stores{},
		encryption.StoreNameKeystore,
	)

	require.EqualError(t, err, "password Secret reference for keystore is required")
}

func newTestEncryptionDatacenterConfig(clientEncryption, serverEncryption bool) *DatacenterConfig {
	clientOptions := map[string]interface{}{
		"enabled": clientEncryption,
	}
	if clientEncryption {
		clientOptions["keystore_password"] = "plaintext-password"
		clientOptions["truststore_password"] = "plaintext-password"
	}
	serverOptions := map[string]interface{}{
		"internode_encryption": "none",
	}
	if serverEncryption {
		serverOptions["internode_encryption"] = "all"
		serverOptions["keystore_password"] = "plaintext-password"
		serverOptions["truststore_password"] = "plaintext-password"
	}
	cassandraYaml := unstructured.Unstructured{
		"client_encryption_options": clientOptions,
		"server_encryption_options": serverOptions,
	}
	template := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{CassandraYaml: cassandraYaml},
	}
	if clientEncryption {
		template.ClientEncryptionStores = newTestEncryptionStores("client")
	}
	if serverEncryption {
		template.ServerEncryptionStores = newTestEncryptionStores("server")
	}
	return template
}

func newTestEncryptionStores(prefix string) *encryption.Stores {
	return &encryption.Stores{
		KeystoreSecretRef: &encryption.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: prefix + "-keystore-secret"},
		},
		TruststoreSecretRef: &encryption.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: prefix + "-truststore-secret"},
		},
	}
}

func newEncryptionPasswordSecret(name, key string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: name},
		Data:       map[string][]byte{key: []byte("password")},
	}
}

func requireVolume(t *testing.T, volumes []corev1.Volume, name string) corev1.Volume {
	t.Helper()
	for _, volume := range volumes {
		if volume.Name == name {
			return volume
		}
	}
	require.FailNow(t, "volume not found", name)
	return corev1.Volume{}
}

func requireContainer(t *testing.T, containers []corev1.Container, name string) corev1.Container {
	t.Helper()
	for _, container := range containers {
		if container.Name == name {
			return container
		}
	}
	require.FailNow(t, "container not found", name)
	return corev1.Container{}
}

func requireEnvVar(t *testing.T, envVars []corev1.EnvVar, name string) corev1.EnvVar {
	t.Helper()
	for _, envVar := range envVars {
		if envVar.Name == name {
			return envVar
		}
	}
	require.FailNow(t, "environment variable not found", name)
	return corev1.EnvVar{}
}

func countEnvVars(envVars []corev1.EnvVar, name string) int {
	count := 0
	for _, envVar := range envVars {
		if envVar.Name == name {
			count++
		}
	}
	return count
}

func countStrings(values []string, expected string) int {
	count := 0
	for _, value := range values {
		if value == expected {
			count++
		}
	}
	return count
}
