package cassandra

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
