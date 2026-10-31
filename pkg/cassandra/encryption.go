package cassandra

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StoresMountPath                  = "/mnt"
	encryptionPasswordsMountPath     = "/etc/k8ssandra/encryption-passwords"
	encryptionPasswordsVolumeName    = "k8ssandra-encryption-passwords"
	encryptionPasswordInitContainer  = "k8ssandra-encryption-passwords"
	clientKeystorePasswordEnvName    = "K8SSANDRA_CLIENT_KEYSTORE_PASSWORD"
	clientTruststorePasswordEnvName  = "K8SSANDRA_CLIENT_TRUSTSTORE_PASSWORD"
	clientKeystorePasswordFileName   = "client-keystore-password"
	clientTruststorePasswordFileName = "client-truststore-password"
	serverKeystorePasswordFileName   = "server-keystore-password"
	serverTruststorePasswordFileName = "server-truststore-password"
)

// HandleEncryptionOptions sets up encryption in the datacenter config template. The keystore and
// truststore Secrets are mounted into the datacenter pod. Passwords are projected only at runtime
// and injected into the generated Cassandra configuration by an init container.
func HandleEncryptionOptions(template *DatacenterConfig, passwordInitImage string) error {
	clientEncryptionEnabled := ClientEncryptionEnabled(template)
	serverEncryptionEnabled := ServerEncryptionEnabled(template)

	if clientEncryptionEnabled {
		if template.ExternalSecrets {
			// enables encryption options, but doesn't set keystore/trustore path and passwords
			enableEncryptionOptions(template)
		} else {
			if err := checkMandatoryEncryptionFields(template.ClientEncryptionStores); err != nil {
				return err
			}
			removeEncryptionPasswordFields(template, "client_encryption_options")
			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, encryption.StoreTypeClient, *template.ClientEncryptionStores)
			// Add JMX encryption jvm options
			addJmxEncryptionOptions(template)

			keystorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameKeystore), encryption.StoreNameKeystore)
			truststorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameTruststore), encryption.StoreNameTruststore)
			template.CassandraConfig.CassandraYaml.PutIfAbsent("client_encryption_options/keystore", keystorePath)
			template.CassandraConfig.CassandraYaml.PutIfAbsent("client_encryption_options/truststore", truststorePath)
		}
	}

	if serverEncryptionEnabled {
		if !template.ExternalSecrets {
			if err := checkMandatoryEncryptionFields(template.ServerEncryptionStores); err != nil {
				return err
			}
			removeEncryptionPasswordFields(template, "server_encryption_options")

			// Create the volume and mount for the keystore
			addVolumesForEncryption(template, encryption.StoreTypeServer, *template.ServerEncryptionStores)

			keystorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeServer, encryption.StoreNameKeystore), encryption.StoreNameKeystore)
			truststorePath := fmt.Sprintf("%s/%s", StoreMountFullPath(encryption.StoreTypeServer, encryption.StoreNameTruststore), encryption.StoreNameTruststore)
			template.CassandraConfig.CassandraYaml.PutIfAbsent("server_encryption_options/keystore", keystorePath)
			template.CassandraConfig.CassandraYaml.PutIfAbsent("server_encryption_options/truststore", truststorePath)
		}
	}

	if !template.ExternalSecrets && (clientEncryptionEnabled || serverEncryptionEnabled) {
		if passwordInitImage == "" {
			return fmt.Errorf("encryption password init container image must not be empty")
		}
		if err := addEncryptionPasswordInjection(template, passwordInitImage, clientEncryptionEnabled, serverEncryptionEnabled); err != nil {
			return err
		}
	}
	return nil
}

func removeEncryptionPasswordFields(template *DatacenterConfig, section string) {
	options, found := template.CassandraConfig.CassandraYaml.Get(section)
	if !found {
		return
	}
	optionsMap, ok := options.(map[string]interface{})
	if !ok {
		return
	}
	delete(optionsMap, "keystore_password")
	delete(optionsMap, "truststore_password")
}

func addVolumesForEncryption(template *DatacenterConfig, storeType encryption.StoreType, encryptionStores encryption.Stores) {
	// Initialize the volume array if it doesn't exist yet
	if template.PodTemplateSpec.Spec.Volumes == nil {
		template.PodTemplateSpec.Spec.Volumes = make([]corev1.Volume, 0)
	}

	keystoreVolume, truststoreVolume := EncryptionVolumes(storeType, encryptionStores)

	for _, volume := range []*corev1.Volume{keystoreVolume, truststoreVolume} {
		if volume != nil {
			indexKey, foundKey := FindVolume(&template.PodTemplateSpec, volume.Name)
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
						Key:  encryptionStores.KeystoreSecretRef.GetSpecificKeyOrDefault(string(encryption.StoreNameKeystore)),
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
						Key:  encryptionStores.TruststoreSecretRef.GetSpecificKeyOrDefault(string(encryption.StoreNameTruststore)),
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

	cassandraContainerIdx, foundCassandra := FindContainer(&template.PodTemplateSpec, "cassandra")
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
	if encryptionStores.KeystoreSecretRef == nil {
		return fmt.Errorf("keystore secret ref is required when EncryptionStores is used")
	}
	if encryptionStores.TruststoreSecretRef == nil {
		return fmt.Errorf("truststore secret ref is required when EncryptionStores is used")
	}
	return nil
}

func StoreMountFullPath(storeType encryption.StoreType, storeName encryption.StoreName) string {
	return fmt.Sprintf("%s/%s-%s", StoresMountPath, storeType, storeName)
}

func ClientEncryptionEnabled(template *DatacenterConfig) bool {
	if template.ClientEncryptionStores == nil {
		return false
	}

	enabled, _ := template.CassandraConfig.CassandraYaml.Get("client_encryption_options/enabled")
	return enabled == true
}

func ServerEncryptionEnabled(template *DatacenterConfig) bool {
	// This allows to bypass the automated behavior, but of course removes the idea of validating this field also
	if template.ServerEncryptionStores == nil {
		return false
	}

	internodeEncryption, _ := template.CassandraConfig.CassandraYaml.Get("server_encryption_options/internode_encryption")
	return internodeEncryption != nil && internodeEncryption != "none"
}

func ValidateEncryptionStoresSecrets(ctx context.Context, datacenterKey types.NamespacedName, template *DatacenterConfig, remoteClient client.Client, logger logr.Logger) error {
	if template.ExternalSecrets {
		logger.Info("ExternalSecrets enabled, operator will not verify existence of encryption stores secrets")
		return nil
	}

	if ClientEncryptionEnabled(template) {
		if err := checkMandatoryEncryptionFields(template.ClientEncryptionStores); err != nil {
			return err
		}
		logger.Info("Client to node encryption is enabled, validating client encryption stores secrets")
		if err := validateEncryptionStorePassword(ctx, datacenterKey.Namespace, remoteClient, template.ClientEncryptionStores, encryption.StoreNameKeystore); err != nil {
			return err
		}

		if err := validateEncryptionStorePassword(ctx, datacenterKey.Namespace, remoteClient, template.ClientEncryptionStores, encryption.StoreNameTruststore); err != nil {
			return err
		}
	}

	if ServerEncryptionEnabled(template) {
		logger.Info("Internode encryption is enabled, validating server encryption stores secrets")
		if err := checkMandatoryEncryptionFields(template.ServerEncryptionStores); err != nil {
			return err
		}

		if err := validateEncryptionStorePassword(ctx, datacenterKey.Namespace, remoteClient, template.ServerEncryptionStores, encryption.StoreNameKeystore); err != nil {
			return err
		}

		if err := validateEncryptionStorePassword(ctx, datacenterKey.Namespace, remoteClient, template.ServerEncryptionStores, encryption.StoreNameTruststore); err != nil {
			return err
		}
	}

	return nil
}

func validateEncryptionStorePassword(ctx context.Context, namespace string, remoteClient client.Client, stores *encryption.Stores, storeName encryption.StoreName) error {
	selector := stores.PasswordSecretKeySelector(storeName)
	if selector == nil {
		return fmt.Errorf("password Secret reference for %s is required", storeName)
	}
	passwordSecretObj := &corev1.Secret{}
	secretObjKey := types.NamespacedName{Namespace: namespace, Name: selector.Name}
	if err := remoteClient.Get(ctx, secretObjKey, passwordSecretObj); err != nil {
		return err
	}
	if _, found := passwordSecretObj.Data[selector.Key]; !found {
		return fmt.Errorf("secret %s is missing required key %q", secretObjKey, selector.Key)
	}
	return nil
}

// Add JVM options required for turning on encryption
func addJmxEncryptionOptions(template *DatacenterConfig) {
	enableEncryptionOptions(template)
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.keyStore=%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameKeystore), encryption.StoreNameKeystore))
	addOptionIfMissing(template, fmt.Sprintf("-Djavax.net.ssl.trustStore=%s/%s", StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameTruststore), encryption.StoreNameTruststore))
}

func addEncryptionPasswordInjection(template *DatacenterConfig, image string, clientEncryptionEnabled, serverEncryptionEnabled bool) error {
	type passwordProjection struct {
		selector *corev1.SecretKeySelector
		path     string
		yamlPath string
	}

	projections := make([]passwordProjection, 0, 4)
	if clientEncryptionEnabled {
		projections = append(projections,
			passwordProjection{
				selector: template.ClientEncryptionStores.PasswordSecretKeySelector(encryption.StoreNameKeystore),
				path:     clientKeystorePasswordFileName,
				yamlPath: ".client_encryption_options.keystore_password",
			},
			passwordProjection{
				selector: template.ClientEncryptionStores.PasswordSecretKeySelector(encryption.StoreNameTruststore),
				path:     clientTruststorePasswordFileName,
				yamlPath: ".client_encryption_options.truststore_password",
			},
		)
	}
	if serverEncryptionEnabled {
		projections = append(projections,
			passwordProjection{
				selector: template.ServerEncryptionStores.PasswordSecretKeySelector(encryption.StoreNameKeystore),
				path:     serverKeystorePasswordFileName,
				yamlPath: ".server_encryption_options.keystore_password",
			},
			passwordProjection{
				selector: template.ServerEncryptionStores.PasswordSecretKeySelector(encryption.StoreNameTruststore),
				path:     serverTruststorePasswordFileName,
				yamlPath: ".server_encryption_options.truststore_password",
			},
		)
	}

	volumeSources := make([]corev1.VolumeProjection, 0, len(projections))
	yamlAssignments := make([]string, 0, len(projections))
	for _, projection := range projections {
		if projection.selector == nil {
			return fmt.Errorf("password Secret reference for %s is required", projection.path)
		}
		volumeSources = append(volumeSources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: projection.selector.LocalObjectReference,
				Items: []corev1.KeyToPath{{
					Key:  projection.selector.Key,
					Path: projection.path,
				}},
			},
		})
		yamlAssignments = append(
			yamlAssignments,
			fmt.Sprintf(`%s = load_str("%s/%s")`, projection.yamlPath, encryptionPasswordsMountPath, projection.path),
		)
	}

	passwordVolume := &corev1.Volume{
		Name: encryptionPasswordsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{Sources: volumeSources},
		},
	}
	index, found := FindVolume(&template.PodTemplateSpec, passwordVolume.Name)
	AddOrUpdateVolume(template, passwordVolume, index, found)

	// Declare the config builder placeholder first so password injection runs after cassandra.yaml is generated.
	UpdateInitContainer(&template.PodTemplateSpec, reconciliation.ServerConfigContainerName, func(*corev1.Container) {})
	UpdateInitContainer(&template.PodTemplateSpec, encryptionPasswordInitContainer, func(container *corev1.Container) {
		container.Image = image
		container.Command = []string{
			"sh",
			"-c",
			fmt.Sprintf(`set -eu; yq -i '%s' /config/cassandra.yaml`, strings.Join(yamlAssignments, " | ")),
		}
		container.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		}
		container.VolumeMounts = []corev1.VolumeMount{
			{Name: "server-config", MountPath: "/config"},
			{Name: encryptionPasswordsVolumeName, MountPath: encryptionPasswordsMountPath, ReadOnly: true},
		}
	})

	if clientEncryptionEnabled {
		return addJmxPasswordEnvironment(template)
	}
	return nil
}

func addJmxPasswordEnvironment(template *DatacenterConfig) error {
	keystoreSelector := template.ClientEncryptionStores.PasswordSecretKeySelector(encryption.StoreNameKeystore)
	truststoreSelector := template.ClientEncryptionStores.PasswordSecretKeySelector(encryption.StoreNameTruststore)
	if keystoreSelector == nil || truststoreSelector == nil {
		return fmt.Errorf("client encryption password Secret references are required")
	}

	var envErr error
	UpdateCassandraContainer(&template.PodTemplateSpec, func(container *corev1.Container) {
		existingJavaToolOptions := ""
		env := make([]corev1.EnvVar, 0, len(container.Env)+3)
		for _, envVar := range container.Env {
			switch envVar.Name {
			case clientKeystorePasswordEnvName, clientTruststorePasswordEnvName:
				continue
			case "JAVA_TOOL_OPTIONS":
				if envVar.ValueFrom != nil {
					envErr = fmt.Errorf("cassandra container JAVA_TOOL_OPTIONS cannot use valueFrom when client encryption is enabled")
					return
				}
				existingJavaToolOptions = strings.TrimSpace(envVar.Value)
			default:
				env = append(env, envVar)
			}
		}

		env = append(env,
			corev1.EnvVar{
				Name:      clientKeystorePasswordEnvName,
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: keystoreSelector},
			},
			corev1.EnvVar{
				Name:      clientTruststorePasswordEnvName,
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: truststoreSelector},
			},
		)

		jmxPasswordOptions := fmt.Sprintf(
			"-Djavax.net.ssl.keyStorePassword=$(%s) -Djavax.net.ssl.trustStorePassword=$(%s)",
			clientKeystorePasswordEnvName,
			clientTruststorePasswordEnvName,
		)
		if existingJavaToolOptions != "" {
			jmxPasswordOptions = existingJavaToolOptions + " " + jmxPasswordOptions
		}
		container.Env = append(env, corev1.EnvVar{Name: "JAVA_TOOL_OPTIONS", Value: jmxPasswordOptions})
	})
	return envErr
}

func enableEncryptionOptions(template *DatacenterConfig) {
	addOptionIfMissing(template, "-Dcom.sun.management.jmxremote.ssl=true")
	addOptionIfMissing(template, "-Dcom.sun.management.jmxremote.ssl.need.client.auth=true")
}

func addOptionIfMissing(template *DatacenterConfig, option string) {
	if !utils.SliceContains(template.CassandraConfig.JvmOptions.AdditionalOptions, option) {
		template.CassandraConfig.JvmOptions.AdditionalOptions = append(
			[]string{option},
			template.CassandraConfig.JvmOptions.AdditionalOptions...,
		)
	}
}
