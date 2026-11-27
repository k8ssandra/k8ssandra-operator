package medusa

import (
	"bytes"
	"fmt"
	"maps"
	reflect "reflect"
	"strconv"
	"text/template"
	"time"

	"github.com/adutra/goalesce"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	cassimages "github.com/k8ssandra/cass-operator/pkg/images"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	DefaultMedusaPort            = api.DefaultGrpcPort
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

	MedusaBackupsVolumeName = "medusa-backups"
	MedusaBackupsMountPath  = "/mnt/backups"

	CredentialsTypeRoleBased = "role-based"

	MedusaClientSecretNameAnnotation = "medusa.k8ssandra.io/grpc-client-secret-name"
	MedusaGRPCPortAnnotation         = "medusa.k8ssandra.io/grpc-port"

	// MedusaTaskConcurrencyAnnotation sets how many Cassandra pods a purge or prepare_restore MedusaTask runs on at the
	// same time: TaskConcurrencyAll, TaskConcurrencyRack or TaskConcurrencyPod. It is read from the MedusaTask, copied
	// from a MedusaBackupSchedule to the purge tasks it creates, and copied from the K8ssandraCluster to the purge
	// schedules the operator creates.
	MedusaTaskConcurrencyAnnotation = "medusa.k8ssandra.io/task-concurrency"

	// MedusaPitrHashAnnotation is set on the Cassandra pod template to a hash of spec.medusa.pitr.
	// Medusa arms commit log archiving at pod start only, so a change to the hash rolls the pods.
	// It is absent when spec.medusa.pitr is nil, so clusters without PITR see no pod template change.
	MedusaPitrHashAnnotation = "k8ssandra.io/medusa-pitr-hash"

	// DefaultPitrSpoolDir mirrors Medusa's [pitr] commitlog_spool_dir default.
	// A change to that default in Medusa must be mirrored here.
	DefaultPitrSpoolDir = "/var/lib/cassandra/commitlog_spool_dir"

	// pitrPreStopScript is the preStop hook of the medusa container when pitr is enabled.
	//
	// It first waits for the management API port to close, and only then for the spool to be empty.
	// Cassandra's drain archives the last commit log segment while the port is still open, so checking
	// the spool first could see it empty before that segment arrives.
	//
	// Only a refused connection counts as "cassandra is gone". Any other error means "still up", because
	// waiting too long is the safe failure.
	//
	// Only regular files count in the spool, as in Medusa's archiver, so a stray directory or symlink
	// cannot hold the hook for the whole grace period.
	pitrPreStopScript = `import os, socket, sys, time
port, spool = int(sys.argv[1]), sys.argv[2]
def cassandra_up():
    try:
        socket.create_connection(("127.0.0.1", port), timeout=1).close()
        return True
    except ConnectionRefusedError:
        return False
    except OSError:
        return True
def spool_has_files():
    try:
        with os.scandir(spool) as entries:
            return any(e.is_file(follow_symlinks=False) for e in entries)
    except (FileNotFoundError, NotADirectoryError):
        return False
while cassandra_up():
    time.sleep(1)
while spool_has_files():
    time.sleep(1)
`

	// defaultPitrArchiveIntervalSeconds is the archive interval assumed for the grace period when
	// spec.medusa.pitr.archiveIntervalSeconds is unset.
	defaultPitrArchiveIntervalSeconds = 60

	// MedusaPurgeTimeoutAnnotation overrides DefaultPurgeTimeout for purge operations. The value is a Go
	// duration, for example "36h". It is read from the purge MedusaTask. The MedusaBackupSchedule copies it to
	// the purge tasks it creates, and the K8ssandraCluster copies it to the purge schedules it manages.
	MedusaPurgeTimeoutAnnotation = "medusa.k8ssandra.io/purge-timeout"
)

// PurgeTaskAnnotations are the annotations copied from a K8ssandraCluster to the purge MedusaBackupSchedules it
// manages, and from a purge MedusaBackupSchedule to the MedusaTasks it creates.
var PurgeTaskAnnotations = []string{MedusaTaskConcurrencyAnnotation, MedusaPurgeTimeoutAnnotation}

// CopyPurgeTaskAnnotations copies the PurgeTaskAnnotations set on src to dst.
func CopyPurgeTaskAnnotations(src, dst metav1.Object) {
	for _, key := range PurgeTaskAnnotations {
		if value, found := src.GetAnnotations()[key]; found {
			annotations := dst.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[key] = value
			dst.SetAnnotations(annotations)
		}
	}
}

// PurgeTimeout returns the timeout of a purge operation: the MedusaPurgeTimeoutAnnotation value of obj, or
// DefaultPurgeTimeout when it is not set. An unparsable or non-positive value returns DefaultPurgeTimeout and
// an error, so the purge still has a bound.
func PurgeTimeout(obj metav1.Object) (time.Duration, error) {
	value, found := obj.GetAnnotations()[MedusaPurgeTimeoutAnnotation]
	if !found {
		return DefaultPurgeTimeout, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil {
		return DefaultPurgeTimeout, fmt.Errorf("invalid %s annotation value %q: %w", MedusaPurgeTimeoutAnnotation, value, err)
	}
	if timeout <= 0 {
		return DefaultPurgeTimeout, fmt.Errorf("invalid %s annotation value %q: must be positive", MedusaPurgeTimeoutAnnotation, value)
	}
	return timeout, nil
}

func CreateMedusaIni(kc *k8ss.K8ssandraCluster, dcConfig *cassandra.DatacenterConfig) string {
	medusaIniTemplate := `
    [cassandra]
    use_sudo = false
    {{- if .Spec.Medusa.CertificatesSecretRef.Name }}
    certfile = /etc/certificates/rootca.crt
    usercert = /etc/certificates/client.crt_signed
    userkey = /etc/certificates/client.key
	{{- else if .Spec.Medusa.ClientEncryptionStores }}
	  {{- if .Spec.Medusa.ClientEncryptionStores.KeystoreSecretRef }}
    certfile = /etc/certificates/certfile/{{ .Spec.Medusa.ClientEncryptionStores.KeystoreSecretRef.Key }}
	  {{- end}}
	  {{- if .Spec.Medusa.ClientEncryptionStores.TruststoreSecretRef }}
    usercert = /etc/certificates/usercert/{{ .Spec.Medusa.ClientEncryptionStores.TruststoreSecretRef.Key }}
	  {{- end}}
	  {{- if .Spec.Medusa.ClientEncryptionStores.TruststorePasswordSecretRef }}
    userkey = /etc/certificates/userkey/{{ .Spec.Medusa.ClientEncryptionStores.TruststorePasswordSecretRef.Key }}
	  {{- end}}
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
    {{- if .Spec.Medusa.StorageProperties.MultiPartChunkSize }}
    multipart_chunksize = {{ .Spec.Medusa.StorageProperties.MultiPartChunkSize }}
    {{- end }}

    [grpc]
    enabled = 1
    {{- if .Spec.Medusa.ServiceProperties.GrpcPort }}
    port = {{ .Spec.Medusa.ServiceProperties.GrpcPort }}
    {{- end }}
	{{- if .Spec.Medusa.ServiceProperties.Encryption }}
    ca_cert = /etc/certificates/grpc-server-certs/ca.crt
    tls_cert = /etc/certificates/grpc-server-certs/tls.crt
    tls_key = /etc/certificates/grpc-server-certs/tls.key
	{{- end }}

    [monitoring]
    prometheus_port = {{ .Spec.Medusa.ServiceProperties.MetricsPortOrDefault }}
    {{- if .Spec.Medusa.ServiceProperties.MetricsTLSEnabled }}
    prometheus_tls_cert = /etc/certificates/grpc-server-certs/tls.crt
    prometheus_tls_key = /etc/certificates/grpc-server-certs/tls.key
    prometheus_ca_cert = /etc/certificates/grpc-server-certs/ca.crt
    {{- end }}
    {{- if and .Spec.Medusa.Pitr .Spec.Medusa.Pitr.Enabled }}

    [pitr]
    enabled = true
    {{- if .Spec.Medusa.Pitr.ArchiveIntervalSeconds }}
    commitlog_archive_interval_seconds = {{ .Spec.Medusa.Pitr.ArchiveIntervalSeconds }}
    {{- end }}
    {{- if .Spec.Medusa.Pitr.SpoolDir }}
    commitlog_spool_dir = {{ .Spec.Medusa.Pitr.SpoolDir }}
    {{- end }}
    {{- end }}

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

	if kc.Spec.Medusa.ServiceProperties.Encryption != nil && kc.Spec.Medusa.ServiceProperties.Encryption.ClientSecretName != "" {
		dcConfig.Meta.Annotations = goalesce.MustDeepMerge(dcConfig.Meta.Annotations, map[string]string{MedusaClientSecretNameAnnotation: kc.Spec.Medusa.ServiceProperties.Encryption.ClientSecretName})
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

func UpdateMedusaInitContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate, useExternalSecrets bool, k8cName string, logger logr.Logger, registry cassimages.ImageRegistry) {
	restoreContainerIndex, found := cassandra.FindInitContainer(&dcConfig.PodTemplateSpec, "medusa-restore")
	restoreContainer := &corev1.Container{Name: "medusa-restore"}
	if found {
		logger.Info("Found medusa-restore init container")
		// medusa-restore init container already exists, we may need to update it
		restoreContainer = dcConfig.PodTemplateSpec.Spec.InitContainers[restoreContainerIndex].DeepCopy()
	}
	setImage(registry, medusaSpec.ContainerImage, restoreContainer)
	// Add default image pull secrets from registry
	if registry != nil {
		names := registry.GetImagePullSecrets()
		for _, n := range names {
			exists := false
			for _, s := range dcConfig.PodTemplateSpec.Spec.ImagePullSecrets {
				if s.Name == n {
					exists = true
					break
				}
			}
			if !exists {
				dcConfig.PodTemplateSpec.Spec.ImagePullSecrets = append(dcConfig.PodTemplateSpec.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: n})
			}
		}
	}
	restoreContainer.SecurityContext = medusaSpec.SecurityContext
	restoreContainer.Env = medusaEnvVars(medusaSpec, k8cName, useExternalSecrets, "RESTORE")
	restoreContainer.VolumeMounts = medusaVolumeMounts(dcConfig, medusaSpec, k8cName)
	restoreContainer.Resources = medusaInitContainerResources(medusaSpec)

	if medusaSpec.Pitr != nil {
		// The map may be shared with the K8ssandraCluster spec, so copy it before writing.
		annotations := map[string]string{}
		maps.Copy(annotations, dcConfig.PodTemplateSpec.Annotations)
		annotations[MedusaPitrHashAnnotation] = utils.DeepHashString(medusaSpec.Pitr)
		dcConfig.PodTemplateSpec.Annotations = annotations
	}

	if medusaSpec.Pitr != nil && medusaSpec.Pitr.Enabled {
		// The preStop hook of the medusa container runs against the pod's grace period.
		// cass-operator's default (reconciliation.DefaultTerminationGracePeriodSeconds) covers the
		// drain and the exit of the cassandra container. The hook then waits up to one archive
		// interval for the next archiver tick, and the upload itself needs about one more interval in
		// the worst case. Hence 120 + 2 × interval. A larger value already on the pod template is
		// kept, so this never lowers a grace period the user chose. When pitr is nil or disabled the
		// field is not touched: it stays nil and cass-operator applies its default, so pod templates
		// without pitr do not change.
		interval := int64(defaultPitrArchiveIntervalSeconds)
		if medusaSpec.Pitr.ArchiveIntervalSeconds != nil {
			interval = int64(*medusaSpec.Pitr.ArchiveIntervalSeconds)
		}
		want := reconciliation.DefaultTerminationGracePeriodSeconds + 2*interval
		current := dcConfig.PodTemplateSpec.Spec.TerminationGracePeriodSeconds
		if current == nil || *current < want {
			dcConfig.PodTemplateSpec.Spec.TerminationGracePeriodSeconds = ptr.To(want)
		}
	}

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

// pitrPreStopLifecycle returns the preStop hook of the medusa container, or nil when pitr is nil or disabled.
func pitrPreStopLifecycle(dcConfig *cassandra.DatacenterConfig, pitr *api.PitrConfig) *corev1.Lifecycle {
	if pitr == nil || !pitr.Enabled {
		return nil
	}
	port := httphelper.GetMgmtApiPort(dcConfig.PodTemplateSpec.Spec.Containers)
	spool := pitr.SpoolDir
	if spool == "" {
		spool = DefaultPitrSpoolDir
	}
	return &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"python3", "-c", pitrPreStopScript, strconv.Itoa(port), spool},
			},
		},
	}
}

func CreateMedusaMainContainer(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate, useExternalSecrets bool, k8cName string, logger logr.Logger, registry cassimages.ImageRegistry) (*corev1.Container, error) {
	medusaContainer := &corev1.Container{Name: "medusa"}
	setImage(registry, medusaSpec.ContainerImage, medusaContainer)
	// Add default image pull secrets from registry
	if registry != nil {
		names := registry.GetImagePullSecrets()
		for _, n := range names {
			exists := false
			for _, s := range dcConfig.PodTemplateSpec.Spec.ImagePullSecrets {
				if s.Name == n {
					exists = true
					break
				}
			}
			if !exists {
				dcConfig.PodTemplateSpec.Spec.ImagePullSecrets = append(dcConfig.PodTemplateSpec.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: n})
			}
		}
	}
	medusaContainer.SecurityContext = medusaSpec.SecurityContext
	medusaContainer.Env = medusaEnvVars(medusaSpec, k8cName, useExternalSecrets, "GRPC")
	var grpcPort = DefaultMedusaPort
	if medusaSpec.ServiceProperties.GrpcPort != 0 {
		grpcPort = medusaSpec.ServiceProperties.GrpcPort
		dcConfig.Meta.Annotations = goalesce.MustDeepMerge(dcConfig.Meta.Annotations, map[string]string{MedusaGRPCPortAnnotation: fmt.Sprintf("%d", grpcPort)})
	}
	medusaContainer.Ports = []corev1.ContainerPort{
		{
			Name:          "grpc",
			ContainerPort: int32(grpcPort),
			Protocol:      "TCP",
		},
		{
			Name:          "metrics",
			ContainerPort: int32(medusaSpec.ServiceProperties.MetricsPortOrDefault()),
			Protocol:      "TCP",
		},
	}

	readinessProbe, err := generateMedusaProbe(medusaSpec.ReadinessProbe, grpcPort, medusaSpec.ServiceProperties.Encryption)
	if err != nil {
		return nil, err
	}
	livenessProbe, err := generateMedusaProbe(medusaSpec.LivenessProbe, grpcPort, medusaSpec.ServiceProperties.Encryption)
	if err != nil {
		return nil, err
	}

	medusaContainer.ReadinessProbe = readinessProbe
	medusaContainer.LivenessProbe = livenessProbe
	medusaContainer.Lifecycle = pitrPreStopLifecycle(dcConfig, medusaSpec.Pitr)
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
func setImage(registry cassimages.ImageRegistry, containerImage *images.Image, container *corev1.Container) {
	// Fallback to legacy image defaults

	image := registry.Image("medusa")
	if containerImage != nil {
		image.ApplyOverrides(containerImage.Convert())
	}

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
		{ // Medusa's /tmp backed by emptyDir to support RO FS
			Name:      "medusa-tmp",
			MountPath: "/tmp",
		},
	}

	// Mount client encryption certificates if the secret ref is provided.
	if medusaSpec.CertificatesSecretRef.Name != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "certificates",
			MountPath: "/etc/certificates",
		})
	} else if medusaSpec.ClientEncryptionStores != nil {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "certfile",
			MountPath: "/etc/certificates/certfile",
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "usercert",
			MountPath: "/etc/certificates/usercert",
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "userkey",
			MountPath: "/etc/certificates/userkey",
		})
	}

	// Mount gRPC server encryption certificates if secretName is provided
	if medusaSpec.ServiceProperties.Encryption != nil && medusaSpec.ServiceProperties.Encryption.ServerSecretName != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "grpc-server-certs",
			MountPath: "/etc/certificates/grpc-server-certs",
		})
	}

	// Mount replicated secret with Medusa storage backend credentials if the secret ref is provided.
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
	if volumes := calculateClientEncryptionVolumes(dcConfig, medusaSpec); len(volumes) > 0 {
		newVolumes = append(newVolumes, volumes...)
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

	// empty dir to back /tmp
	emptyDirVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, "medusa-tmp")
	emptyDirVolume := &corev1.Volume{
		Name: "medusa-tmp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	newVolumes = append(newVolumes, medusaVolume{
		Volume:      emptyDirVolume,
		VolumeIndex: emptyDirVolumeIndex,
		Exists:      found,
	})

	if medusaSpec.ServiceProperties.Encryption != nil && medusaSpec.ServiceProperties.Encryption.ServerSecretName != "" {
		grpcServerCertsVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, "grpc-server-certs")
		grpcServerCertsVolume := &corev1.Volume{
			Name: "grpc-server-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: medusaSpec.ServiceProperties.Encryption.ServerSecretName,
				},
			},
		}
		newVolumes = append(newVolumes, medusaVolume{
			Volume:      grpcServerCertsVolume,
			VolumeIndex: grpcServerCertsVolumeIndex,
			Exists:      found,
		})
	}

	return newVolumes
}

func calculateClientEncryptionVolumes(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate) []medusaVolume {
	volumes := []medusaVolume{}
	if medusaSpec.CertificatesSecretRef.Name != "" {
		// default but deprecated, has priority to ensure backwards compatibility
		volumes = append(volumes, newVolumeFromCertificateSecret(dcConfig, medusaSpec))
	} else if medusaSpec.ClientEncryptionStores != nil {
		volumes = append(volumes, newVolumesFromClientEncryptionStores(dcConfig, medusaSpec)...)
	}
	return volumes
}

func newVolumeFromCertificateSecret(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate) medusaVolume {
	// the CertificateSecretRef only allows one secret reference, the individual field keys are assumed
	return newMedusaVolume(dcConfig, "certificates", medusaSpec.CertificatesSecretRef.Name)
}

func newVolumesFromClientEncryptionStores(dcConfig *cassandra.DatacenterConfig, medusaSpec *api.MedusaClusterTemplate) []medusaVolume {
	// with ClientEncryptionStores we can have each component reside in a different secret (and use a configurable key inside that secret)
	volumes := []medusaVolume{}
	if medusaSpec.ClientEncryptionStores.KeystoreSecretRef != nil {
		volumes = append(volumes, newMedusaVolume(dcConfig, "certfile", medusaSpec.ClientEncryptionStores.KeystoreSecretRef.Name))
	}
	if medusaSpec.ClientEncryptionStores.TruststoreSecretRef != nil {
		volumes = append(volumes, newMedusaVolume(dcConfig, "usercert", medusaSpec.ClientEncryptionStores.TruststoreSecretRef.Name))
	}
	if medusaSpec.ClientEncryptionStores.TruststorePasswordSecretRef != nil {
		volumes = append(volumes, newMedusaVolume(dcConfig, "userkey", medusaSpec.ClientEncryptionStores.TruststorePasswordSecretRef.Name))
	}
	return volumes
}

func newMedusaVolume(dcConfig *cassandra.DatacenterConfig, volumeName, secretName string) medusaVolume {
	encryptionClientVolumeIndex, found := cassandra.FindVolume(&dcConfig.PodTemplateSpec, volumeName)
	encryptionClientVolume := &corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
	return medusaVolume{
		Volume:      encryptionClientVolume,
		VolumeIndex: encryptionClientVolumeIndex,
		Exists:      found,
	}
}

func MedusaPurgeScheduleName(clusterName string, dcName string) string {
	return fmt.Sprintf("%s-%s-medusa-purge", clusterName, dcName)
}

func generateMedusaProbe(configuredProbe *corev1.Probe, grpcPort int, encryption *api.GRPCEncryption) (*corev1.Probe, error) {
	// Goalesce the custom probe with the default probe,
	defaultProbe := defaultMedusaProbe(grpcPort, encryption)
	if configuredProbe == nil {
		return defaultProbe, nil
	}
	mergedProbe := goalesce.MustDeepMerge(*defaultProbe, *configuredProbe)
	if !reflect.DeepEqual(defaultProbe.ProbeHandler, mergedProbe.ProbeHandler) {
		// If the user has configured a custom probe, use it
		// Otherwise, use the default probe
		return nil, fmt.Errorf("invalid probe configuration. You should not modify the probe handler")
	}

	return &mergedProbe, nil
}

func defaultMedusaProbe(grpcPort int, encryption *api.GRPCEncryption) *corev1.Probe {
	// Build the command for grpc_health_probe
	command := []string{"/bin/grpc_health_probe", fmt.Sprintf("--addr=:%d", grpcPort)}

	// Add TLS flags if encryption is configured
	if encryption != nil {
		command = append(command,
			"--tls",
			"--tls-ca-cert=/etc/certificates/grpc-server-certs/ca.crt",
			"--tls-client-cert=/etc/certificates/grpc-server-certs/tls.crt",
			"--tls-client-key=/etc/certificates/grpc-server-certs/tls.key",
		)
	}

	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: command,
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
