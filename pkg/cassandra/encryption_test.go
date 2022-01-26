package cassandra

import (
	"fmt"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestCheckMandatoryEncryptionFields(t *testing.T) {
	dcConfig := &DatacenterConfig{
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: api.CassandraYaml{
				ClientEncryptionOptions: &api.ClientEncryptionOptions{
					Enabled: true,
				},
				ServerEncryptionOptions: &api.ServerEncryptionOptions{
					Enabled: pointer.Bool(true),
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-password-secret",
			},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			},
		},
	}

	noErr := checkMandatoryEncryptionFields(dcConfig.ClientEncryptionStores)
	assert.NoError(t, noErr)

	err := checkMandatoryEncryptionFields(dcConfig.ServerEncryptionStores)
	assert.Error(t, err)
}

func TestAddEncryptionMountToCassandra(t *testing.T) {
	dcConfig := &DatacenterConfig{
		PodTemplateSpec: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{},
		},
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: api.CassandraYaml{
				ClientEncryptionOptions: &api.ClientEncryptionOptions{
					Enabled: true,
				},
				ServerEncryptionOptions: &api.ServerEncryptionOptions{
					Enabled: pointer.Bool(true),
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-password-secret",
			},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-password-secret",
			},
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

	addEncryptionMountToCassandra(dcConfig, clientKeystoreVolume, clientTruststoreVolume, "client")
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

	addEncryptionMountToCassandra(dcConfig, serverKeystoreVolume, serverTruststoreVolume, "server")
	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts))
	assert.Equal(t, "server-keystore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[2].Name)
	assert.Equal(t, "server-truststore", dcConfig.PodTemplateSpec.Spec.Containers[0].VolumeMounts[3].Name)
}

func TestAddVolumesForEncryption(t *testing.T) {
	dcConfig := &DatacenterConfig{
		PodTemplateSpec: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{},
		},
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: api.CassandraYaml{
				ClientEncryptionOptions: &api.ClientEncryptionOptions{
					Enabled: true,
				},
				ServerEncryptionOptions: &api.ServerEncryptionOptions{
					Enabled: pointer.Bool(true),
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-password-secret",
			},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-password-secret",
			},
		},
	}

	addVolumesForEncryption(dcConfig, "client", *dcConfig.ClientEncryptionStores)
	assert.Equal(t, 2, len(dcConfig.PodTemplateSpec.Spec.Volumes))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-keystore", "client-keystore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "client-truststore", "client-truststore-secret"))

	addVolumesForEncryption(dcConfig, "server", *dcConfig.ServerEncryptionStores)
	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Volumes))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-truststore", "server-truststore-secret"))
	assert.True(t, volumeExists(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore"))
	assert.True(t, volumeHasSecretSource(dcConfig.PodTemplateSpec.Spec.Volumes, "server-keystore", "server-keystore-secret"))
}

func TestHandleEncryptionOptions(t *testing.T) {
	// Test a succeeding case with both client and server encryption turned on
	dcConfig := &DatacenterConfig{
		PodTemplateSpec: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{},
		},
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: api.CassandraYaml{
				ClientEncryptionOptions: &api.ClientEncryptionOptions{
					Enabled: true,
				},
				ServerEncryptionOptions: &api.ServerEncryptionOptions{
					Enabled: pointer.Bool(true),
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-password-secret",
			},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-password-secret",
			},
		},
	}

	encryptionStoresSecrets := EncryptionStoresPasswords{
		ClientKeystorePassword:   "test",
		ClientTruststorePassword: "test",
	}
	HandleEncryptionOptions(dcConfig, encryptionStoresSecrets)
	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Volumes))
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
	for _, jvmOption := range []string{"-Dcom.sun.management.jmxremote.ssl=true", "-Dcom.sun.management.jmxremote.ssl.need.client.auth=true", fmt.Sprintf("-Djavax.net.ssl.keyStore=%s/keystore", StoreMountFullPath("client", "keystore")), fmt.Sprintf("-Djavax.net.ssl.trustStore=%s/truststore", StoreMountFullPath("client", "truststore")), fmt.Sprintf("-Djavax.net.ssl.keyStorePassword=%s", encryptionStoresSecrets.ClientKeystorePassword), fmt.Sprintf("-Djavax.net.ssl.trustStorePassword=%s", encryptionStoresSecrets.ClientTruststorePassword)} {
		assert.True(t, utils.SliceContains(dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, jvmOption), fmt.Sprintf("JVM option %s not found", jvmOption))
	}
}

func TestHandleEncryptionOptionsWithExistingContainers(t *testing.T) {
	// Test a succeeding case with both client and server encryption turned on
	dcConfig := &DatacenterConfig{
		PodTemplateSpec: &corev1.PodTemplateSpec{
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
			CassandraYaml: api.CassandraYaml{
				ClientEncryptionOptions: &api.ClientEncryptionOptions{
					Enabled: true,
				},
				ServerEncryptionOptions: &api.ServerEncryptionOptions{
					Enabled: pointer.Bool(true),
				},
			},
		},
		ClientEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "client-truststore-password-secret",
			},
		},
		ServerEncryptionStores: &encryption.Stores{
			KeystoreSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-secret",
			},
			KeystorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-keystore-password-secret",
			},
			TruststoreSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-secret",
			},
			TruststorePasswordSecretRef: corev1.LocalObjectReference{
				Name: "server-truststore-password-secret",
			},
		},
	}
	encryptionStoresSecrets := EncryptionStoresPasswords{
		ClientKeystorePassword:   "test",
		ClientTruststorePassword: "test",
	}
	HandleEncryptionOptions(dcConfig, encryptionStoresSecrets)
	assert.Equal(t, 4, len(dcConfig.PodTemplateSpec.Spec.Volumes))

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
		PodTemplateSpec: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{},
		},
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: api.CassandraYaml{
				ClientEncryptionOptions: &api.ClientEncryptionOptions{
					Enabled: false,
				},
				ServerEncryptionOptions: &api.ServerEncryptionOptions{
					Enabled: pointer.Bool(false),
				},
			},
		},
	}
	encryptionStoresSecrets := EncryptionStoresPasswords{
		ClientKeystorePassword:   "test",
		ClientTruststorePassword: "test",
	}

	err := HandleEncryptionOptions(dcConfig, encryptionStoresSecrets)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(dcConfig.PodTemplateSpec.Spec.Volumes))
	assert.Equal(t, 0, len(dcConfig.PodTemplateSpec.Spec.Containers))
}

func TestHandleFailedEncryptionOptions(t *testing.T) {
	// Test a succeeding case with missing encryption options
	dcConfig := &DatacenterConfig{
		PodTemplateSpec: &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{},
		},
		CassandraConfig: api.CassandraConfig{
			CassandraYaml: api.CassandraYaml{
				ClientEncryptionOptions: &api.ClientEncryptionOptions{
					Enabled: true,
				},
				ServerEncryptionOptions: &api.ServerEncryptionOptions{
					Enabled: pointer.Bool(false),
				},
			},
		},
	}
	encryptionStoresSecrets := EncryptionStoresPasswords{
		ClientKeystorePassword:   "test",
		ClientTruststorePassword: "test",
	}
	HandleEncryptionOptions(dcConfig, encryptionStoresSecrets)
	err := HandleEncryptionOptions(dcConfig, encryptionStoresSecrets)
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
			if volume.VolumeSource.Secret != nil {
				if volume.VolumeSource.Secret.SecretName == secretName {
					return true
				}
			}
		}
	}
	return false
}
