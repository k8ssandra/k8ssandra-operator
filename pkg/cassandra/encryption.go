package cassandra

import (
	"context"
	"fmt"

	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StoresMountPath = "/mnt"
)

// Sets up encryption in the datacenter config template.
// The keystore and truststore config maps are mounted into the datacenter pod and the secrets are read to be set in the datacenter config template.
func HandleEncryptionOptions(template *DatacenterConfig) error {
	if ClientEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.CassandraConfig.CassandraYaml.ClientEncryptionOptions.EncryptionStores); err != nil {
			return err
		} else {
			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, "client", *template.CassandraConfig.CassandraYaml.ClientEncryptionOptions.EncryptionStores)
		}
	}

	if ServerEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.EncryptionStores); err != nil {
			return err
		} else {
			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, "server", *template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.EncryptionStores)
		}
	}
	return nil
}

func addVolumesForEncryption(template *DatacenterConfig, storeType string, encryptionStores v1alpha1.EncryptionStores) {
	// Initialize the volume array if it doesn't exist yet
	if template.PodTemplateSpec.Spec.Volumes == nil {
		template.PodTemplateSpec.Spec.Volumes = make([]corev1.Volume, 0)
	}

	// Create the volume for the keystore
	keystoreVolume := &corev1.Volume{
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

	indexKey, foundKey := FindVolume(template.PodTemplateSpec, keystoreVolume.Name)
	AddOrUpdateVolume(template, keystoreVolume, indexKey, foundKey)

	// Create the volume for the truststore
	truststoreVolume := &corev1.Volume{
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

	indexTrust, foundTrust := FindVolume(template.PodTemplateSpec, truststoreVolume.Name)
	AddOrUpdateVolume(template, truststoreVolume, indexTrust, foundTrust)

	// Find the cassandra container and add the volume mounts for both stores
	addEncryptionMountToCassandra(template, keystoreVolume, truststoreVolume, storeType)
}

// Adds keystore and truststore volume mounts to the cassandra container.
func addEncryptionMountToCassandra(template *DatacenterConfig, keystoreVolume, truststoreVolume *corev1.Volume, storeType string) {
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

	AddOrUpdateVolumeMount(cassandraContainer, keystoreVolume, StoreMountFullPath(storeType, "keystore"))
	AddOrUpdateVolumeMount(cassandraContainer, truststoreVolume, StoreMountFullPath(storeType, "truststore"))

	// If the container is not found, add it to the pod spec
	// If the container is found, update it
	if foundCassandra {
		template.PodTemplateSpec.Spec.Containers[cassandraContainerIdx] = *cassandraContainer
	} else {
		template.PodTemplateSpec.Spec.Containers = append(template.PodTemplateSpec.Spec.Containers, *cassandraContainer)
	}
}

func checkMandatoryEncryptionFields(encryptionStores *v1alpha1.EncryptionStores) error {
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
	return template.CassandraConfig.CassandraYaml.ServerEncryptionOptions != nil && template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.Enabled
}

type EncryptionStoresPasswords struct {
	ClientKeystorePassword   string
	ClientTruststorePassword string
	ServerKeystorePassword   string
	ServerTruststorePassword string
}

func ReadEncryptionStoresSecrets(ctx context.Context, klusterKey types.NamespacedName, template *DatacenterConfig, remoteClient client.Client) (EncryptionStoresPasswords, error) {
	encryptionStoresPasswords := EncryptionStoresPasswords{}
	if ClientEncryptionEnabled(template) {
		// Read client keystore password
		clientKeystoreSecret := &corev1.Secret{}
		secretKey := types.NamespacedName{Namespace: klusterKey.Namespace, Name: template.CassandraConfig.CassandraYaml.ClientEncryptionOptions.EncryptionStores.KeystorePasswordSecretRef.Name}
		if err := remoteClient.Get(ctx, secretKey, clientKeystoreSecret); err != nil {
			return encryptionStoresPasswords, err
		}
		encryptionStoresPasswords.ClientKeystorePassword = string(clientKeystoreSecret.Data["password"])

		// Read client truststore password
		clientTruststoreSecret := &corev1.Secret{}
		secretKey = types.NamespacedName{Namespace: klusterKey.Namespace, Name: template.CassandraConfig.CassandraYaml.ClientEncryptionOptions.EncryptionStores.TruststorePasswordSecretRef.Name}
		if err := remoteClient.Get(ctx, secretKey, clientTruststoreSecret); err != nil {
			return encryptionStoresPasswords, err
		}
		encryptionStoresPasswords.ClientTruststorePassword = string(clientTruststoreSecret.Data["password"])
	}

	if ServerEncryptionEnabled(template) {
		// Read server keystore password
		serverKeystoreSecret := &corev1.Secret{}
		secretKey := types.NamespacedName{Namespace: klusterKey.Namespace, Name: template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.EncryptionStores.KeystorePasswordSecretRef.Name}
		if err := remoteClient.Get(ctx, secretKey, serverKeystoreSecret); err != nil {
			return encryptionStoresPasswords, err
		}
		encryptionStoresPasswords.ServerKeystorePassword = string(serverKeystoreSecret.Data["password"])

		// Read server truststore password
		serverTruststoreSecret := &corev1.Secret{}
		secretKey = types.NamespacedName{Namespace: klusterKey.Namespace, Name: template.CassandraConfig.CassandraYaml.ServerEncryptionOptions.EncryptionStores.TruststorePasswordSecretRef.Name}
		if err := remoteClient.Get(ctx, secretKey, serverTruststoreSecret); err != nil {
			return encryptionStoresPasswords, err
		}
		encryptionStoresPasswords.ServerTruststorePassword = string(serverTruststoreSecret.Data["password"])
	}

	return encryptionStoresPasswords, nil
}
