package reaper

import (
	"fmt"
	"math"
	"strings"

	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	goalesceutils "github.com/k8ssandra/k8ssandra-operator/pkg/goalesce"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultImageRepository = "thelastpickle"
	DefaultImageName       = "cassandra-reaper"
	DefaultVersion         = "3.8.0"
	// When changing the default version above, please also change the kubebuilder markers in
	// apis/reaper/v1alpha1/reaper_types.go accordingly.

	InitContainerMemRequest = "128Mi"
	InitContainerMemLimit   = "512Mi"
	InitContainerCpuRequest = "100m"
	MainContainerMemRequest = "256Mi"
	MainContainerMemLimit   = "3Gi"
	MainContainerCpuRequest = "100m"
)

var defaultImage = images.Image{
	Repository: DefaultImageRepository,
	Name:       DefaultImageName,
	Tag:        DefaultVersion,
}

func computeEnvVars(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter) []corev1.EnvVar {
	var storageType string
	if reaper.Spec.StorageType == api.StorageTypeLocal {
		storageType = "memory"
	} else {
		storageType = "cassandra"
	}
	envVars := []corev1.EnvVar{
		{
			Name:  "REAPER_STORAGE_TYPE",
			Value: storageType,
		},
		{
			Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
			Value: "false",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: reaper.Spec.DatacenterAvailability,
		},
	}

	// env vars used to interact with Cassandra cluster used for storage (not the one to repair) are only needed
	// when we actually have a cass-dc available
	if dc.DatacenterName() != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_CASS_LOCAL_DC",
			Value: dc.DatacenterName(),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_CASS_KEYSPACE",
			Value: reaper.Spec.Keyspace,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_CASS_CONTACT_POINTS",
			Value: fmt.Sprintf("[%s]", dc.GetDatacenterServiceName()),
		})
	}

	if reaper.Spec.AutoScheduling.Enabled {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_ENABLED",
			Value: "true",
		})
		serverVersion := ""
		if dc != nil && dc.Spec.ServerVersion != "" {
			serverVersion = dc.Spec.ServerVersion
		}
		adaptive, incremental := getAdaptiveIncremental(reaper, serverVersion)
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_ADAPTIVE",
			Value: fmt.Sprintf("%v", adaptive),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_INCREMENTAL",
			Value: fmt.Sprintf("%v", incremental),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_PERCENT_UNREPAIRED_THRESHOLD",
			Value: fmt.Sprintf("%v", reaper.Spec.AutoScheduling.PercentUnrepairedThreshold),
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_INITIAL_DELAY_PERIOD",
			Value: reaper.Spec.AutoScheduling.InitialDelay,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_PERIOD_BETWEEN_POLLS",
			Value: reaper.Spec.AutoScheduling.PeriodBetweenPolls,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_TIME_BEFORE_FIRST_SCHEDULE",
			Value: reaper.Spec.AutoScheduling.TimeBeforeFirstSchedule,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_SCHEDULE_SPREAD_PERIOD",
			Value: reaper.Spec.AutoScheduling.ScheduleSpreadPeriod,
		})
		if reaper.Spec.AutoScheduling.ExcludedClusters != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_CLUSTERS",
				Value: fmt.Sprintf("[%s]", strings.Join(reaper.Spec.AutoScheduling.ExcludedClusters, ", ")),
			})
		}
		if reaper.Spec.AutoScheduling.ExcludedKeyspaces != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_KEYSPACES",
				Value: fmt.Sprintf("[%s]", strings.Join(reaper.Spec.AutoScheduling.ExcludedKeyspaces, ", ")),
			})
		}
	}

	if reaper.Spec.SkipSchemaMigration {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_SKIP_SCHEMA_MIGRATION",
			Value: "true",
		})
	}

	if reaper.Spec.HeapSize != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_HEAP_SIZE",
			Value: fmt.Sprintf("%d", reaper.Spec.HeapSize.Value()),
		})
	}
	if reaper.Spec.HttpManagement.Enabled {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_HTTP_MANAGEMENT_ENABLE",
			Value: "true",
		})

		if reaper.Spec.HttpManagement.Keystores != nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_HTTP_MANAGEMENT_KEYSTORE_PATH",
				Value: "/etc/encryption/mgmt/keystore.jks",
			})
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REAPER_HTTP_MANAGEMENT_TRUSTSTORE_PATH",
				Value: "/etc/encryption/mgmt/truststore.jks",
			})
		}
	}

	return envVars
}

func computeVolumes(reaper *api.Reaper) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{
		{
			Name: "conf",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "temp-dir",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "conf",
			MountPath: "/etc/cassandra-reaper/config",
		},
		{
			Name:      "temp-dir",
			MountPath: "/tmp",
		},
	}

	if reaper.Spec.HttpManagement.Enabled && reaper.Spec.HttpManagement.Keystores != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "management-api-keystore",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: reaper.Spec.HttpManagement.Keystores.Name,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "management-api-keystore",
			MountPath: "/etc/encryption/mgmt",
		})
	}

	if reaper.Spec.StorageType == api.StorageTypeLocal {
		volumes = append(volumes, corev1.Volume{
			Name: "reaper-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("reaper-data-%s", reaper.Name),
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "reaper-data",
			MountPath: "/var/lib/cassandra-reaper/storage",
		})
	}

	return volumes, volumeMounts
}

func makeSelector(reaper *api.Reaper) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			// Note: managed-by shouldn't be used here, but we're keeping it for backwards compatibility, since changing
			// a deployment's selector is a breaking change.
			{
				Key:      v1alpha1.ManagedByLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{v1alpha1.NameLabelValue},
			},
			{
				Key:      api.ReaperLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{reaper.Name},
			},
		},
	}
}

func makeObjectMeta(reaper *api.Reaper) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:   reaper.Namespace,
		Name:        reaper.Name,
		Labels:      createServiceAndDeploymentLabels(reaper),
		Annotations: map[string]string{},
	}
}

func computePodMeta(reaper *api.Reaper) metav1.ObjectMeta {
	podMeta := getPodMeta(reaper)
	return metav1.ObjectMeta{
		Labels:      podMeta.Labels,
		Annotations: podMeta.Annotations,
	}
}

func configureClientEncryption(reaper *api.Reaper, envVars []corev1.EnvVar, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, keystorePassword *string, truststorePassword *string) ([]corev1.EnvVar, []corev1.Volume, []corev1.VolumeMount) {
	// if client encryption is turned on, we need to mount the keystore and truststore volumes
	// by client we mean a C* client, so this is only relevant if we are making a Deployment which uses C* as storage backend
	if reaper.Spec.ClientEncryptionStores != nil && keystorePassword != nil && truststorePassword != nil {
		keystoreVolume, truststoreVolume := cassandra.EncryptionVolumes(encryption.StoreTypeClient, *reaper.Spec.ClientEncryptionStores)
		volumes = append(volumes, *keystoreVolume)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      keystoreVolume.Name,
			MountPath: cassandra.StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameKeystore),
		})
		volumes = append(volumes, *truststoreVolume)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      truststoreVolume.Name,
			MountPath: cassandra.StoreMountFullPath(encryption.StoreTypeClient, encryption.StoreNameTruststore),
		})

		javaOpts := fmt.Sprintf("-Djavax.net.ssl.keyStore=/mnt/client-keystore/keystore -Djavax.net.ssl.keyStorePassword=%s -Djavax.net.ssl.trustStore=/mnt/client-truststore/truststore -Djavax.net.ssl.trustStorePassword=%s -Dssl.enable=true", *keystorePassword, *truststorePassword)
		envVars = append(envVars, corev1.EnvVar{
			Name:  "JAVA_OPTS",
			Value: javaOpts,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_CASS_NATIVE_PROTOCOL_SSL_ENCRYPTION_ENABLED",
			Value: "true",
		})
	}
	return envVars, volumes, volumeMounts
}

func computePodSpec(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter, initContainerResources *corev1.ResourceRequirements, keystorePassword *string, truststorePassword *string) corev1.PodSpec {
	envVars := computeEnvVars(reaper, dc)
	volumes, volumeMounts := computeVolumes(reaper)
	mainImage := reaper.Spec.ContainerImage.ApplyDefaults(defaultImage)
	mainContainerResources := computeMainContainerResources(reaper.Spec.Resources)

	if keystorePassword != nil && truststorePassword != nil {
		envVars, volumes, volumeMounts = configureClientEncryption(reaper, envVars, volumes, volumeMounts, keystorePassword, truststorePassword)
	}

	var initContainers []corev1.Container
	if initContainerResources != nil {
		initContainers = computeInitContainers(reaper, mainImage, envVars, volumeMounts, initContainerResources)
	} else {
		initContainers = nil
	}

	// Default security context settings
	defaultPodSecurityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		FSGroup:      ptr.To[int64](1000),
	}

	defaultContainerSecurityContext := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To[int64](1000),
		RunAsGroup:               ptr.To[int64](1000),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	// Use provided security contexts if specified, otherwise use defaults
	podSecurityContext := reaper.Spec.PodSecurityContext
	if podSecurityContext == nil {
		podSecurityContext = defaultPodSecurityContext
	}

	containerSecurityContext := reaper.Spec.SecurityContext
	if containerSecurityContext == nil {
		containerSecurityContext = defaultContainerSecurityContext
	}

	return corev1.PodSpec{
		Affinity:       reaper.Spec.Affinity,
		InitContainers: initContainers,
		Containers: []corev1.Container{
			{
				Name:            "reaper",
				Image:           mainImage.String(),
				ImagePullPolicy: mainImage.PullPolicy,
				SecurityContext: containerSecurityContext,
				Ports: []corev1.ContainerPort{
					{
						Name:          "app",
						ContainerPort: 8080,
						Protocol:      "TCP",
					},
					{
						Name:          "admin",
						ContainerPort: 8081,
						Protocol:      "TCP",
					},
				},
				ReadinessProbe: computeProbe(reaper.Spec.ReadinessProbe),
				LivenessProbe:  computeProbe(reaper.Spec.LivenessProbe),
				Env:            envVars,
				VolumeMounts:   volumeMounts,
				Resources:      *mainContainerResources,
			},
		},
		ServiceAccountName: reaper.Spec.ServiceAccountName,
		Tolerations:        reaper.Spec.Tolerations,
		SecurityContext:    podSecurityContext,
		ImagePullSecrets:   computeImagePullSecrets(reaper, mainImage),
		Volumes:            volumes,
	}
}

func computeVolumeClaims(reaper *api.Reaper) []corev1.PersistentVolumeClaim {

	vcs := make([]corev1.PersistentVolumeClaim, 0)

	volumeClaimsPec := reaper.Spec.StorageConfig.DeepCopy()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reaper-data",
			Namespace: reaper.Namespace,
		},
		Spec: *volumeClaimsPec,
	}
	vcs = append(vcs, *pvc)

	return vcs
}

func NewStatefulSet(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter, logger logr.Logger, _ *string, _ *string, authVars ...*corev1.EnvVar) *appsv1.StatefulSet {

	if reaper.Spec.ReaperTemplate.StorageType != api.StorageTypeLocal {
		logger.Error(fmt.Errorf("cannot be creating a Reaper statefulset with storage type other than Memory"), "bad storage type", "storageType", reaper.Spec.StorageType)
		return nil
	}

	if reaper.Spec.ReaperTemplate.StorageConfig == nil {
		logger.Error(fmt.Errorf("reaper spec needs storage config when using memory sotrage type"), "missing storage config")
		return nil
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: makeObjectMeta(reaper),
		Spec: appsv1.StatefulSetSpec{
			Selector: makeSelector(reaper),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: computePodMeta(reaper),
				Spec:       computePodSpec(reaper, dc, nil, nil, nil),
			},
			VolumeClaimTemplates: computeVolumeClaims(reaper),
			Replicas:             ptr.To[int32](1),
		},
	}
	addAuthEnvVars(&statefulSet.Spec.Template, authVars)
	configureVector(reaper, &statefulSet.Spec.Template, dc, logger)
	labels.AddCommonLabelsFromReaper(statefulSet, reaper)
	annotations.AddCommonAnnotationsFromReaper(statefulSet, reaper)

	// Merge the user-provided PodTemplateSpec as the last step
	statefulSet.Spec.Template = *goalesceutils.MergeCRs(reaper.Spec.PodTemplateSpec, &statefulSet.Spec.Template)

	annotations.AddHashAnnotation(statefulSet)
	return statefulSet
}

func NewDeployment(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter, keystorePassword *string, truststorePassword *string, logger logr.Logger, authVars ...*corev1.EnvVar) *appsv1.Deployment {

	if reaper.Spec.ReaperTemplate.StorageType != api.StorageTypeCassandra {
		logger.Error(fmt.Errorf("cannot be creating a Reaper deployment with storage type other than Cassandra"), "bad storage type", "storageType", reaper.Spec.ReaperTemplate.StorageType)
		return nil
	}

	initContainerResources := computeInitContainerResources(reaper.Spec.InitContainerResources)

	deployment := &appsv1.Deployment{
		ObjectMeta: makeObjectMeta(reaper),
		Spec: appsv1.DeploymentSpec{
			Selector: makeSelector(reaper),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: computePodMeta(reaper),
				Spec:       computePodSpec(reaper, dc, initContainerResources, keystorePassword, truststorePassword),
			},
		},
	}
	addAuthEnvVars(&deployment.Spec.Template, authVars)
	configureVector(reaper, &deployment.Spec.Template, dc, logger)

	labels.AddCommonLabelsFromReaper(deployment, reaper)
	annotations.AddCommonAnnotationsFromReaper(deployment, reaper)

	// Merge the user-provided PodTemplateSpec as the last step
	deployment.Spec.Template = *goalesceutils.MergeCRs(reaper.Spec.PodTemplateSpec, &deployment.Spec.Template)

	annotations.AddHashAnnotation(deployment)
	return deployment
}

func computeInitContainerResources(resourceRequirements *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	if resourceRequirements != nil {
		return resourceRequirements
	}

	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(InitContainerCpuRequest),
			corev1.ResourceMemory: resource.MustParse(InitContainerMemRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(InitContainerMemLimit),
		},
	}
}

func computeMainContainerResources(resourceRequirements *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	if resourceRequirements != nil {
		return resourceRequirements
	}

	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(MainContainerCpuRequest),
			corev1.ResourceMemory: resource.MustParse(MainContainerMemRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(MainContainerMemLimit),
		},
	}
}

func computeInitContainers(
	reaper *api.Reaper,
	mainImage *images.Image,
	envVars []corev1.EnvVar,
	volumeMounts []corev1.VolumeMount,
	resourceRequirements *corev1.ResourceRequirements) []corev1.Container {
	var initContainers []corev1.Container
	if !reaper.Spec.SkipSchemaMigration {
		// Default security context for init container
		defaultInitContainerSecurityContext := &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			RunAsUser:                ptr.To[int64](1000),
			ReadOnlyRootFilesystem:   ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		}

		// Use provided security context if specified, otherwise use default
		initContainerSecurityContext := reaper.Spec.InitContainerSecurityContext
		if initContainerSecurityContext == nil {
			initContainerSecurityContext = defaultInitContainerSecurityContext
		}

		initContainers = append(initContainers,
			corev1.Container{
				Name:            "reaper-schema-init",
				Image:           mainImage.String(),
				ImagePullPolicy: mainImage.PullPolicy,
				SecurityContext: initContainerSecurityContext,
				Env:             envVars,
				Args:            []string{"schema-migration"},
				VolumeMounts:    volumeMounts,
				Resources:       *resourceRequirements,
			})
	}
	return initContainers
}

func computeImagePullSecrets(reaper *api.Reaper, mainImage *images.Image) []corev1.LocalObjectReference {
	return images.CollectPullSecrets(mainImage)
}

func computeProbe(probeTemplate *corev1.Probe) *corev1.Probe {
	var probe *corev1.Probe
	if probeTemplate != nil {
		probe = probeTemplate.DeepCopy()
	} else {
		probe = &corev1.Probe{
			InitialDelaySeconds: 45,
			PeriodSeconds:       15,
		}
	}
	// The handler cannot be user-specified, so force it now
	probe.ProbeHandler = corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/healthcheck",
			Port: intstr.FromInt(8081),
		},
	}
	return probe
}

func addAuthEnvVars(template *corev1.PodTemplateSpec, vars []*corev1.EnvVar) {
	envVars := template.Spec.Containers[0].Env
	for _, v := range vars {
		envVars = append(envVars, *v)
	}
	template.Spec.Containers[0].Env = envVars
	if len(template.Spec.InitContainers) > 0 {
		initEnvVars := template.Spec.InitContainers[0].Env
		for _, v := range vars {
			initEnvVars = append(initEnvVars, *v)
		}
		template.Spec.InitContainers[0].Env = initEnvVars
	}
}

func getAdaptiveIncremental(reaper *api.Reaper, serverVersion string) (adaptive bool, incremental bool) {
	switch reaper.Spec.AutoScheduling.RepairType {
	case "ADAPTIVE":
		adaptive = true
	case "INCREMENTAL":
		incremental = true
	case "AUTO":
		if serverVersion == "" || semver.MustParse(serverVersion).Major() == 3 {
			adaptive = true
		} else {
			incremental = true
		}
	}
	return
}

func getPodMeta(reaper *api.Reaper) meta.Tags {
	labels := createPodLabels(reaper)

	var podAnnotations map[string]string
	if meta := reaper.Spec.ResourceMeta; meta != nil {
		podAnnotations = meta.Pods.Annotations

	}

	return meta.Tags{
		Labels:      labels,
		Annotations: podAnnotations,
	}
}

func MakeActualDeploymentType(actualReaper *api.Reaper) (client.Object, error) {
	if actualReaper.Spec.StorageType == api.StorageTypeCassandra {
		return &appsv1.Deployment{}, nil
	} else if actualReaper.Spec.StorageType == api.StorageTypeLocal {
		return &appsv1.StatefulSet{}, nil
	} else {
		err := fmt.Errorf("invalid storage type %s", actualReaper.Spec.StorageType)
		return nil, err
	}
}

func MakeDesiredDeploymentType(actualReaper *api.Reaper, dc *cassdcapi.CassandraDatacenter, keystorePassword *string, truststorePassword *string, logger logr.Logger, authVars ...*corev1.EnvVar) (client.Object, error) {
	if actualReaper.Spec.StorageType == api.StorageTypeCassandra {
		return NewDeployment(actualReaper, dc, keystorePassword, truststorePassword, logger, authVars...), nil
	} else if actualReaper.Spec.StorageType == api.StorageTypeLocal {
		// we're checking for this same thing in the k8ssandra-cluster webohooks
		// but in tests (and in the future) we'll be creating reaper directly (not through k8ssandra-cluster)
		// so we need to double-check
		if actualReaper.Spec.StorageConfig == nil {
			err := fmt.Errorf("storageConfig is required for memory storage")
			return nil, err
		}
		return NewStatefulSet(actualReaper, dc, logger, keystorePassword, truststorePassword, authVars...), nil
	} else {
		err := fmt.Errorf("invalid storage type %s", actualReaper.Spec.StorageType)
		return nil, err
	}
}

func DeepCopyActualDeployment(actualDeployment client.Object) (client.Object, error) {
	switch actual := actualDeployment.(type) {
	case *appsv1.Deployment:
		actualDeployment = actual.DeepCopy()
		return actualDeployment, nil
	case *appsv1.StatefulSet:
		actualDeployment = actual.DeepCopy()
		return actualDeployment, nil
	default:
		err := fmt.Errorf("unexpected type %T", actualDeployment)
		return nil, err
	}
}

func EnsureSingleReplica(actualReaper *api.Reaper, actualDeployment client.Object, desiredDeployment client.Object, logger logr.Logger) error {
	if actualReaper.Spec.StorageType == api.StorageTypeLocal {
		desiredReplicas := getDeploymentReplicas(desiredDeployment, logger)
		if desiredReplicas > 1 {
			logger.Info(fmt.Sprintf("reaper with memory storage can only have one replica, not allowing the desired %d", desiredReplicas))
			if err := setDeploymentReplicas(&desiredDeployment, ptr.To[int32](1)); err != nil {
				return err
			}
		}
		actualReplicas := getDeploymentReplicas(actualDeployment, logger)
		if actualReplicas > 1 {
			logger.Info(fmt.Sprintf("reaper with memory storage currently has %d replicas, scaling down to 1", actualReplicas))
			if err := setDeploymentReplicas(&desiredDeployment, ptr.To[int32](1)); err != nil {
				// returning error if the setter failed
				return err
			}
		}
	}
	return nil
}

func getDeploymentReplicas(actualDeployment client.Object, logger logr.Logger) int32 {
	switch actual := actualDeployment.(type) {
	case *appsv1.Deployment:
		return *actual.Spec.Replicas
	case *appsv1.StatefulSet:
		return *actual.Spec.Replicas
	default:
		logger.Error(fmt.Errorf("unexpected type %T", actualDeployment), "Failed to get deployment replicas")
		return math.MaxInt32
	}
}

func setDeploymentReplicas(desiredDeployment *client.Object, numberOfReplicas *int32) error {
	switch desired := (*desiredDeployment).(type) {
	case *appsv1.Deployment:
		desired.Spec.Replicas = numberOfReplicas
	case *appsv1.StatefulSet:
		desired.Spec.Replicas = numberOfReplicas
	default:
		err := fmt.Errorf("unexpected type %T", desiredDeployment)
		return err
	}
	return nil
}
