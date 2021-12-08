package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestNewDeployment(t *testing.T) {
	mainImage := &reaperapi.ContainerImage{Repository: "test", Name: "reaper", PullPolicy: corev1.PullAlways}
	initImage := &reaperapi.ContainerImage{Repository: "test", Name: "reaper-init", PullPolicy: corev1.PullNever}
	reaper := newTestReaper()
	reaper.Spec.ContainerImage = mainImage
	reaper.Spec.InitContainerImage = initImage
	reaper.Spec.AutoScheduling = reaperapi.AutoScheduling{Enabled: false}
	reaper.Spec.ServiceAccountName = "reaper"
	reaper.Spec.DatacenterAvailability = DatacenterAvailabilityLocal

	labels := createServiceAndDeploymentLabels(reaper)
	deployment := NewDeployment(reaper, newTestDatacenter())

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

	assert.Equal(t, labels, deployment.Spec.Template.Labels)

	podSpec := deployment.Spec.Template.Spec
	assert.Len(t, podSpec.Containers, 1)

	container := podSpec.Containers[0]

	assert.Equal(t, "docker.io/test/reaper:latest", container.Image)
	assert.Equal(t, corev1.PullAlways, container.ImagePullPolicy)
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
			Name:  "REAPER_AUTH_ENABLED",
			Value: "false",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: DatacenterAvailabilityLocal,
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

	assert.Len(t, podSpec.InitContainers, 1)

	initContainer := podSpec.InitContainers[0]
	assert.Equal(t, "docker.io/test/reaper-init:latest", initContainer.Image)
	assert.Equal(t, corev1.PullNever, initContainer.ImagePullPolicy)
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
			Name:  "REAPER_AUTH_ENABLED",
			Value: "false",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: DatacenterAvailabilityLocal,
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

	deployment = NewDeployment(reaper, newTestDatacenter())
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Len(t, container.Env, 7)

	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  "REAPER_CASS_KEYSPACE",
		Value: "ks1",
	})

	reaper.Spec.AutoScheduling.Enabled = true
	deployment = NewDeployment(reaper, newTestDatacenter())
	podSpec = deployment.Spec.Template.Spec
	container = podSpec.Containers[0]
	assert.Len(t, container.Env, 17)

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
		Handler: corev1.Handler{
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
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/custom",
				Port: intstr.FromInt(8080),
			},
		},
		InitialDelaySeconds: 123,
	}
	deployment := NewDeployment(reaper, newTestDatacenter())
	expected := &corev1.Probe{
		Handler: corev1.Handler{
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
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/custom",
				Port: intstr.FromInt(8080),
			},
		},
		InitialDelaySeconds: 123,
	}
	deployment := NewDeployment(reaper, newTestDatacenter())
	expected := &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthcheck",
				Port: intstr.FromInt(8081),
			},
		},
		InitialDelaySeconds: 123,
	}
	assert.Equal(t, expected, deployment.Spec.Template.Spec.Containers[0].LivenessProbe)
}

func Test_computeImage(t *testing.T) {
	tests := []struct {
		name     string
		input    *reaperapi.ContainerImage
		expected string
	}{
		{"nil", nil, DefaultReaperImage},
		{"non nil with defaults", &reaperapi.ContainerImage{}, "docker.io/thelastpickle/cassandra-reaper:latest"},
		{"non nil with custom values", &reaperapi.ContainerImage{
			Registry:   "localhost:5000",
			Repository: "k8ssandra",
			Name:       "reaper",
			Tag:        "1.2.3",
		}, "localhost:5000/k8ssandra/reaper:1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := computeImage(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_computeImagePullPolicy(t *testing.T) {
	tests := []struct {
		name     string
		input    *reaperapi.ContainerImage
		expected corev1.PullPolicy
	}{
		{"nil", nil, corev1.PullAlways},
		{"non nil with defaults", &reaperapi.ContainerImage{}, corev1.PullAlways},
		{"non nil with non-latest", &reaperapi.ContainerImage{Tag: "1.2.3"}, corev1.PullIfNotPresent},
		{"non nil with custom values", &reaperapi.ContainerImage{PullPolicy: corev1.PullNever}, corev1.PullNever},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := computeImagePullPolicy(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_computeImagePullSecrets(t *testing.T) {
	tests := []struct {
		name     string
		main     *reaperapi.ContainerImage
		init     *reaperapi.ContainerImage
		expected []corev1.LocalObjectReference
	}{
		{"nil", nil, nil, nil},
		{"non nil with defaults", &reaperapi.ContainerImage{}, &reaperapi.ContainerImage{}, nil},
		{
			"non nil with custom values",
			&reaperapi.ContainerImage{PullSecretRef: &corev1.LocalObjectReference{Name: "my-main-secret"}},
			&reaperapi.ContainerImage{PullSecretRef: &corev1.LocalObjectReference{Name: "my-init-secret"}},
			[]corev1.LocalObjectReference{{Name: "my-main-secret"}, {Name: "my-init-secret"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := computeImagePullSecrets(tt.main, tt.init)
			assert.Equal(t, tt.expected, actual)
		})
	}
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

	deployment := NewDeployment(reaper, newTestDatacenter())
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

	deployment := NewDeployment(reaper, newTestDatacenter())
	assert.EqualValues(t, affinity, deployment.Spec.Template.Spec.Affinity, "affinity does not match")
}

func TestContainerSecurityContext(t *testing.T) {
	readOnlyRootFilesystemOverride := true
	securityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystemOverride,
	}
	reaper := newTestReaper()
	reaper.Spec.SecurityContext = securityContext

	deployment := NewDeployment(reaper, newTestDatacenter())
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

	deployment := NewDeployment(reaper, newTestDatacenter())
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

	deployment := NewDeployment(reaper, newTestDatacenter())
	podSpec := deployment.Spec.Template.Spec

	assert.EqualValues(t, podSecurityContext, podSpec.SecurityContext, "podSecurityContext expected at pod level")
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
			ReaperClusterTemplate: reaperapi.ReaperClusterTemplate{
				Keyspace: "reaper_db",
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
