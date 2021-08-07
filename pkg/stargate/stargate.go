package stargate

import (
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
)

const (
	cassandraConfigPath = "/config/cassandra.yaml"
)

// NewDeployment creates a Deployment object for the given Stargate and CassandraDatacenter
// resources.
func NewDeployment(stargate *api.Stargate, cassdc *cassdcapi.CassandraDatacenter) *appsv1.Deployment {
	cassandraVersion := cassdc.Spec.ServerVersion

	var clusterVersion string
	if strings.HasPrefix(cassandraVersion, "3") {
		clusterVersion = "3.11"
	} else {
		clusterVersion = "4.0"
	}

	var image string
	pullPolicy := corev1.PullIfNotPresent
	containerImage := stargate.Spec.StargateContainerImage
	if containerImage == nil {
		if clusterVersion == "3.11" {
			image = fmt.Sprintf("%s/%s:v%s", "stargateio", "stargate-3_11", api.DefaultStargateVersion)
		} else {
			image = fmt.Sprintf("%s/%s:v%s", "stargateio", "stargate-4_0", api.DefaultStargateVersion)
		}
	} else {
		if containerImage.Registry == nil {
			defaultRegistry := "docker.io"
			containerImage.Registry = &defaultRegistry
		}
		if containerImage.Tag == nil {
			defaultTag := "latest"
			containerImage.Tag = &defaultTag
		}
		image = fmt.Sprintf("%v/%v:%v", containerImage.Registry, containerImage.Repository, containerImage.Tag)
		if containerImage.PullPolicy != nil {
			pullPolicy = *containerImage.PullPolicy
		}
	}

	clusterName := cassdc.Spec.ClusterName
	// FIXME can this be customized? "{{ .Values.clusterDomain | default \"cluster.local\" }}
	clusterDomain := "cluster.local"

	dcName := cassdc.Name
	seedService := clusterName + "-seed-service." + cassdc.Namespace + ".svc." + clusterDomain

	deploymentName := getStargateContainerName(cassdc)

	heapSize := stargate.Spec.HeapSize
	heapSizeInBytes := heapSize.Value()

	var resources *corev1.ResourceRequirements
	if stargate.Spec.Resources == nil {
		memoryRequest := heapSize.DeepCopy()
		memoryRequest.Add(memoryRequest) // heap x2
		memoryLimit := memoryRequest.DeepCopy()
		memoryLimit.Add(memoryLimit) // heap x4
		resources = &corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: memoryRequest,
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1000m"),
				corev1.ResourceMemory: memoryLimit,
			},
		}
	} else {
		resources = stargate.Spec.Resources
	}

	var livenessProbe *corev1.Probe
	if stargate.Spec.LivenessProbe == nil {
		livenessProbe = &corev1.Probe{
			TimeoutSeconds:      10,
			InitialDelaySeconds: 30,
			FailureThreshold:    5,
		}
	} else {
		livenessProbe = stargate.Spec.LivenessProbe
	}

	var readinessProbe *corev1.Probe
	if stargate.Spec.ReadinessProbe == nil {
		readinessProbe = &corev1.Probe{
			TimeoutSeconds:      10,
			InitialDelaySeconds: 30,
			FailureThreshold:    5,
		}
	} else {
		readinessProbe = stargate.Spec.ReadinessProbe
	}

	volumes := make([]corev1.Volume, 0)
	volumeMounts := make([]corev1.VolumeMount, 0)
	var javaOpts string

	if stargate.Spec.CassandraConfigMap == nil {
		javaOpts = fmt.Sprintf("-XX:+CrashOnOutOfMemoryError -Xms%v -Xmx%v", heapSizeInBytes, heapSizeInBytes)
	} else {
		javaOpts = fmt.Sprintf("-XX:+CrashOnOutOfMemoryError -Xms%v -Xmx%v -Dstargate.unsafe.cassandra_config_path=%s", heapSizeInBytes, heapSizeInBytes, cassandraConfigPath)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: "cassandra-config", MountPath: "/config"})
		volumes = append(volumes, corev1.Volume{
			Name: "cassandra-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: *stargate.Spec.CassandraConfigMap,
					Items: []corev1.KeyToPath{
						{
							Key: "cassandra.yaml",
							Path: "cassandra.yaml",
						},
					},
				},
			},
		})
	}

	// The handlers cannot be user-specified, so force them now
	livenessProbe.Handler = corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/checker/liveness",
			Port: intstr.FromString("health"),
		},
	}
	readinessProbe.Handler = corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/checker/readiness",
			Port: intstr.FromString("health"),
		},
	}

	serviceAccountName := "default"
	if stargate.Spec.ServiceAccount != nil {
		serviceAccountName = *stargate.Spec.ServiceAccount
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{

			Name:        deploymentName,
			Namespace:   stargate.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.StargateLabel: deploymentName,
			},
		},

		Spec: appsv1.DeploymentSpec{

			Replicas: &stargate.Spec.Size,

			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{api.StargateLabel: deploymentName},
			},

			Template: corev1.PodTemplateSpec{

				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{api.StargateLabel: deploymentName},
				},

				Spec: corev1.PodSpec{

					ServiceAccountName: serviceAccountName,

					Containers: []corev1.Container{{

						Name:            deploymentName,
						Image:           image,
						ImagePullPolicy: pullPolicy,

						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080, Name: "graphql"},
							{ContainerPort: 8081, Name: "authorization"},
							{ContainerPort: 8082, Name: "rest"},
							{ContainerPort: 8084, Name: "health"},
							{ContainerPort: 8085, Name: "metrics"},
							{ContainerPort: 8090, Name: "http-schemaless"},
							{ContainerPort: 9042, Name: "native"},
							{ContainerPort: 8609, Name: "inter-node-msg"},
							{ContainerPort: 7000, Name: "intra-node"},
							{ContainerPort: 7001, Name: "tls-intra-node"},
						},

						Resources: *resources,

						Env: []corev1.EnvVar{
							{Name: "JAVA_OPTS", Value: javaOpts},
							{Name: "CLUSTER_NAME", Value: clusterName},
							{Name: "CLUSTER_VERSION", Value: clusterVersion},
							{Name: "SEED", Value: seedService},
							{Name: "DATACENTER_NAME", Value: dcName},
							// The rack name is temporarily hard coded until we get multi-rack support implemented.
							// See https://github.com/k8ssandra/k8ssandra/issues/54.
							{Name: "RACK_NAME", Value: "default"},
							{Name: "ENABLE_AUTH", Value: "true"},
						},

						LivenessProbe:  livenessProbe,
						ReadinessProbe: readinessProbe,

						VolumeMounts: volumeMounts,
					}},

					Affinity:    stargate.Spec.Affinity,
					Tolerations: stargate.Spec.Tolerations,
					Volumes:     volumes,
				},
			},
		},
	}
	deployment.Annotations[api.ResourceHashAnnotation] = utils.DeepHashString(deployment)
	return deployment
}

// NewService creates a Service object for the given Stargate and CassandraDatacenter
// resources.
func NewService(sg *api.Stargate, dc *cassdcapi.CassandraDatacenter) *corev1.Service {
	serviceName := dc.Spec.ClusterName + "-" + dc.Name + "-stargate-service"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   sg.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.StargateLabel: serviceName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{Port: 8080, Name: "graphql"},
				{Port: 8081, Name: "authorization"},
				{Port: 8082, Name: "rest"},
				{Port: 8084, Name: "health"},
				{Port: 8085, Name: "metrics"},
				{Port: 9042, Name: "cassandra"},
			},
			Selector: map[string]string{
				api.StargateLabel: serviceName,
			},
		},
	}
	service.Annotations[api.ResourceHashAnnotation] = utils.DeepHashString(service)
	return service
}

func getStargateContainerName(dc *cassdcapi.CassandraDatacenter) string {
	return dc.Spec.ClusterName + "-" + dc.Name + "-stargate-deployment"
}

func IsReady(sg *api.Stargate) bool {
	if sg.Status.Progress != api.StargateProgressRunning {
		return false
	}

	for _, condition := range sg.Status.Conditions {
		if condition.Type == api.StargateReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
