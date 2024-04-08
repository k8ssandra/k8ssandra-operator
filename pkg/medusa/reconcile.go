package medusa

import (
	"bytes"
	"fmt"
	"os"
	reflect "reflect"
	"text/template"

	"github.com/adutra/goalesce"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultMedusaImageRepository = "k8ssandra"
	DefaultMedusaImageName       = "medusa"
	DefaultMedusaVersion         = "0.20.1"
	DefaultMedusaPort            = 50051
	DefaultProbeInitialDelay     = 10
	DefaultProbeTimeout          = 1
	DefaultProbePeriod           = 10
	DefaultProbeSuccessThreshold = 1
	DefaultProbeFailureThreshold = 10

	InitContainerMemRequest = "100Mi"
	InitContainerMemLimit   = "8Gi"
	InitContainerCpuRequest = "100m"
	MainContainerMemRequest = "100Mi"
	MainContainerMemLimit   = "8Gi"
	MainContainerCpuRequest = "100m"

	MedusaBackupsVolumeName  = "medusa-backups"
	MedusaBackupsMountPath   = "/mnt/backups"
	serviceAccountNameEnvVar = "SERVICE_ACCOUNT_NAME"
)

var (
	defaultMedusaImage = images.Image{
		Registry:   images.DefaultRegistry,
		Repository: DefaultMedusaImageRepository,
		Name:       DefaultMedusaImageName,
		Tag:        DefaultMedusaVersion,
	}
)

func CreateMedusaIni(kc *k8ss.K8ssandraCluster, dcConfig *cassandra.DatacenterConfig) string {
	medusaIniTemplate := `
    [cassandra]
    use_sudo = false
    {{- if .Spec.Medusa.CertificatesSecretRef.Name }}
    certfile = /etc/certificates/rootca.crt
    usercert = /etc/certificates/client.crt_signed
    userkey = /etc/certificates/client.key
    {{- end}}

    [storage]
    use_sudo_for_restore = false
    storage_provider = {{ .Spec.Medusa.StorageProperties.StorageProvider }}
    bucket_name = {{ .Spec.Medusa.StorageProperties.BucketName }}
    {{- if .Spec.Medusa.StorageProperties.StorageSecretRef.Name }}
    key_file = /etc/medusa-secrets/credentials
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.Prefix }}
    prefix = {{ .Spec.Medusa.StorageProperties.Prefix }}
    {{- else }}
    prefix = {{ .Name }}
    {{- end }}
    max_backup_age = {{ .Spec.Medusa.StorageProperties.MaxBackupAge }}
    max_backup_count = {{ .Spec.Medusa.StorageProperties.MaxBackupCount }}
    {{- if .Spec.Medusa.StorageProperties.Region }}
    region = {{ .Spec.Medusa.StorageProperties.Region }}
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.Host }}
    host = {{ .Spec.Medusa.StorageProperties.Host }}
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.Port }}
    port = {{ .Spec.Medusa.StorageProperties.Port }}
    {{- end }}
    {{- if not .Spec.Medusa.StorageProperties.Secure }}
    secure = False
    {{- else }}
    secure = True
    {{- end }}
    {{- if not .Spec.Medusa.StorageProperties.SslVerify }}
    ssl_verify = False
    {{- else }}
    ssl_verify = True
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.BackupGracePeriodInDays }}
    backup_grace_period_in_days = {{ .Spec.Medusa.StorageProperties.BackupGracePeriodInDays }}
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.ApiProfile }}
    api_profile = {{ .Spec.Medusa.StorageProperties.ApiProfile }}
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.TransferMaxBandwidth }}
    transfer_max_bandwidth = {{ .Spec.Medusa.StorageProperties.TransferMaxBandwidth }}
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.ConcurrentTransfers }}
    concurrent_transfers = {{ .Spec.Medusa.StorageProperties.ConcurrentTransfers }}
    {{- end }}
    {{- if .Spec.Medusa.StorageProperties.MultiPartUploadThreshold }}
    multi_part_upload_threshold = {{ .Spec.Medusa.StorageProperties.MultiPartUploadThreshold }}
    {{- end }}

    [grpc]
    enabled = 1

    [logging]
    level = DEBUG

    [kubernetes]
    use_mgmt_api = 1
    enabled = 1`

	kcWithProperlyConcurrentMedusa := kc.DeepCopy()
	if kc.Spec.Medusa.StorageProperties.ConcurrentTransfers == 0 {
		kcWithProperlyConcurrentMedusa.Spec.Medusa.StorageProperties.ConcurrentTransfers = 1
	}
	t, err := template.New("ini").Parse(medusaIniTemplate)
	if err != nil {
		panic(err)
	}
	medusaIni := new(bytes.Buffer)
	err = t.Execute(medusaIni, kcWithProperlyConcurrentMedusa)
	if err != nil {
		panic(err)
	}

	medusaConfig := medusaIni.String()

	// Create Kubernetes config here and append it
	if dcConfig.ManagementApiAuth != nil && dcConfig.ManagementApiAuth.Manual != nil {
		medusaConfig += `
    cassandra_url = https://127.0.0.1:8080/api/v0/ops/node/snapshots
    ca_cert = /etc/encryption/mgmt/ca.crt
    tls_cert = /etc/encryption/mgmt/tls.crt
    tls_key = /etc/encryption/mgmt/tls.key`
	} else {
		medusaConfig += `
    cassandra_url = http://127.0.0.1:8080/api/v0/ops/node/snapshots`
	}

	return medusaConfig
}

func CreateMedusaConfigMap(namespace, k8cName, medusaIni string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-medusa", k8cName),
			Namespace: namespace,
		},
		Data: map[string]string{
			"medusa.ini": medusaIni,
		},
	}
}

// CassandraUserSecretName gets the cassandra user secret name from the medusa spec or generate one
// based on the cluster name.
func CassandraUserSecretName(medusaSpec *api.MedusaClusterTemplate, k8cName string) string {
	if medusaSpec.CassandraUserSecretRef.Name == "" {
		return fmt.Sprintf("%s-medusa", k8cName)
	}
	return medusaSpec.CassandraUserSecretRef.Name
}

func UpdateMedusaInitContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate, useExternalSecrets bool, k8cName string, logger logr.Logger) {
	restoreContainerIndex, found := cassandra.FindInitContainer(&dcConfig.PodTemplateSpec, "medusa-restore")
	restoreContainer := &corev1.Container{Name: "medusa-restore"}
	if found {
		logger.Info("Found medusa-restore init container")
		// medusa-restore init container already exists, we may need to update it
		restoreContainer = dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex].DeepCopy()
	}
	setImage(medusaSpec.ContainerImage, restoreContainer)
	restoreContainer.SecurityContext = medusaSpec.SecurityContext
	restoreContainer.Env = medusaEnvVars(medusaSpec, k8cName, useExternalSecrets, "RESTORE")
	restoreContainer.VolumeMounts = medusaVolumeMounts(dcConfig, medusaSpec, k8cName)
	restoreContainer.Resources = medusaInitContainerResources(medusaSpec)

	if !found {
		logger.Info("Couldn't find medusa-restore init container")
		// medusa-restore init container doesn't exist, we need to add it
		// We'll add the server-config-init init container in first position so it initializes cassandra.yaml for Medusa to use, if it doesn't exist already.
		// The definition of that container will be completed later by cass-operator.
		_, serverConfigInitContainerFound := cassandra.FindInitContainer(&dcConfig.PodTemplateSpec, "server-config-init")
		if !serverConfigInitContainerFound {
			serverConfigContainer := &corev1.Container{Name: "server-config-init"}
			dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, *serverConfigContainer)
		}
		dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, *restoreContainer)
	} else {
		// Overwrite existing medusa-restore init container
		dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex] = *restoreContainer
	}
}

func medusaInitContainerResources(medusaSpec *api.MedusaClusterTemplate) corev1.ResourceRequirements {
	if medusaSpec.InitContainerResources != nil {
		return *medusaSpec.InitContainerResources
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(InitContainerCpuRequest),
			corev1.ResourceMemory: resource.MustParse(InitContainerMemRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(InitContainerMemLimit),
		},
	}
}

func CreateMedusaMainContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate, useExternalSecrets bool, k8cName string, logger logr.Logger) (*corev1.Container, error) {
	medusaContainer := &corev1.Container{Name: "medusa"}
	setImage(medusaSpec.ContainerImage, medusaContainer)
	medusaContainer.SecurityContext = medusaSpec.SecurityContext
	medusaContainer.Env = medusaEnvVars(medusaSpec, k8cName, useExternalSecrets, "GRPC")
	medusaContainer.Ports = []corev1.ContainerPort{
		{
			Name:          "grpc",
			ContainerPort: DefaultMedusaPort,
			Protocol:      "TCP",
		},
	}

	readinessProbe, err := generateMedusaProbe(medusaSpec.ReadinessProbe)
	if err != nil {
		return nil, err
	}
	livenessProbe, err := generateMedusaProbe(medusaSpec.LivenessProbe)
	if err != nil {
		return nil, err
	}

	medusaContainer.ReadinessProbe = readinessProbe
	medusaContainer.LivenessProbe = livenessProbe
	medusaContainer.VolumeMounts = medusaVolumeMounts(dcConfig, medusaSpec, k8cName)
	medusaContainer.Resources = medusaMainContainerResources(medusaSpec)
	return medusaContainer, nil
}

func UpdateMedusaMainContainer(dcConfig *cassandra.DatacenterConfig, medusaContainer *corev1.Container) {
	cassandra.UpdateContainer(&dcConfig.PodTemplateSpec, "medusa", func(c *corev1.Container) {
		*c = *medusaContainer
	})
}

func medusaMainContainerResources(medusaSpec *api.MedusaClusterTemplate) corev1.ResourceRequirements {
	if medusaSpec.Resources != nil {
		return *medusaSpec.Resources
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(MainContainerCpuRequest),
			corev1.ResourceMemory: resource.MustParse(MainContainerMemRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(MainContainerMemLimit),
		},
	}
}

// Build the image name and pull policy and add it to a medusa container definition
func setImage(containerImage *images.Image, container *corev1.Container) {
	image := containerImage.ApplyDefaults(defaultMedusaImage)
	container.Image = image.String()
	container.ImagePullPolicy = image.PullPolicy
}

func medusaVolumeMounts(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate, k8cName string) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{ // Cassandra config volume
			Name:      "server-config",
			MountPath: "/etc/cassandra",
		},
		{ // Cassandra data volume
			Name:      "server-data",
			MountPath: "/var/lib/cassandra",
		},
		{ // Medusa config volume
			Name:      fmt.Sprintf("%s-medusa", k8cName),
			MountPath: "/etc/medusa",
		},
		{ // Pod info volume
			Name:      "podinfo",
			MountPath: "/etc/podinfo",
		},
	}

	// Mount client encryption certificates if the secret ref is provided.
	if medusaSpec.CertificatesSecretRef.Name != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "certificates",
			MountPath: "/etc/certificates",
		})
	}

	// Mount secret with Medusa storage backend credentials if the secret ref is provided.
	if medusaSpec.StorageProperties.StorageSecretRef.Name != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      medusaSpec.StorageProperties.StorageSecretRef.Name,
			MountPath: "/etc/medusa-secrets",
		})
	}

	// Management-api encryption client certificates
	if dcConfig.ManagementApiAuth != nil && dcConfig.ManagementApiAuth.Manual != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "mgmt-encryption",
			MountPath: "/etc/encryption/mgmt",
		})
	}

	return volumeMounts
}

func medusaEnvVars(medusaSpec *api.MedusaClusterTemplate, k8cName string, useExternalSecrets bool, mode string) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "MEDUSA_MODE",
			Value: mode,
		},
		{
			Name:  "MEDUSA_TMP_DIR",
			Value: "/var/lib/cassandra",
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}

	if useExternalSecrets {
		return envVars
	}

	secretRefs := []corev1.EnvVar{
		{
			Name: "CQL_USERNAME",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: CassandraUserSecretName(medusaSpec, k8cName),
					},
					Key: "username",
				},
			},
		},
		{
			Name: "CQL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: CassandraUserSecretName(medusaSpec, k8cName),
					},
					Key: "password",
				},
			},
		},
	}
	return append(envVars, secretRefs...)
}

type medusaVolume struct {
	Volume      *corev1.Volume
	VolumeIndex int
	Exists      bool
}

// Create or update volumes for medusa
func GenerateMedusaVolumes(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate, k8cName string) []medusaVolume {
	var newVolumes []medusaVolume

	// Medusa config volume, containing medusa.ini
	configVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, fmt.Sprintf("%s-medusa", k8cName))
	configVolume := &corev1.Volume{
		Name: fmt.Sprintf("%s-medusa", k8cName),
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("%s-medusa", k8cName),
				},
			},
		},
	}

	newVolumes = append(newVolumes, medusaVolume{
		Volume:      configVolume,
		VolumeIndex: configVolumeIndex,
		Exists:      found,
	})

	// Medusa credentials volume using the referenced secret
	if medusaSpec.StorageProperties.StorageSecretRef.Name != "" {
		secretVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, medusaSpec.StorageProperties.StorageSecretRef.Name)
		secretVolume := &corev1.Volume{
			Name: medusaSpec.StorageProperties.StorageSecretRef.Name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: medusaSpec.StorageProperties.StorageSecretRef.Name,
				},
			},
		}

		newVolumes = append(newVolumes, medusaVolume{
			Volume:      secretVolume,
			VolumeIndex: secretVolumeIndex,
			Exists:      found,
		})
	}

	// Pod info volume
	podInfoVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, "podinfo")
	podInfoVolume := &corev1.Volume{
		Name: "podinfo",
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: "labels",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.labels",
						},
					},
				},
			},
		},
	}

	newVolumes = append(newVolumes, medusaVolume{
		Volume:      podInfoVolume,
		VolumeIndex: podInfoVolumeIndex,
		Exists:      found,
	})

	// Encryption client certificates
	if medusaSpec.CertificatesSecretRef.Name != "" {
		encryptionClientVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, "certificates")
		encryptionClientVolume := &corev1.Volume{
			Name: "certificates",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: medusaSpec.CertificatesSecretRef.Name,
				},
			},
		}

		newVolumes = append(newVolumes, medusaVolume{
			Volume:      encryptionClientVolume,
			VolumeIndex: encryptionClientVolumeIndex,
			Exists:      found,
		})
	}

	// Management-api client certificates
	if dcConfig.ManagementApiAuth != nil && dcConfig.ManagementApiAuth.Manual != nil {
		managementApiVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, "mgmt-encryption")
		managementApiVolume := &corev1.Volume{
			Name: "mgmt-encryption",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: dcConfig.ManagementApiAuth.Manual.ClientSecretName,
				},
			},
		}

		newVolumes = append(newVolumes, medusaVolume{
			Volume:      managementApiVolume,
			VolumeIndex: managementApiVolumeIndex,
			Exists:      found,
		})
	}

	return newVolumes
}

func MedusaServiceName(clusterName string, dcName string) string {
	return fmt.Sprintf("%s-%s-medusa-service", clusterName, dcName)
}

func MedusaStandaloneDeploymentName(clusterName string, dcName string) string {
	return fmt.Sprintf("%s-%s-medusa-standalone", clusterName, dcName)
}

func MedusaPurgeCronJobName(clusterName string, dcName string) string {
	return fmt.Sprintf("%s-%s-medusa-purge", clusterName, dcName)
}

func StandaloneMedusaDeployment(medusaContainer corev1.Container, clusterName, dcName, namespace string, logger logr.Logger, medusaImage *images.Image) *appsv1.Deployment {
	// The standalone medusa pod won't be able to resolve its own IP address using DNS entries
	medusaContainer.Env = append(medusaContainer.Env, corev1.EnvVar{Name: "MEDUSA_RESOLVE_IP_ADDRESSES", Value: "False"})
	medusaDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MedusaStandaloneDeploymentName(clusterName, dcName),
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": MedusaStandaloneDeploymentName(clusterName, dcName),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": MedusaStandaloneDeploymentName(clusterName, dcName),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						medusaContainer,
					},
					ImagePullSecrets: images.CollectPullSecrets(medusaImage.ApplyDefaults(defaultMedusaImage)),
					Volumes:          []corev1.Volume{},
					Hostname:         MedusaStandaloneDeploymentName(clusterName, dcName),
				},
			},
		},
	}

	logger.Info("Creating standalone medusa deployment on namespace", "namespace", medusaDeployment.Namespace)
	// Create dummy additional volumes
	// These volumes won't be used by the Medusa standalone pod, but the mounts will be necessary for Medusa to startup properly
	// TODO: Investigate if we can adapt Medusa to remove these volumes
	for _, extraVolume := range [](string){"server-config", "server-data", MedusaBackupsVolumeName} {
		medusaDeployment.Spec.Template.Spec.Volumes = append(medusaDeployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: extraVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	return medusaDeployment
}

func StandaloneMedusaService(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate, clusterName, namespace string, logger logr.Logger) *corev1.Service {
	medusaService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MedusaServiceName(clusterName, dcConfig.SanitizedName()),
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       DefaultMedusaPort,
					TargetPort: intstr.FromInt(DefaultMedusaPort),
				},
			},
			Selector: map[string]string{
				"app": MedusaStandaloneDeploymentName(clusterName, dcConfig.SanitizedName()),
			},
		},
	}

	return medusaService
}

func PurgeCronJob(dcConfig *cassandra.DatacenterConfig, clusterName, namespace string, logger logr.Logger) (*batchv1.CronJob, error) {
	cronJobName := MedusaPurgeCronJobName(cassdcapi.CleanupForKubernetes(clusterName), dcConfig.SanitizedName())
	logger.Info(fmt.Sprintf("Creating Medusa purge backups cronjob: %s", cronJobName))
	if len(cronJobName) > 253 {
		return nil, fmt.Errorf("Medusa purge backups cronjob name too long (must be less than 253 characters). Length: %d, Job name: %s", len(cronJobName), cronJobName)
	}
	purgeCronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   "0 0 * * *",
			Suspend:                    pointer.Bool(false),
			SuccessfulJobsHistoryLimit: pointer.Int32(3),
			FailedJobsHistoryLimit:     pointer.Int32(1),
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy:      corev1.RestartPolicyOnFailure,
							ServiceAccountName: getServiceAccountName(),
							Containers: []corev1.Container{
								{
									Name:                     "k8ssandra-purge-backups",
									Image:                    "bitnami/kubectl:1.29.3",
									ImagePullPolicy:          corev1.PullIfNotPresent,
									TerminationMessagePath:   "/dev/termination-log",
									TerminationMessagePolicy: "File",
									Command: []string{
										"/bin/bash",
										"-c",
										createPurgeTaskStr(dcConfig.SanitizedName(), namespace),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return purgeCronJob, nil
}

func generateMedusaProbe(configuredProbe *corev1.Probe) (*corev1.Probe, error) {
	// Goalesce the custom probe with the default probe,
	defaultProbe := defaultMedusaProbe()
	if configuredProbe == nil {
		return defaultProbe, nil
	}
	mergedProbe := goalesce.MustDeepMerge(*defaultProbe, *configuredProbe)
	if !reflect.DeepEqual(defaultProbe.ProbeHandler, mergedProbe.ProbeHandler) {
		// If the user has configured a custom probe, use it
		// Otherwise, use the default probe
		return nil, fmt.Errorf("invalid probe configuration. You should not modify the probe handler.")
	}

	return &mergedProbe, nil
}

func defaultMedusaProbe() *corev1.Probe {
	// Goalesce the custom probe with the default probe,
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"/bin/grpc_health_probe", fmt.Sprintf("--addr=:%d", DefaultMedusaPort)},
			},
		},
		InitialDelaySeconds: DefaultProbeInitialDelay,
		TimeoutSeconds:      DefaultProbeTimeout,
		PeriodSeconds:       DefaultProbePeriod,
		SuccessThreshold:    DefaultProbeSuccessThreshold,
		FailureThreshold:    DefaultProbeFailureThreshold,
	}

	return probe
}

func createPurgeTaskStr(dcName string, namespace string) string {
	return fmt.Sprintf("printf \""+
		"apiVersion: medusa.k8ssandra.io/v1alpha1\\n"+
		"kind: MedusaTask\\n"+
		"metadata:\\n"+
		"  name: purge-backups-timestamp\\n"+
		"  namespace: %s\\n"+
		"spec:\\n"+
		"  cassandraDatacenter: %s\\n"+
		"  operation: purge"+
		"\" "+
		"| sed \"s/timestamp/$(date +%%Y%%m%%d%%H%%M%%S)/g\" "+
		"| kubectl apply -f -", namespace, dcName)
}

func getServiceAccountName() string {
	val, found := os.LookupEnv(serviceAccountNameEnvVar)
	if !found {
		return "default"
	}
	return val
}
