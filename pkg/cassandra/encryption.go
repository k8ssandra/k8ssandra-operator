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

// HandleEncryptionOptions sets up encryption in the datacenter config template.
// The keystore and truststore config maps are mounted into the datacenter pod and the secrets are read to be set in the datacenter config template.
func handleEncryptionOptions(template *DatacenterConfig) error {
	if ClientEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.ClientEncryptionStores); err != nil {
			return err
		} else {
			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, encryption.StoreTypeClient, *template.ClientEncryptionStores)
			// Add JMX encryption jvm options
			addJmxEncryptionOptions(template)
		}
	}

	if ServerEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.ServerEncryptionStores); err != nil {
			return err
		} else {
			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, encryption.StoreTypeServer, *template.ServerEncryptionStores)
		}
	}
	return nil
}

func addVolumesForEncryption(template *DatacenterConfig, storeType encryption.StoreType, encryptionStores encryption.Stores) {
	// Initialize the volume array if it doesn't exist yet
	if template.PodTemplateSpec.Spec.Volumes == nil {
		template.PodTemplateSpec.Spec.Volumes = make([]corev1.Volume, 0)
	}

	keystoreVolume, truststoreVolume := EncryptionVolumes(storeType, encryptionStores)

	for _, volume := range []*corev1.Volume{keystoreVolume, truststoreVolume} {
		if volume != nil {
			indexKey, foundKey := FindVolume(template.PodTemplateSpec, volume.Name)
			AddOrUpdateVolume(template, volume, indexKey, foundKey)
		}
	}

	// Find the cassandra container and add the volume mounts for both stores
	addEncryptionMountToCassandra(template, keystoreVolume, truststoreVolume, storeType)
}

func EncryptionVolumes(storeType encryption.StoreType, encryptionStores encryption.Stores) (*corev1.Volume, *corev1.Volume) {
	// Create the volume for the keystore
	keystoreVolume := corev1.Volume{
		Name: fmt.Sprintf("%s-keystore", storeType),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: encryptionStores.KeystoreSecretRef.Name,
				Items: []corev1.KeyToPath{
					{
						Key:  string(encryption.StoreNameKeystore),
						Path: string(encryption.StoreNameKeystore),
					},
				},
			},
		},
	}

	truststoreVolume := corev1.Volume{
		Name: fmt.Sprintf("%s-truststore", storeType),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: encryptionStores.TruststoreSecretRef.Name,
				Items: []corev1.KeyToPath{
					{
						Key:  string(encryption.StoreNameTruststore),
						Path: string(encryption.StoreNameTruststore),
					},
				},
			},
		},
	}
	return &keystoreVolume, &truststoreVolume
}

// Adds keystore and truststore volume mounts to the cassandra container.
func addEncryptionMountToCassandra(template *DatacenterConfig, keystoreVolume, truststoreVolume *corev1.Volume, storeType encryption.StoreType) {
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

	AddOrUpdateVolumeMount(cassandraContainer, keystoreVolume, StoreMountFullPath(storeType, encryption.StoreNameKeystore))
	if truststoreVolume != nil {
		AddOrUpdateVolumeMount(cassandraContainer, truststoreVolume, StoreMountFullPath(storeType, encryption.StoreNameTruststore))
	}

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
	return nil
}

func StoreMountFullPath(storeType encryption.StoreType, storeName encryption.StoreName) string {
	return fmt.Sprintf("%s/%s-%s", StoresMountPath, storeType, storeName)
}

func ClientEncryptionEnabled(template *DatacenterConfig) bool {
	return template.CassandraConfig.CassandraYaml.ClientEncryptionOptions != nil && template.CassandraConfig.CassandraYaml.ClientEncryptionOptions.Enabled
}

func ServerEncryptionEnabled(template *DatacenterConfig) bool {
	return template.CassandraConfig.CassandraYaml.ServerEncryptionOptions != nil && template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.InternodeEncryption != "none"
}

func ReadEncryptionStoresSecrets(ctx context.Context, klusterKey types.NamespacedName, template *DatacenterConfig, remoteClient client.Client, logger logr.Logger) error {
	if ClientEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.ClientEncryptionStores); err != nil {
			return err
		}
		logger.Info("Client to node encryption is enabled, reading client encryption stores secrets")
		// Read client keystore password
		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ClientEncryptionStores.KeystoreSecretRef.Name, encryption.StoreNameKeystore); err != nil {
			return err
		} else {
			template.ClientKeystorePassword = password
		}

		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ClientEncryptionStores.TruststoreSecretRef.Name, encryption.StoreNameTruststore); err != nil {
			return err
		} else {
			template.ClientTruststorePassword = password
		}
	}

	if ServerEncryptionEnabled(template) {
		logger.Info("Internode encryption is enabled, reading server encryption stores secrets")
		// Read server keystore password
		if err := checkMandatoryEncryptionFields(template.ServerEncryptionStores); err != nil {
			return err
		}

		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ServerEncryptionStores.KeystoreSecretRef.Name, encryption.StoreNameKeystore); err != nil {
			return err
		} else {
			logger.Info(fmt.Sprintf("Read keystore password %s", password))
			template.ServerKeystorePassword = password
		}

		// Read server truststore password
		if password, err := ReadEncryptionStorePassword(ctx, klusterKey.Namespace, remoteClient, template.ServerEncryptionStores.TruststoreSecretRef.Name, encryption.StoreNameTruststore); err != nil {
			return err
		} else {
			logger.Info(fmt.Sprintf("Read truststore password %s", password))
			template.ServerTruststorePassword = password
		}
	}

	return nil
}

func ReadEncryptionStorePassword(ctx context.Context, namespace string, remoteClient client.Client, secretName string, storeName encryption.StoreName) (string, error) {
	clientKeystoreSecret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: namespace, Name: secretName}
	if err := remoteClient.Get(ctx, secretKey, clientKeystoreSecret); err != nil {
		return "", err
	}
	password := string(clientKeystoreSecret.Data[fmt.Sprintf("%s-password", storeName)])
	return password, nil
}

// Add JVM options required for turning on encryption
func addJmxEncryptionOptions(template *DatacenterConfig) {
	addOptionIfMissing(template, "-Dcom.sun.management.jmxremote.ssl=true")
	addOptionIfMissing(template, "-Dcom.sun.management.jmxremote.ssl.need.client.auth=true")
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.keyStore=%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameKeystore), encryption.StoreNameKeystore))
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.trustStore=%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameTruststore), encryption.StoreNameTruststore))
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.keyStorePassword=%s", template.ClientKeystorePassword))
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.trustStorePassword=%s", template.ClientTruststorePassword))
}

func addOptionIfMissing(template *DatacenterConfig, option string) {
	if !utils.SliceContains(template.CassandraConfig.JvmOptions.AdditionalOptions, option) {
		template.CassandraConfig.JvmOptions.AdditionalOptions = append(
			[]string{option},
			template.CassandraConfig.JvmOptions.AdditionalOptions...,
		)
	}
}
