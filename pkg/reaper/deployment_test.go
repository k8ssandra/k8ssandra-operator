package reaper

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassimages "github.com/k8ssandra/cass-operator/pkg/images"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestNewDeployment(t *testing.T) {
	mainImage := &images.Image{Repository: "test", Name: "reaper", Tag: "latest", PullPolicy: corev1.PullAlways}
	reaper := newTestReaper()
	reaper.Spec.ContainerImage = mainImage
	reaper.Spec.AutoScheduling = reaperapi.AutoScheduling{Enabled: false}
	reaper.Spec.ServiceAccountName = "reaper"
	reaper.Spec.DatacenterAvailability = DatacenterAvailabilityAll
	reaper.Spec.HttpManagement.Enabled = true
	reaper.Spec.ClientEncryptionStores = &encryption.Stores{
		KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "keystore-secret",
		}},
		TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "truststore-secret",
		}},
	}
	reaper.Spec.ResourceMeta = &meta.ResourceMeta{
		CommonLabels:      map[string]string{"common": "everywhere", "override": "commonLevel"},
		CommonAnnotations: map[string]string{"common-annotation": "common-value", "annotation-override": "commonLevel"},
		Pods: meta.Tags{
			Labels:      map[string]string{"pod-label": "pod-label-value", "override": "podLevel"},
			Annotations: map[string]string{"pod-annotation": "pod-annotation-value", "annotation-override": "podLevel"},
		},
		Service: meta.Tags{
			Labels:      map[string]string{"service-label": "service-label-value"},
			Annotations: map[string]string{"service-annotation": "service-annotation-value"},
		},
	}

	labels := createServiceAndDeploymentLabels(reaper)
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), ptr.To("keystore-password"), ptr.To("truststore-password"), logger, getTestImageRegistry(t))

	assert.Equal(t, reaper.Namespace, deployment.Namespace)
	assert.Equal(t, reaper.Name, deployment.Name)
	assert.Equal(t, labels, deployment.Labels)
	assert.Equal(t, reaper.Spec.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)

	selector := deployment.Spec.Selector
	assert.Len(t, selector.MatchLabels, 0)
	assert.ElementsMatch(t, selector.MatchExpressions, []metav1.LabelSelectorRequirement{
		{
			Key:      k8ssandraapi.ManagedByLabel,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{k8ssandraapi.NameLabelValue},
		},
		{
			Key:      reaperapi.ReaperLabel,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{reaper.Name},
		},
	})

	// Verify labels and annotations are there. PodLevel is always overriding the common ones
	assert.Contains(t, deployment.Spec.Template.Labels, "pod-label")
	assert.Equal(t, "pod-label-value", deployment.Spec.Template.Labels["pod-label"])
	assert.Contains(t, deployment.Spec.Template.Labels, "common")
	assert.Equal(t, "everywhere", deployment.Spec.Template.Labels["common"])

	assert.Contains(t, deployment.Spec.Template.Labels, "override")
	assert.Equal(t, "podLevel", deployment.Spec.Template.Labels["override"])

	assert.Contains(t, deployment.Annotations, "common-annotation")
	assert.Equal(t, "common-value", deployment.Annotations["common-annotation"])
	assert.Contains(t, deployment.Annotations, k8ssandraapi.ResourceHashAnnotation)
	assert.NotEmpty(t, deployment.Annotations[k8ssandraapi.ResourceHashAnnotation])

	assert.Contains(t, deployment.Spec.Template.Annotations, "pod-annotation")
	assert.Equal(t, "pod-annotation-value", deployment.Spec.Template.Annotations["pod-annotation"])
	assert.Contains(t, deployment.Spec.Template.Annotations, "common-annotation")
	assert.Equal(t, "common-value", deployment.Spec.Template.Annotations["common-annotation"])
	assert.Contains(t, deployment.Spec.Template.Annotations, "annotation-override")
	assert.Equal(t, "podLevel", deployment.Spec.Template.Annotations["annotation-override"])

	podSpec := deployment.Spec.Template.Spec
	assert.Len(t, podSpec.Containers, 1)

	container := podSpec.Containers[0]

	assert.Equal(t, "docker.io/test/reaper:latest", container.Image)
	assert.Equal(t, corev1.PullAlways, container.ImagePullPolicy)

	// Verify temp directory volume
	t.Log("check that temp directory volume is properly configured")
	var tempDirVolume *corev1.Volume
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == "temp-dir" {
			tempDirVolume = &volume
			break
		}
	}
	assert.NotNil(t, tempDirVolume, "temp-dir volume not found")
	assert.NotNil(t, tempDirVolume.EmptyDir, "temp-dir volume is not of type EmptyDir")

	// Verify temp directory volume mount in main container
	var tempDirMount *corev1.VolumeMount
	for _, mount := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if mount.Name == "temp-dir" {
			tempDirMount = &mount
			break
		}
	}
	assert.NotNil(t, tempDirMount, "temp-dir volume mount not found in main container")
	assert.Equal(t, "/tmp", tempDirMount.MountPath, "temp-dir volume mount path is not /tmp")

	// Verify temp directory volume mount in init container
	tempDirMount = nil
	for _, mount := range deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts {
		if mount.Name == "temp-dir" {
			tempDirMount = &mount
			break
		}
	}
	assert.NotNil(t, tempDirMount, "temp-dir volume mount not found in init container")
	assert.Equal(t, "/tmp", tempDirMount.MountPath, "temp-dir volume mount path is not /tmp")

	assert.ElementsMatch(t, container.Env, []corev1.EnvVar{
		{
			Name:  "REAPER_STORAGE_TYPE",
			Value: "cassandra",
		},
		{
			Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
			Value: "false",
		},
		{
			Name:  "REAPER_CASS_CONTACT_POINTS",
			Value: "[{\"host\": \"cluster1-dc1-service\", \"port\": 9042}]",
		},
		{
			Name:  "REAPER_SKIP_SCHEMA_MIGRATION",
			Value: "true",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: DatacenterAvailabilityAll,
		},
		{
			Name:  "REAPER_CASS_LOCAL_DC",
			Value: "dc1",
		},
		{
			Name:  "REAPER_CASS_KEYSPACE",
			Value: "reaper_db",
		},
		{
			Name:  "JAVA_OPTS",
			Value: "-Djavax.net.ssl.keyStore=/mnt/client-keystore/keystore -Djavax.net.ssl.keyStorePassword=keystore-password -Djavax.net.ssl.trustStore=/mnt/client-truststore/truststore -Djavax.net.ssl.trustStorePassword=truststore-password -Dssl.enable=true -Ddatastax-java-driver.advanced.ssl-engine-factory.class=com.datastax.oss.driver.internal.core.ssl.DefaultSslEngineFactory -Ddatastax-java-driver.advanced.ssl-engine-factory.hostname-validation=false",
		},
		{
			Name:  "REAPER_CASS_NATIVE_PROTOCOL_SSL_ENCRYPTION_ENABLED",
			Value: "true",
		},
		{
			Name:  "REAPER_HTTP_MANAGEMENT_ENABLE",
			Value: "true",
		},
	})

	assert.Len(t, podSpec.InitContainers, 1)

	initContainer := podSpec.InitContainers[0]
	assert.Equal(t, "docker.io/test/reaper:latest", initContainer.Image)
	assert.Equal(t, corev1.PullAlways, initContainer.ImagePullPolicy)
	assert.ElementsMatch(t, initContainer.Env, []corev1.EnvVar{
		{
			Name:  "REAPER_STORAGE_TYPE",
			Value: "cassandra",
		},
		{
			Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
			Value: "false",
		},
		{
			Name:  "REAPER_CASS_CONTACT_POINTS",
			Value: "[{\"host\": \"cluster1-dc1-service\", \"port\": 9042}]",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: DatacenterAvailabilityAll,
		},
		{
			Name:  "REAPER_CASS_LOCAL_DC",
			Value: "dc1",
		},
		{
			Name:  "REAPER_CASS_KEYSPACE",
			Value: "reaper_db",
		},
		{
			Name:  "JAVA_OPTS",
			Value: "-Djavax.net.ssl.keyStore=/mnt/client-keystore/keystore -Djavax.net.ssl.keyStorePassword=keystore-password -Djavax.net.ssl.trustStore=/mnt/client-truststore/truststore -Djavax.net.ssl.trustStorePassword=truststore-password -Dssl.enable=true -Ddatastax-java-driver.advanced.ssl-engine-factory.class=com.datastax.oss.driver.internal.core.ssl.DefaultSslEngineFactory -Ddatastax-java-driver.advanced.ssl-engine-factory.hostname-validation=false",
		},
		{
			Name:  "REAPER_CASS_NATIVE_PROTOCOL_SSL_ENCRYPTION_ENABLED",
			Value: "true",
		},
		{
			Name:  "REAPER_HTTP_MANAGEMENT_ENABLE",
			Value: "true",
		},
	})

	assert.ElementsMatch(t, initContainer.Args, []string{"schema-migration"})

	reaper.Spec.AutoScheduling = reaperapi.AutoScheduling{
		Enabled:                    false,
		InitialDelay:               "PT10S",
		PeriodBetweenPolls:         "PT5M",
		TimeBeforeFirstSchedule:    "PT10M",
		ScheduleSpreadPeriod:       "PT6H",
		RepairType:                 "AUTO",
		PercentUnrepairedThreshold: 30,
		ExcludedClusters:           []string{"a", "b"},
		ExcludedKeyspaces:          []string{"system.powers"},
	}

	reaper.Spec.Keyspace = "ks1"

	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Len(t, container.Env, 8)

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_CASS_KEYSPACE",
		Value: "ks1",
	})

	reaper.Spec.AutoScheduling.Enabled = true
	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Len(t, container.Env, 18)

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_ADAPTIVE",
		Value: "false",
	})

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_INCREMENTAL",
		Value: "true",
	})

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_PERCENT_UNREPAIRED_THRESHOLD",
		Value: "30",
	})

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_PERIOD_BETWEEN_POLLS",
		Value: "PT5M",
	})

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_TIME_BEFORE_FIRST_SCHEDULE",
		Value: "PT10M",
	})

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_INITIAL_DELAY_PERIOD",
		Value: "PT10S",
	})

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_CLUSTERS",
		Value: "[a, b]",
	})

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_AUTO_SCHEDULING_EXCLUDED_KEYSPACES",
		Value: "[system.powers]",
	})

	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthcheck",
				Port: intstr.FromInt(8081),
			},
		},
		InitialDelaySeconds: 45,
		PeriodSeconds:       15,
	}
	assert.Equal(t, probe, container.LivenessProbe)
	assert.Equal(t, probe, container.ReadinessProbe)
}

func TestNewStatefulSet(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.StorageType = reaperapi.StorageTypeLocal
	reaper.Spec.StorageConfig = newTestStorageConfig()

	logger := testlogr.NewTestLogger(t)

	sts := NewStatefulSet(reaper, newTestDatacenter(), logger, getTestImageRegistry(t))

	podSpec := sts.Spec.Template.Spec
	assert.Len(t, podSpec.Containers, 1)

	container := podSpec.Containers[0]

	assert.ElementsMatch(t, container.Env, []corev1.EnvVar{
		{
			Name:  "REAPER_STORAGE_TYPE",
			Value: "memory",
		},
		{
			Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
			Value: "false",
		},
		{
			Name:  "REAPER_CASS_CONTACT_POINTS",
			Value: "[{\"host\": \"cluster1-dc1-service\", \"port\": 9042}]",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: "",
		},
		{
			Name:  "REAPER_SKIP_SCHEMA_MIGRATION",
			Value: "true",
		},
		{
			Name:  "REAPER_CASS_LOCAL_DC",
			Value: "dc1",
		},
		{
			Name:  "REAPER_CASS_KEYSPACE",
			Value: "reaper_db",
		},
	})
}

func TestNewStatefulSetForControlPlane(t *testing.T) {
	reaper := newTestControlPlaneReaper()
	noDataCenter := &cassdcapi.CassandraDatacenter{}

	sts := NewStatefulSet(reaper, noDataCenter, testlogr.NewTestLogger(t), getTestImageRegistry(t))
	podSpec := sts.Spec.Template.Spec
	container := podSpec.Containers[0]
	assert.ElementsMatch(t, container.Env, []corev1.EnvVar{
		{
			Name:  "REAPER_STORAGE_TYPE",
			Value: "memory",
		},
		{
			Name:  "REAPER_ENABLE_DYNAMIC_SEED_LIST",
			Value: "false",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: "",
		},
		{
			Name:  "REAPER_SKIP_SCHEMA_MIGRATION",
			Value: "true",
		},
	})

}

func TestHttpManagementConfiguration(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.HttpManagement.Enabled = true
	reaper.Spec.HttpManagement.Keystores = &corev1.LocalObjectReference{Name: "test-dc1-c-mgmt-ks"}
	logger := testlogr.NewTestLogger(t)

	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))

	assert := assert.New(t)
	assert.Len(deployment.Spec.Template.Spec.Containers, 1)
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Contains(container.Env, corev1.EnvVar{
		Name:  "REAPER_HTTP_MANAGEMENT_KEYSTORE_PATH",
		Value: "/etc/encryption/mgmt/keystore.jks",
	})
	assert.Contains(container.Env, corev1.EnvVar{
		Name:  "REAPER_HTTP_MANAGEMENT_TRUSTSTORE_PATH",
		Value: "/etc/encryption/mgmt/truststore.jks",
	})

	assert.Contains(container.VolumeMounts, corev1.VolumeMount{
		Name:      "management-api-keystore",
		MountPath: "/etc/encryption/mgmt",
	})

	assert.Contains(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "management-api-keystore",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: reaper.Spec.HttpManagement.Keystores.Name,
			},
		},
	})
}

func TestReadinessProbe(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/custom",
				Port: intstr.FromInt(8080),
			},
		},
		InitialDelaySeconds: 123,
	}
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	expected := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthcheck",
				Port: intstr.FromInt(8081),
			},
		},
		InitialDelaySeconds: 123,
	}
	assert.Equal(t, expected, deployment.Spec.Template.Spec.Containers[0].ReadinessProbe)
}

func TestLivenessProbe(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/custom",
				Port: intstr.FromInt(8080),
			},
		},
		InitialDelaySeconds: 123,
	}
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	expected := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthcheck",
				Port: intstr.FromInt(8081),
			},
		},
		InitialDelaySeconds: 123,
	}
	assert.Equal(t, expected, deployment.Spec.Template.Spec.Containers[0].LivenessProbe)
}

func TestImages(t *testing.T) {
	// Note: nil images are normally not possible due to the kubebuilder markers on the CRD spec
	t.Run("nil images", func(t *testing.T) {
		reaper := newTestReaper()
		reaper.Spec.ContainerImage = nil
		logger := testlogr.NewTestLogger(t)
		deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:4.0.1", deployment.Spec.Template.Spec.InitContainers[0].Image)
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:4.0.1", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("default images", func(t *testing.T) {
		reaper := newTestReaper()
		reaper.Spec.ContainerImage = nil
		logger := testlogr.NewTestLogger(t)
		deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:4.0.1", deployment.Spec.Template.Spec.InitContainers[0].Image)
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:4.0.1", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("custom images", func(t *testing.T) {
		reaper := newTestReaper()
		image := &images.Image{
			Repository:    "my-custom-repo",
			Name:          "my-custom-name",
			Tag:           "latest",
			PullSecretRef: &corev1.LocalObjectReference{Name: "my-secret"},
		}
		reaper.Spec.ContainerImage = image
		logger := testlogr.NewTestLogger(t)
		deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
		assert.Equal(t, "docker.io/my-custom-repo/my-custom-name:latest", deployment.Spec.Template.Spec.InitContainers[0].Image)
		assert.Equal(t, "docker.io/my-custom-repo/my-custom-name:latest", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Contains(t, deployment.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: "my-secret"})
		assert.Len(t, deployment.Spec.Template.Spec.ImagePullSecrets, 1)
	})
}

func TestTolerations(t *testing.T) {
	tolerations := []corev1.Toleration{
		{
			Key:      "key1",
			Operator: corev1.TolerationOpEqual,
			Value:    "value1",
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "key2",
			Operator: corev1.TolerationOpEqual,
			Value:    "value2",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	reaper := newTestReaper()
	reaper.Spec.Tolerations = tolerations
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	assert.ElementsMatch(t, tolerations, deployment.Spec.Template.Spec.Tolerations)
}

func TestAffinity(t *testing.T) {
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/e2e-az-name",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"e2e-az1", "e2e-az2"},
							},
						},
					},
				},
			},
		},
	}
	reaper := newTestReaper()
	reaper.Spec.Affinity = affinity
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	assert.EqualValues(t, affinity, deployment.Spec.Template.Spec.Affinity, "affinity does not match")
}

func TestContainerSecurityContext(t *testing.T) {
	reaper := newTestReaper()
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, testlogr.NewTestLogger(t), getTestImageRegistry(t))

	// Test default security context
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.NotNil(t, container.SecurityContext)
	assert.True(t, *container.SecurityContext.RunAsNonRoot)
	assert.Equal(t, int64(1000), *container.SecurityContext.RunAsUser)
	assert.True(t, *container.SecurityContext.ReadOnlyRootFilesystem)
	assert.False(t, *container.SecurityContext.AllowPrivilegeEscalation)
	assert.NotNil(t, container.SecurityContext.Capabilities)
	assert.Equal(t, []corev1.Capability{"ALL"}, container.SecurityContext.Capabilities.Drop)

	// Test custom security context
	customSecurityContext := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To[int64](2000),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
	reaper.Spec.SecurityContext = customSecurityContext
	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, testlogr.NewTestLogger(t), getTestImageRegistry(t))
	container = deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, customSecurityContext, container.SecurityContext)
}

func TestSchemaInitContainerSecurityContext(t *testing.T) {
	reaper := newTestReaper()
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, testlogr.NewTestLogger(t), getTestImageRegistry(t))

	// Test default security context
	initContainer := deployment.Spec.Template.Spec.InitContainers[0]
	assert.NotNil(t, initContainer.SecurityContext)
	assert.True(t, *initContainer.SecurityContext.RunAsNonRoot)
	assert.Equal(t, int64(1000), *initContainer.SecurityContext.RunAsUser)
	assert.True(t, *initContainer.SecurityContext.ReadOnlyRootFilesystem)
	assert.False(t, *initContainer.SecurityContext.AllowPrivilegeEscalation)
	assert.NotNil(t, initContainer.SecurityContext.Capabilities)
	assert.Equal(t, []corev1.Capability{"ALL"}, initContainer.SecurityContext.Capabilities.Drop)

	// Test custom security context
	customSecurityContext := &corev1.SecurityContext{
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To[int64](2000),
		ReadOnlyRootFilesystem:   ptr.To(true),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
	reaper.Spec.InitContainerSecurityContext = customSecurityContext
	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, testlogr.NewTestLogger(t), getTestImageRegistry(t))
	initContainer = deployment.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, customSecurityContext, initContainer.SecurityContext)
}

func TestPodSecurityContext(t *testing.T) {
	reaper := newTestReaper()
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, testlogr.NewTestLogger(t), getTestImageRegistry(t))

	// Test default security context
	podSpec := deployment.Spec.Template.Spec
	assert.NotNil(t, podSpec.SecurityContext)
	assert.True(t, *podSpec.SecurityContext.RunAsNonRoot)
	assert.Equal(t, int64(1000), *podSpec.SecurityContext.FSGroup)

	// Test custom security context
	customPodSecurityContext := &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		FSGroup:      ptr.To[int64](2000),
	}
	reaper.Spec.PodSecurityContext = customPodSecurityContext
	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, testlogr.NewTestLogger(t), getTestImageRegistry(t))
	podSpec = deployment.Spec.Template.Spec
	assert.Equal(t, customPodSecurityContext, podSpec.SecurityContext)
}

func TestSkipSchemaMigration(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.SkipSchemaMigration = true
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 0, "expected pod template to not have any init container")
}

func TestDeploymentTypes(t *testing.T) {
	logger := testlogr.NewTestLogger(t)

	// reaper with cassandra backend becomes a deployment
	reaper := newTestReaper()
	reaper.Spec.ReaperTemplate.StorageType = reaperapi.StorageTypeCassandra
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, reaperapi.StorageTypeCassandra, deployment.Spec.Template.Spec.Containers[0].Env[0].Value)

	// asking for a deployment with memory backend does not work
	reaper = newTestReaper()
	reaper.Spec.ReaperTemplate.StorageType = reaperapi.StorageTypeLocal
	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	assert.Nil(t, deployment)

	// reaper with memory backend becomes a stateful set
	reaper = newTestReaper()
	reaper.Spec.ReaperTemplate.StorageType = reaperapi.StorageTypeLocal
	reaper.Spec.ReaperTemplate.StorageConfig = &corev1.PersistentVolumeClaimSpec{
		StorageClassName: func() *string { s := "test"; return &s }(),
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
	}
	sts := NewStatefulSet(reaper, newTestDatacenter(), logger, getTestImageRegistry(t))
	assert.Len(t, sts.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, "memory", sts.Spec.Template.Spec.Containers[0].Env[0].Value)

	// asking for a stateful set with cassandra backend does not work
	reaper = newTestReaper()
	reaper.Spec.ReaperTemplate.StorageType = reaperapi.StorageTypeCassandra
	sts = NewStatefulSet(reaper, newTestDatacenter(), logger, getTestImageRegistry(t))
	assert.Nil(t, sts)
}

func TestMakeActualDeploymentType(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.StorageType = reaperapi.StorageTypeCassandra
	d, err := MakeActualDeploymentType(reaper)
	assert.Nil(t, err)
	assert.IsType(t, &appsv1.Deployment{}, d)

	reaper.Spec.StorageType = reaperapi.StorageTypeLocal
	sts, err := MakeActualDeploymentType(reaper)
	assert.Nil(t, err)
	assert.IsType(t, &appsv1.StatefulSet{}, sts)

	reaper.Spec.StorageType = "invalid"
	d, err = MakeActualDeploymentType(reaper)
	assert.NotNil(t, err)
	assert.Nil(t, d)
}

func TestMakeDesiredDeploymentType(t *testing.T) {
	reaper := newTestReaper()
	fakeDc := newTestDatacenter()
	logger := testlogr.NewTestLogger(t)

	reaper.Spec.StorageType = reaperapi.StorageTypeCassandra
	d, err := MakeDesiredDeploymentType(reaper, fakeDc, nil, nil, logger, getTestImageRegistry(t))
	assert.Nil(t, err)
	assert.IsType(t, &appsv1.Deployment{}, d)

	reaper.Spec.StorageType = reaperapi.StorageTypeLocal
	reaper.Spec.StorageConfig = newTestStorageConfig()
	sts, err := MakeDesiredDeploymentType(reaper, fakeDc, nil, nil, logger, getTestImageRegistry(t))
	assert.Nil(t, err)
	assert.IsType(t, &appsv1.StatefulSet{}, sts)

	reaper.Spec.StorageType = "invalid"
	d, err = MakeDesiredDeploymentType(reaper, fakeDc, nil, nil, logger, getTestImageRegistry(t))
	assert.NotNil(t, err)
	assert.Nil(t, d)
}

func TestDeepCopyActualDeployment(t *testing.T) {
	reaper := newTestReaper()

	reaper.Spec.StorageType = reaperapi.StorageTypeCassandra
	deployment, err := MakeDesiredDeploymentType(reaper, newTestDatacenter(), nil, nil, testlogr.NewTestLogger(t), getTestImageRegistry(t))
	assert.Nil(t, err)
	deepCopy, err := DeepCopyActualDeployment(deployment)
	assert.Nil(t, err)
	assert.Equal(t, deployment, deepCopy)

	wrongDeployment := &appsv1.DaemonSet{}
	deepCopy, err = DeepCopyActualDeployment(wrongDeployment)
	assert.NotNil(t, err)
	assert.Nil(t, deepCopy)
}

func TestEnsureSingleReplica(t *testing.T) {
	reaper := newTestReaper()
	logger := testlogr.NewTestLogger(t)
	oneReplica := int32(1)
	twoReplicas := int32(2)

	// deployment size is not touched on Deployments
	actualDeployment, _ := MakeActualDeploymentType(reaper)
	desiredDeployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	desiredDeployment.Spec.Replicas = func() *int32 { i := int32(2); return &i }()
	err := EnsureSingleReplica(reaper, actualDeployment, desiredDeployment, logger)
	assert.Nil(t, err)
	assert.Equal(t, int32(2), *desiredDeployment.Spec.Replicas)

	// deployment size greater than 1 is not allowed on Stateful Sets
	reaper.Spec.StorageType = reaperapi.StorageTypeLocal
	reaper.Spec.StorageConfig = newTestStorageConfig()
	actualStatefulSet, _ := MakeActualDeploymentType(reaper)
	err = setDeploymentReplicas(&actualStatefulSet, &oneReplica)
	assert.Nil(t, err)
	desiredStatefulSet := NewStatefulSet(reaper, newTestDatacenter(), logger, getTestImageRegistry(t))
	desiredStatefulSet.Spec.Replicas = &twoReplicas
	err = EnsureSingleReplica(reaper, actualStatefulSet, desiredStatefulSet, logger)
	// errors out because we desire 2 replicas
	assert.Nil(t, err)

	// if we find a STS with more than 1 replicas, we forcefully scale it down
	actualStatefulSet, _ = MakeActualDeploymentType(reaper)
	err = setDeploymentReplicas(&actualStatefulSet, &twoReplicas)
	assert.Nil(t, err)
	desiredStatefulSetObject, _ := MakeDesiredDeploymentType(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))
	err = setDeploymentReplicas(&desiredStatefulSetObject, &oneReplica)
	assert.Nil(t, err)

	assert.Equal(t, twoReplicas, getDeploymentReplicas(desiredDeployment, logger))
	err = EnsureSingleReplica(reaper, actualStatefulSet, desiredStatefulSetObject, logger)
	assert.Nil(t, err)
	assert.Equal(t, oneReplica, getDeploymentReplicas(desiredStatefulSetObject, logger))
}

func newTestReaper() *reaperapi.Reaper {
	namespace := "service-test"
	reaperName := "test-reaper"
	dcName := "dc1"
	return &reaperapi.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      reaperName,
		},
		Spec: reaperapi.ReaperSpec{
			DatacenterRef: reaperapi.CassandraDatacenterRef{
				Name:      dcName,
				Namespace: namespace,
			},
			ReaperTemplate: reaperapi.ReaperTemplate{
				Keyspace:    "reaper_db",
				StorageType: "cassandra",
				ResourceMeta: &meta.ResourceMeta{
					CommonLabels:      map[string]string{"common": "everywhere", "override": "commonLevel"},
					CommonAnnotations: map[string]string{"common-annotation": "common-annotation-value"},
					Pods: meta.Tags{
						Labels:      map[string]string{"pod-label": "pod-label-value", "override": "podLevel"},
						Annotations: map[string]string{"pod-annotation": "pod-annotation-value"},
					},
					Service: meta.Tags{
						Labels:      map[string]string{"service-label": "service-label-value", "override": "serviceLevel"},
						Annotations: map[string]string{"service-annotation": "service-annotation-value"},
					},
				},
			},
		},
	}
}

func newTestControlPlaneReaper() *reaperapi.Reaper {
	namespace := "reaper-cp-test"
	reaperName := "cp-reaper"
	return &reaperapi.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      reaperName,
		},
		Spec: reaperapi.ReaperSpec{
			ReaperTemplate: reaperapi.ReaperTemplate{
				StorageType:   "local",
				StorageConfig: newTestStorageConfig(),
			},
		},
	}
}

func newTestDatacenter() *cassdcapi.CassandraDatacenter {
	namespace := "service-test"
	dcName := "dc1"
	clusterName := "cluster1"
	return &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dcName,
			Namespace: namespace,
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:   clusterName,
			ServerVersion: "4.0.1",
		},
	}
}

func newTestStorageConfig() *corev1.PersistentVolumeClaimSpec {
	return &corev1.PersistentVolumeClaimSpec{
		StorageClassName: func() *string { s := "test"; return &s }(),
		AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("1Gi"),
			},
		},
	}
}

func TestDefaultResources(t *testing.T) {
	reaper := newTestReaper()
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))

	// Init container resources
	assert.Equal(t, resource.MustParse(InitContainerMemRequest), *deployment.Spec.Template.Spec.InitContainers[0].Resources.Requests.Memory(), "expected init container memory request to be set")
	assert.Equal(t, resource.MustParse(InitContainerCpuRequest), *deployment.Spec.Template.Spec.InitContainers[0].Resources.Requests.Cpu(), "expected init container cpu request to be set")
	assert.Equal(t, resource.MustParse(InitContainerMemLimit), *deployment.Spec.Template.Spec.InitContainers[0].Resources.Limits.Memory(), "expected init container memory limit to be set")

	// Main container resources
	assert.Equal(t, resource.MustParse(MainContainerMemRequest), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory(), "expected main container memory request to be set")
	assert.Equal(t, resource.MustParse(MainContainerCpuRequest), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu(), "expected main container cpu request to be set")
	assert.Equal(t, resource.MustParse(MainContainerMemLimit), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory(), "expected main container memory limit to be set")
}

func TestCustomResources(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.Resources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("10Gi"),
			corev1.ResourceCPU:    resource.MustParse("4"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("20Gi"),
		},
	}

	reaper.Spec.InitContainerResources = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1Gi"),
			corev1.ResourceCPU:    resource.MustParse("2"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))

	// Init container resources
	assert.Equal(t, resource.MustParse("1Gi"), *deployment.Spec.Template.Spec.InitContainers[0].Resources.Requests.Memory(), "expected init container memory request to be set")
	assert.Equal(t, resource.MustParse("2"), *deployment.Spec.Template.Spec.InitContainers[0].Resources.Requests.Cpu(), "expected init container cpu request to be set")
	assert.Equal(t, resource.MustParse("2Gi"), *deployment.Spec.Template.Spec.InitContainers[0].Resources.Limits.Memory(), "expected init container memory limit to be set")

	// Main container resources
	assert.Equal(t, resource.MustParse("10Gi"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory(), "expected main container memory request to be set")
	assert.Equal(t, resource.MustParse("4"), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu(), "expected main container cpu request to be set")
	assert.Equal(t, resource.MustParse("20Gi"), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory(), "expected main container memory limit to be set")
}

func TestLabelsAnnotations(t *testing.T) {
	reaper := newTestReaper()
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger, getTestImageRegistry(t))

	deploymentLabels := map[string]string{
		k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		reaperapi.ReaperLabel:       reaper.Name,
		"common":                    "everywhere",
		"override":                  "commonLevel",
	}

	podLabels := map[string]string{
		k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		reaperapi.ReaperLabel:       reaper.Name,
		"common":                    "everywhere",
		"override":                  "podLevel", // Pod-level labels override CommonLabels
		"pod-label":                 "pod-label-value",
	}

	assert.Equal(t, deploymentLabels, deployment.Labels)
	assert.Equal(t, podLabels, deployment.Spec.Template.Labels)

	assert.Contains(t, deployment.Annotations, "common-annotation")
	assert.Equal(t, "common-annotation-value", deployment.Annotations["common-annotation"])
	assert.Contains(t, deployment.Annotations, k8ssandraapi.ResourceHashAnnotation)
	assert.NotEmpty(t, deployment.Annotations[k8ssandraapi.ResourceHashAnnotation])

	assert.Contains(t, deployment.Spec.Template.Annotations, "pod-annotation")
	assert.Equal(t, "pod-annotation-value", deployment.Spec.Template.Annotations["pod-annotation"])
	assert.Contains(t, deployment.Spec.Template.Annotations, "common-annotation")
	assert.Equal(t, "common-annotation-value", deployment.Spec.Template.Annotations["common-annotation"])
}

func TestGetAdaptiveIncremental(t *testing.T) {
	reaper := &reaperapi.Reaper{}
	dc := &cassdcapi.CassandraDatacenter{}

	adaptive, incremental := getAdaptiveIncremental(reaper, dc.Spec.ServerVersion)
	assert.False(t, adaptive)
	assert.False(t, incremental)

	reaper.Spec.AutoScheduling.RepairType = "ADAPTIVE"
	adaptive, incremental = getAdaptiveIncremental(reaper, dc.Spec.ServerVersion)
	assert.True(t, adaptive)
	assert.False(t, incremental)

	reaper.Spec.AutoScheduling.RepairType = "INCREMENTAL"
	adaptive, incremental = getAdaptiveIncremental(reaper, dc.Spec.ServerVersion)
	assert.False(t, adaptive)
	assert.True(t, incremental)

	reaper.Spec.AutoScheduling.RepairType = "AUTO"
	adaptive, incremental = getAdaptiveIncremental(reaper, dc.Spec.ServerVersion)
	assert.True(t, adaptive)
	assert.False(t, incremental)

	dc.Spec.ServerVersion = "3.11.1"
	adaptive, incremental = getAdaptiveIncremental(reaper, dc.Spec.ServerVersion)
	assert.True(t, adaptive)
	assert.False(t, incremental)

	dc.Spec.ServerVersion = "4.0.0"
	adaptive, incremental = getAdaptiveIncremental(reaper, dc.Spec.ServerVersion)
	assert.False(t, adaptive)
	assert.True(t, incremental)
}

func TestComputeEnvVarsAdditionalEnvVars(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.AdditionalEnvVars = []corev1.EnvVar{
		{Name: "CUSTOM_VAR", Value: "custom_value"},
		{Name: "REAPER_STORAGE_TYPE", Value: "should_be_overridden"},
	}
	dc := newTestDatacenter()

	envVars := computeEnvVars(reaper, dc, getTestImageRegistry(t))

	// Test 1: Additional env var can be added
	found := false
	for _, env := range envVars {
		if env.Name == "CUSTOM_VAR" && env.Value == "custom_value" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// Test 2: Static env var takes precedence over additional one
	found = false
	for _, env := range envVars {
		if env.Name == "REAPER_STORAGE_TYPE" && env.Value == "cassandra" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestIsReaperPostV4(t *testing.T) {
	tests := []struct {
		name     string
		reaper   *reaperapi.Reaper
		expected bool
	}{
		{
			name: "v3 tag",
			reaper: &reaperapi.Reaper{
				Spec: reaperapi.ReaperSpec{
					ReaperTemplate: reaperapi.ReaperTemplate{
						ContainerImage: &images.Image{
							Tag: "3.2.0",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "v4 tag",
			reaper: &reaperapi.Reaper{
				Spec: reaperapi.ReaperSpec{
					ReaperTemplate: reaperapi.ReaperTemplate{
						ContainerImage: &images.Image{
							Tag: "4.0.1",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "v5 tag",
			reaper: &reaperapi.Reaper{
				Spec: reaperapi.ReaperSpec{
					ReaperTemplate: reaperapi.ReaperTemplate{
						ContainerImage: &images.Image{
							Tag: "5.0.0",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "no container image specified",
			reaper: &reaperapi.Reaper{
				Spec: reaperapi.ReaperSpec{},
			},
			expected: true,
		},
		{
			name: "no tag specified",
			reaper: &reaperapi.Reaper{
				Spec: reaperapi.ReaperSpec{
					ReaperTemplate: reaperapi.ReaperTemplate{
						ContainerImage: &images.Image{},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReaperPostV4(tt.reaper, getTestImageRegistry(t))
			if result != tt.expected {
				t.Errorf("isReaperPostV4() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReaperV4ContactPointsFormat(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.ContainerImage = &images.Image{
		Repository: "test",
		Name:       "reaper",
		Tag:        "4.0.1",
		PullPolicy: corev1.PullAlways,
	}
	dc := newTestDatacenter()
	logger := testlogr.NewTestLogger(t)

	deployment := NewDeployment(reaper, dc, nil, nil, logger, getTestImageRegistry(t))
	container := deployment.Spec.Template.Spec.Containers[0]

	// Find the REAPER_CASS_CONTACT_POINTS env var
	var contactPointsEnvVar *corev1.EnvVar
	for _, env := range container.Env {
		if env.Name == "REAPER_CASS_CONTACT_POINTS" {
			contactPointsEnvVar = &env
			break
		}
	}

	assert.NotNil(t, contactPointsEnvVar, "REAPER_CASS_CONTACT_POINTS environment variable not found")
	assert.Equal(t, "[{\"host\": \"cluster1-dc1-service\", \"port\": 9042}]", contactPointsEnvVar.Value,
		"REAPER_CASS_CONTACT_POINTS should be formatted as a JSON array with host and port for Reaper v4")
}

func TestDefaultReaperContactPointsFormat(t *testing.T) {
	reaper := newTestReaper()
	dc := newTestDatacenter()
	logger := testlogr.NewTestLogger(t)

	deployment := NewDeployment(reaper, dc, nil, nil, logger, getTestImageRegistry(t))
	container := deployment.Spec.Template.Spec.Containers[0]

	// Find the REAPER_CASS_CONTACT_POINTS env var
	var contactPointsEnvVar *corev1.EnvVar
	for _, env := range container.Env {
		if env.Name == "REAPER_CASS_CONTACT_POINTS" {
			contactPointsEnvVar = &env
			break
		}
	}

	assert.NotNil(t, contactPointsEnvVar, "REAPER_CASS_CONTACT_POINTS environment variable not found")
	assert.Equal(t, "[{\"host\": \"cluster1-dc1-service\", \"port\": 9042}]", contactPointsEnvVar.Value,
		"REAPER_CASS_CONTACT_POINTS should be formatted as an array of hosts for Reaper v3 and below")
}

var (
	regOnce           sync.Once
	imageRegistryTest cassimages.ImageRegistry
)

func getTestImageRegistry(t testing.TB) cassimages.ImageRegistry {
	regOnce.Do(func() {
		// Path from pkg/reaper to testdata
		p := filepath.Clean("../../test/testdata/imageconfig/image_config_test.yaml")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("failed reading test image config: %v", err)
		}
		r, err := cassimages.NewImageRegistryV2(data)
		if err != nil {
			t.Fatalf("failed parsing test image config: %v", err)
		}
		imageRegistryTest = r
	})
	return imageRegistryTest
}
