package reaper

import (
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
)

func NewDeployment(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter, authVars ...*corev1.EnvVar) *appsv1.Deployment {
	labels := createServiceAndDeploymentLabels(reaper)

	selector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      k8ssandraapi.ManagedByLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{k8ssandraapi.NameLabelValue},
			},
			{
				Key:      api.ReaperLabel,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{reaper.Name},
			},
		},
	}

	readinessProbe := computeProbe(reaper.Spec.ReadinessProbe)
	livenessProbe := computeProbe(reaper.Spec.LivenessProbe)

	envVars := []corev1.EnvVar{
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
			Value: fmt.Sprintf("[%s]", dc.GetDatacenterServiceName()),
		},
		{
			Name:  "REAPER_AUTH_ENABLED",
			Value: "false",
		},
		{
			Name:  "REAPER_DATACENTER_AVAILABILITY",
			Value: reaper.Spec.DatacenterAvailability,
		},
		{
			Name:  "REAPER_CASS_LOCAL_DC",
			Value: dc.Name,
		},
		{
			Name:  "REAPER_CASS_KEYSPACE",
			Value: reaper.Spec.Keyspace,
		},
	}

	if reaper.Spec.AutoScheduling.Enabled {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "REAPER_AUTO_SCHEDULING_ENABLED",
			Value: "true",
		})
		adaptive, incremental := getAdaptiveIncremental(reaper, dc)
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

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reaper.Namespace,
			Name:      reaper.Name,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity: reaper.Spec.Affinity,
					InitContainers: []corev1.Container{
						{
							Name:            "reaper-schema-init",
							ImagePullPolicy: reaper.Spec.ImagePullPolicy,
							Image:           reaper.Spec.Image,
							SecurityContext: reaper.Spec.InitContainerSecurityContext,
							Env:             envVars,
							Args:            []string{"schema-migration"},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "reaper",
							ImagePullPolicy: reaper.Spec.ImagePullPolicy,
							Image:           reaper.Spec.Image,
							SecurityContext: reaper.Spec.SecurityContext,
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
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
							Env:            envVars,
						},
					},
					ServiceAccountName: reaper.Spec.ServiceAccountName,
					Tolerations:        reaper.Spec.Tolerations,
					SecurityContext:    reaper.Spec.PodSecurityContext,
				},
			},
		},
	}
	addAuthEnvVars(deployment, authVars)
	utils.AddHashAnnotation(deployment, k8ssandraapi.ResourceHashAnnotation)
	return deployment
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
	probe.Handler = corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/healthcheck",
			Port: intstr.FromInt(8081),
		},
	}
	return probe
}

func addAuthEnvVars(deployment *appsv1.Deployment, vars []*corev1.EnvVar) {
	initEnvVars := deployment.Spec.Template.Spec.InitContainers[0].Env
	envVars := deployment.Spec.Template.Spec.Containers[0].Env
	for _, v := range vars {
		initEnvVars = append(initEnvVars, *v)
		envVars = append(envVars, *v)
	}
	deployment.Spec.Template.Spec.InitContainers[0].Env = initEnvVars
	deployment.Spec.Template.Spec.Containers[0].Env = envVars
}

func getAdaptiveIncremental(reaper *api.Reaper, dc *cassdcapi.CassandraDatacenter) (adaptive bool, incremental bool) {
	switch reaper.Spec.AutoScheduling.RepairType {
	case "ADAPTIVE":
		adaptive = true
	case "INCREMENTAL":
		incremental = true
	case "AUTO":
		if cassandra.IsCassandra3(dc.Spec.ServerVersion) {
			adaptive = true
		} else {
			incremental = true
		}
	}
	return
}
