package cassandra

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StoresMountPath = "/mnt"
)

// Sets up encryption in the datacenter config template.
// The keystore and truststore config maps are mounted into the datacenter pod and the secrets are read to be set in the datacenter config template.
func HandleEncryptionOptions(template *DatacenterConfig, encryptionStoresSecrets EncryptionStoresPasswords) error {
	if ClientEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.ClientEncryptionStores); err != nil {
			return err
		} else {
			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, "client", *template.ClientEncryptionStores)
			// Add JMX encryption jvm options
			addJmxEncryptionOptions(template, *template.ClientEncryptionStores, encryptionStoresSecrets)
		}
	}

	if ServerEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.ServerEncryptionStores); err != nil {
			return err
		} else {
			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, "server", *template.ServerEncryptionStores)
		}
	}
	return nil
}

func addVolumesForEncryption(template *DatacenterConfig, storeType string, encryptionStores encryption.Stores) {
	// Initialize the volume array if it doesn't exist yet
	if template.PodTemplateSpec.Spec.Volumes == nil {
		template.PodTemplateSpec.Spec.Volumes = make([]corev1.Volume, 0)
	}

	volumes := EncryptionVolumes(storeType, encryptionStores)

	for _, volume := range volumes {
		indexKey, foundKey := FindVolume(template.PodTemplateSpec, volume.Name)
		AddOrUpdateVolume(template, &volume, indexKey, foundKey)
	}

	// Find the cassandra container and add the volume mounts for both stores
	addEncryptionMountToCassandra(template, volumes["keystore"], volumes["truststore"], storeType)
}

func EncryptionVolumes(storeType string, encryptionStores encryption.Stores) map[string]corev1.Volume {
	volumes := map[string]corev1.Volume{}

	// Create the volume for the keystore
	keystoreVolume := corev1.Volume{
		Name: fmt.Sprintf("%s-keystore", storeType),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: encryptionStores.KeystoreSecretRef.Name,
				Items: []corev1.KeyToPath{
					{
						Key:  "keystore",
						Path: "keystore",
					},
				},
			},
		},
	}

	// Create the volume for the truststore
	truststoreVolume := corev1.Volume{
		Name: fmt.Sprintf("%s-truststore", storeType),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: encryptionStores.TruststoreSecretRef.Name,
				Items: []corev1.KeyToPath{
					{
						Key:  "truststore",
						Path: "truststore",
					},
				},
			},
		},
	}

	volumes["keystore"] = keystoreVolume
	volumes["truststore"] = truststoreVolume
	return volumes
}

// Adds keystore and truststore volume mounts to the cassandra container.
func addEncryptionMountToCassandra(template *DatacenterConfig, keystoreVolume, truststoreVolume corev1.Volume, storeType string) {
	// Initialize the container array if it doesn't exist yet
	if template.PodTemplateSpec.Spec.Containers == nil {
		template.PodTemplateSpec.Spec.Containers = make([]corev1.Container, 0)
	}

	cassandraContainer := &corev1.Container{
		Name: "cassandra",
	}

	cassandraContainerIdx, foundCassandra := FindContainer(template.PodTemplateSpec, "cassandra")
	if foundCassandra {
		cassandraContainer = template.PodTemplateSpec.Spec.Containers[cassandraContainerIdx].DeepCopy()
	}

	AddOrUpdateVolumeMount(cassandraContainer, &keystoreVolume, StoreMountFullPath(storeType, "keystore"))
	AddOrUpdateVolumeMount(cassandraContainer, &truststoreVolume, StoreMountFullPath(storeType, "truststore"))

	// If the container is not found, add it to the pod spec
	// If the container is found, update it
	if foundCassandra {
		template.PodTemplateSpec.Spec.Containers[cassandraContainerIdx] = *cassandraContainer
	} else {
		template.PodTemplateSpec.Spec.Containers = append(template.PodTemplateSpec.Spec.Containers, *cassandraContainer)
	}
}

func checkMandatoryEncryptionFields(encryptionStores *encryption.Stores) error {
	if encryptionStores == nil {
		return fmt.Errorf("EncryptionStores is required to set up encryption")
	}
	if encryptionStores.KeystoreSecretRef.Name == "" {
		return fmt.Errorf("keystore secret ref was not set")
	}
	if encryptionStores.KeystorePasswordSecretRef.Name == "" {
		return fmt.Errorf("keystore password secret ref was not set")
	}
	if encryptionStores.TruststoreSecretRef.Name == "" {
		return fmt.Errorf("truststore secret ref was not set")
	}
	if encryptionStores.TruststorePasswordSecretRef.Name == "" {
		return fmt.Errorf("truststore password secret ref was not set")
	}
	return nil
}

func StoreMountFullPath(storeType string, storeName string) string {
	return fmt.Sprintf("%s/%s-%s", StoresMountPath, storeType, storeName)
}

func ClientEncryptionEnabled(template *DatacenterConfig) bool {
	return template.CassandraConfig.CassandraYaml.ClientEncryptionOptions != nil && template.CassandraConfig.CassandraYaml.ClientEncryptionOptions.Enabled
}

func ServerEncryptionEnabled(template *DatacenterConfig) bool {
	return template.CassandraConfig.CassandraYaml.ServerEncryptionOptions != nil && template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.Enabled != nil && *template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.Enabled
}

type EncryptionStoresPasswords struct {
	ClientKeystorePassword   string
	ClientTruststorePassword string
	ServerKeystorePassword   string
	ServerTruststorePassword string
}

func ReadEncryptionStoresSecrets(ctx context.Context, klusterKey types.NamespacedName, template *DatacenterConfig, remoteClient client.Client, logger logr.Logger) (EncryptionStoresPasswords, error) {
	encryptionStoresPasswords := EncryptionStoresPasswords{}
	if ClientEncryptionEnabled(template) {
		if template.ClientEncryptionStores == nil || template.ClientEncryptionStores.KeystorePasswordSecretRef.Name == "" {
			return encryptionStoresPasswords, fmt.Errorf("client encryption stores are not properly configured")
		}
		logger.Info("Client to node encryption is enabled, reading client encryption stores secrets")
		// Read client keystore password
		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ClientEncryptionStores.KeystorePasswordSecretRef.Name, "keystore"); err != nil {
			return encryptionStoresPasswords, err
		} else {
			encryptionStoresPasswords.ClientKeystorePassword = password
		}

		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ClientEncryptionStores.TruststorePasswordSecretRef.Name, "truststore"); err != nil {
			return encryptionStoresPasswords, err
		} else {
			encryptionStoresPasswords.ClientTruststorePassword = password
		}
	}

	if ServerEncryptionEnabled(template) {
		logger.Info("Internode encryption is enabled, reading server encryption stores secrets")
		// Read server keystore password
		if template.ServerEncryptionStores == nil || template.ServerEncryptionStores.KeystorePasswordSecretRef.Name == "" {
			return encryptionStoresPasswords, fmt.Errorf("server encryption stores are not properly configured")
		}

		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ServerEncryptionStores.KeystorePasswordSecretRef.Name, "keystore"); err != nil {
			return encryptionStoresPasswords, err
		} else {
			encryptionStoresPasswords.ServerKeystorePassword = password
		}

		// Read server truststore password
		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ServerEncryptionStores.TruststorePasswordSecretRef.Name, "truststore"); err != nil {
			return encryptionStoresPasswords, err
		} else {
			encryptionStoresPasswords.ServerTruststorePassword = password
		}
	}

	return encryptionStoresPasswords, nil
}

func ReadEncryptionStorePassword(ctx context.Context, namespace string, remoteClient client.Client, secretName, storeType string) (string, error) {
	clientKeystoreSecret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: namespace, Name: secretName}
	if err := remoteClient.Get(ctx, secretKey, clientKeystoreSecret); err != nil {
		return "", err
	}
	password := string(clientKeystoreSecret.Data[fmt.Sprintf("%s-password", storeType)])
	return password, nil
}

// Add JVM options required for turning on encryption
func addJmxEncryptionOptions(template *DatacenterConfig, encryptionStores encryption.Stores, encryptionStoresSecrets EncryptionStoresPasswords) {
	addOptionIfMissing(template, "-Dcom.sun.management.jmxremote.ssl=true")
	addOptionIfMissing(template, "-Dcom.sun.management.jmxremote.ssl.need.client.auth=true")
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.keyStore=%s/%s", StoreMountFullPath("client", "keystore"), "keystore"))
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.trustStore=%s/%s", StoreMountFullPath("client", "truststore"), "truststore"))
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.keyStorePassword=%s", encryptionStoresSecrets.ClientKeystorePassword))
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.trustStorePassword=%s", encryptionStoresSecrets.ClientTruststorePassword))
}

func addOptionIfMissing(template *DatacenterConfig, option string) {
	if !utils.SliceContains(template.CassandraConfig.JvmOptions.AdditionalOptions, option) {
		template.CassandraConfig.JvmOptions.AdditionalOptions = append(
			[]string{option},
			template.CassandraConfig.JvmOptions.AdditionalOptions...,
		)
	}
}
