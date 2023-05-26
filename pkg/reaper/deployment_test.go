package reaper

import (
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	configapi "github.com/k8ssandra/cass-operator/apis/config/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/images"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func TestNewDeployment(t *testing.T) {
	mainImage := "test/reaper:latest"
	initImage := "test/reaper-init:1.2.3"
	reaper := newTestReaper()
	images.GetImageConfig().DefaultImages = &configapi.DefaultImages{}
	images.GetImageConfig().Images = &configapi.Images{}
	reaper.Spec.ContainerImage = mainImage
	reaper.Spec.InitContainerImage = initImage
	reaper.Spec.AutoScheduling = reaperapi.AutoScheduling{Enabled: false}
	reaper.Spec.ServiceAccountName = "reaper"
	reaper.Spec.DatacenterAvailability = DatacenterAvailabilityAll
	reaper.Spec.ClientEncryptionStores = &encryption.Stores{
		KeystoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "keystore-secret",
		}},
		TruststoreSecretRef: &encryption.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{
			Name: "truststore-secret",
		}},
	}
	reaper.Spec.ResourceMeta = &meta.ResourceMeta{
		CommonLabels: map[string]string{"common": "everywhere", "override": "commonLevel"},
		Pods: meta.Tags{
			Labels:      map[string]string{"pod-label": "pod-label-value", "override": "podLevel"},
			Annotations: map[string]string{"pod-annotation": "pod-annotation-value"},
		},
		Service: meta.Tags{
			Labels:      map[string]string{"service-label": "service-label-value"},
			Annotations: map[string]string{"service-annotation": "service-annotation-value"},
		},
	}

	labels := createServiceAndDeploymentLabels(reaper)
	podLabels := utils.MergeMap(labels, reaper.Spec.ResourceMeta.Pods.Labels)
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), pointer.String("keystore-password"), pointer.String("truststore-password"), logger)

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

	assert.Equal(t, podLabels, deployment.Spec.Template.Labels)
	assert.Equal(t, reaper.Spec.ResourceMeta.Pods.Annotations, deployment.Spec.Template.Annotations)

	podSpec := deployment.Spec.Template.Spec
	assert.Len(t, podSpec.Containers, 1)

	container := podSpec.Containers[0]

	assert.Equal(t, "test/reaper:latest", container.Image)
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
			Value: "[cluster1-dc1-service]",
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
			Value: "-Djavax.net.ssl.keyStore=/mnt/client-keystore/keystore -Djavax.net.ssl.keyStorePassword=keystore-password -Djavax.net.ssl.trustStore=/mnt/client-truststore/truststore -Djavax.net.ssl.trustStorePassword=truststore-password -Dssl.enable=true",
		},
		{
			Name:  "REAPER_CASS_NATIVE_PROTOCOL_SSL_ENCRYPTION_ENABLED",
			Value: "true",
		},
	})

	assert.Len(t, podSpec.InitContainers, 1)

	initContainer := podSpec.InitContainers[0]
	assert.Equal(t, "test/reaper-init:1.2.3", initContainer.Image)
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
			Value: "[cluster1-dc1-service]",
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
			Value: "-Djavax.net.ssl.keyStore=/mnt/client-keystore/keystore -Djavax.net.ssl.keyStorePassword=keystore-password -Djavax.net.ssl.trustStore=/mnt/client-truststore/truststore -Djavax.net.ssl.trustStorePassword=truststore-password -Dssl.enable=true",
		},
		{
			Name:  "REAPER_CASS_NATIVE_PROTOCOL_SSL_ENCRYPTION_ENABLED",
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

	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Len(t, container.Env, 6)

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_CASS_KEYSPACE",
		Value: "ks1",
	})

	reaper.Spec.AutoScheduling.Enabled = true
	deployment = NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Len(t, container.Env, 16)

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
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
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
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
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
	defaultImages := &configapi.Images{}
	defaults := &configapi.DefaultImages{}
	defaults.ImageComponents = make(configapi.ImageComponents)
	defaultImages.Others = make(map[string]string, 1)
	images.GetImageConfig().Images = defaultImages
	images.GetImageConfig().DefaultImages = defaults
	defaults.ImageComponents["reaper"] = configapi.ImageComponent{
		ImagePolicy: configapi.ImagePolicy{
			ImagePullPolicy: corev1.PullIfNotPresent,
		},
	}
	defaultImages.Others["reaper"] = "docker.io/thelastpickle/cassandra-reaper:3.2.1"
	// Note: nil images are normally not possible due to the kubebuilder markers on the CRD spec
	t.Run("nil images", func(t *testing.T) {
		reaper := newTestReaper()
		logger := testlogr.NewTestLogger(t)
		deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:3.2.1", deployment.Spec.Template.Spec.InitContainers[0].Image)
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:3.2.1", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("default images", func(t *testing.T) {
		reaper := newTestReaper()
		reaper.Spec.InitContainerImage = "docker.io/thelastpickle/cassandra-reaper:3.2.1"
		logger := testlogr.NewTestLogger(t)
		deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:3.2.1", deployment.Spec.Template.Spec.InitContainers[0].Image)
		assert.Equal(t, "docker.io/thelastpickle/cassandra-reaper:3.2.1", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy)
		assert.Equal(t, corev1.PullIfNotPresent, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		assert.Empty(t, deployment.Spec.Template.Spec.ImagePullSecrets)
	})
	t.Run("custom images", func(t *testing.T) {
		reaper := newTestReaper()
		reaper.Spec.InitContainerImage = "docker.io/my-custom-repo/my-custom-name:latest"
		reaper.Spec.ContainerImage = "docker.io/my-custom-repo/my-custom-name:latest"
		logger := testlogr.NewTestLogger(t)
		defaults.ImageComponents["reaper"] = configapi.ImageComponent{
			ImagePolicy: configapi.ImagePolicy{
				ImagePullPolicy: corev1.PullAlways,
				ImagePullSecret: corev1.LocalObjectReference{
					Name: "my-secret",
				},
			},
		}
		deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
		assert.Equal(t, "docker.io/my-custom-repo/my-custom-name:latest", deployment.Spec.Template.Spec.InitContainers[0].Image)
		assert.Equal(t, "docker.io/my-custom-repo/my-custom-name:latest", deployment.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(t, corev1.PullAlways, deployment.Spec.Template.Spec.InitContainers[0].ImagePullPolicy)
		assert.Equal(t, corev1.PullAlways, deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
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
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
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
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
	assert.EqualValues(t, affinity, deployment.Spec.Template.Spec.Affinity, "affinity does not match")
}

func TestContainerSecurityContext(t *testing.T) {
	readOnlyRootFilesystemOverride := true
	securityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystemOverride,
	}
	reaper := newTestReaper()
	reaper.Spec.SecurityContext = securityContext
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
	podSpec := deployment.Spec.Template.Spec

	assert.Len(t, podSpec.Containers, 1, "Expected a single container to exist")
	assert.Equal(t, podSpec.Containers[0].Name, "reaper")
	assert.EqualValues(t, securityContext, podSpec.Containers[0].SecurityContext, "securityContext does not match for container")
}

func TestSchemaInitContainerSecurityContext(t *testing.T) {
	readOnlyRootFilesystemOverride := true
	initContainerSecurityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystemOverride,
	}
	nonInitContainerSecurityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystemOverride,
	}

	reaper := newTestReaper()
	reaper.Spec.SecurityContext = nonInitContainerSecurityContext
	reaper.Spec.InitContainerSecurityContext = initContainerSecurityContext
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
	podSpec := deployment.Spec.Template.Spec

	assert.Equal(t, podSpec.InitContainers[0].Name, "reaper-schema-init")
	assert.Len(t, podSpec.InitContainers, 1, "Expected a single schema init container to exist")
	assert.EqualValues(t, initContainerSecurityContext, podSpec.InitContainers[0].SecurityContext, "securityContext does not match for schema init container")
}

func TestPodSecurityContext(t *testing.T) {
	runAsUser := int64(8675309)
	podSecurityContext := &corev1.PodSecurityContext{
		RunAsUser: &runAsUser,
	}
	reaper := newTestReaper()
	reaper.Spec.PodSecurityContext = podSecurityContext
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
	podSpec := deployment.Spec.Template.Spec

	assert.EqualValues(t, podSecurityContext, podSpec.SecurityContext, "podSecurityContext expected at pod level")
}

func TestSkipSchemaMigration(t *testing.T) {
	reaper := newTestReaper()
	reaper.Spec.SkipSchemaMigration = true
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)
	assert.Len(t, deployment.Spec.Template.Spec.InitContainers, 0, "expected pod template to not have any init container")
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
				Keyspace: "reaper_db",
				ResourceMeta: &meta.ResourceMeta{
					CommonLabels: map[string]string{"common": "everywhere", "override": "commonLevel"},
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

func TestDefaultResources(t *testing.T) {
	reaper := newTestReaper()
	logger := testlogr.NewTestLogger(t)
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)

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
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)

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
	deployment := NewDeployment(reaper, newTestDatacenter(), nil, nil, logger)

	deploymentLabels := map[string]string{
		k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		reaperapi.ReaperLabel:       reaper.Name,
		k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueReaperController,
		"common":                    "everywhere",
		"override":                  "commonLevel",
	}

	podLabels := map[string]string{
		k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		reaperapi.ReaperLabel:       reaper.Name,
		k8ssandraapi.CreatedByLabel: k8ssandraapi.CreatedByLabelValueReaperController,
		"common":                    "everywhere",
		"override":                  "podLevel",
		"pod-label":                 "pod-label-value",
	}

	assert.Equal(t, deploymentLabels, deployment.Labels)
	assert.Equal(t, podLabels, deployment.Spec.Template.Labels)
}
