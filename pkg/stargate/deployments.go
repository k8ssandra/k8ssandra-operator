package stargate

import (
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"strings"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	cassandraConfigDir  = "/config"
	cassandraConfigPath = "/config/cassandra.yaml"

	// FIXME should this be customized? Cf. K8ssandra 1.x Helm chart template:
	// "{{ .Values.clusterDomain | default \"cluster.local\" }}
	clusterDomain = "cluster.local"
)

const (
	DefaultImageRepository = "stargateio"
	DefaultImageName3      = "stargate-3_11"
	DefaultImageName4      = "stargate-4_0"
	DefaultVersion         = "1.0.45"
	// When changing the default version above, please also change the kubebuilder marker in
	// apis/stargate/v1alpha1/stargate_types.go accordingly.
)

type ClusterVersion string

const (
	ClusterVersion3 ClusterVersion = "3.11"
	ClusterVersion4 ClusterVersion = "4.0"
)

var (
	defaultImage3 = images.Image{
		Registry:   images.DefaultRegistry,
		Repository: DefaultImageRepository,
		Name:       DefaultImageName3,
		Tag:        "v" + DefaultVersion,
	}
	defaultImage4 = images.Image{
		Registry:   images.DefaultRegistry,
		Repository: DefaultImageRepository,
		Name:       DefaultImageName4,
		Tag:        "v" + DefaultVersion,
	}
)

// NewDeployments compute the Deployments to create for the given Stargate and CassandraDatacenter
// resources.
func NewDeployments(stargate *api.Stargate, dc *cassdcapi.CassandraDatacenter) map[string]appsv1.Deployment {

	clusterVersion := computeClusterVersion(dc)
	seedService := computeSeedServiceUrl(dc)

	racks := dc.GetRacks()
	replicasByRack := cassdcapi.SplitRacks(int(stargate.Spec.Size), len(racks))
	dnsPolicy := computeDNSPolicy(dc)

	var deployments = make(map[string]appsv1.Deployment)
	for i, rack := range racks {

		replicas := int32(replicasByRack[i])
		if replicas == 0 {
			break
		}

		template := stargate.GetRackTemplate(rack.Name).Coalesce(&stargate.Spec.StargateDatacenterTemplate)

		deploymentName := DeploymentName(dc, &rack)
		image := computeImage(template, clusterVersion)
		resources := computeResourceRequirements(template)
		livenessProbe := computeLivenessProbe(template)
		readinessProbe := computeReadinessProbe(template)
		jvmOptions := computeJvmOptions(template)
		volumes := computeVolumes(template)
		volumeMounts := computeVolumeMounts(template)
		serviceAccountName := computeServiceAccount(template)
		nodeSelector := computeNodeSelector(template, dc)
		tolerations := computeTolerations(template, dc)
		affinity := computeAffinity(template, dc, &rack)

		deployment := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        deploymentName,
				Namespace:   stargate.Namespace,
				Annotations: map[string]string{},
				Labels: map[string]string{
					utils.NameLabel:      utils.NameLabelValue,
					utils.PartOfLabel:    utils.PartOfLabelValue,
					utils.ComponentLabel: utils.ComponentLabelValueStargate,
					utils.CreatedByLabel: utils.CreatedByLabelValueStargateController,
					api.StargateLabel:    stargate.Name,
				},
			},

			Spec: appsv1.DeploymentSpec{

				Replicas: &replicas,

				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						api.StargateDeploymentLabel: deploymentName,
					},
				},

				Template: corev1.PodTemplateSpec{

					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							utils.NameLabel:             utils.NameLabelValue,
							utils.PartOfLabel:           utils.PartOfLabelValue,
							utils.ComponentLabel:        utils.ComponentLabelValueStargate,
							utils.CreatedByLabel:        utils.CreatedByLabelValueStargateController,
							api.StargateLabel:           stargate.Name,
							api.StargateDeploymentLabel: deploymentName,
						},
					},

					Spec: corev1.PodSpec{

						ServiceAccountName: serviceAccountName,

						HostNetwork:      dc.IsHostNetworkEnabled(),
						DNSPolicy:        dnsPolicy,
						ImagePullSecrets: images.CollectPullSecrets(image),

						Containers: []corev1.Container{{

							Name:            deploymentName,
							Image:           image.String(),
							ImagePullPolicy: image.PullPolicy,

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

							Resources: resources,

							Env: []corev1.EnvVar{
								{
									Name: "LISTEN",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
								{Name: "JAVA_OPTS", Value: jvmOptions},
								{Name: "CLUSTER_NAME", Value: dc.Spec.ClusterName},
								{Name: "CLUSTER_VERSION", Value: string(clusterVersion)},
								{Name: "SEED", Value: seedService},
								{Name: "DATACENTER_NAME", Value: dc.Name},
								{Name: "RACK_NAME", Value: rack.Name},
								{Name: "ENABLE_AUTH", Value: "true"},
								// Watching bundles is unnecessary in a k8s deployment. See
								// https://github.com/stargate/stargate/issues/1286 for
								// details.
								{Name: "DISABLE_BUNDLES_WATCH", Value: "true"},
							},

							LivenessProbe:  &livenessProbe,
							ReadinessProbe: &readinessProbe,

							VolumeMounts: volumeMounts,
						}},

						NodeSelector: nodeSelector,
						Tolerations:  tolerations,
						Affinity:     affinity,
						Volumes:      volumes,
					},
				},
			},
		}

		klusterName, nameFound := stargate.Labels[utils.K8ssandraClusterNameLabel]
		klusterNamespace, namespaceFound := stargate.Labels[utils.K8ssandraClusterNamespaceLabel]

		if nameFound && namespaceFound {
			deployment.Labels[utils.K8ssandraClusterNameLabel] = klusterName
			deployment.Spec.Template.Labels[utils.K8ssandraClusterNameLabel] = klusterName
			deployment.Spec.Template.Labels[utils.K8ssandraClusterNamespaceLabel] = klusterNamespace
		}
		deployment.Annotations[utils.ResourceHashAnnotation] = utils.DeepHashString(deployment)
		deployments[deploymentName] = deployment
	}
	return deployments
}

func computeDNSPolicy(dc *cassdcapi.CassandraDatacenter) corev1.DNSPolicy {
	if dc.IsHostNetworkEnabled() {
		return corev1.DNSClusterFirstWithHostNet
	}
	return corev1.DNSClusterFirst
}

func computeSeedServiceUrl(dc *cassdcapi.CassandraDatacenter) string {
	return dc.Spec.ClusterName + "-seed-service." + dc.Namespace + ".svc." + clusterDomain
}

func computeClusterVersion(dc *cassdcapi.CassandraDatacenter) ClusterVersion {
	cassandraVersion := dc.Spec.ServerVersion
	if strings.HasPrefix(cassandraVersion, "3") {
		return ClusterVersion3
	} else {
		return ClusterVersion4
	}
}

func computeImage(template *api.StargateTemplate, clusterVersion ClusterVersion) *images.Image {
	if clusterVersion == ClusterVersion3 {
		return template.ContainerImage.ApplyDefaults(defaultImage3)
	} else {
		return template.ContainerImage.ApplyDefaults(defaultImage4)
	}
}

func computeResourceRequirements(template *api.StargateTemplate) corev1.ResourceRequirements {
	if template.Resources != nil {
		return *template.Resources
	} else {
		heapSize := computeHeapSize(template)
		memoryRequest := heapSize.DeepCopy()
		memoryRequest.Add(memoryRequest) // heap x2
		memoryLimit := memoryRequest.DeepCopy()
		memoryLimit.Add(memoryLimit) // heap x4
		return corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: memoryRequest,
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("1000m"),
				corev1.ResourceMemory: memoryLimit,
			},
		}
	}
}

func computeLivenessProbe(template *api.StargateTemplate) corev1.Probe {
	var livenessProbe corev1.Probe
	if template.LivenessProbe != nil {
		livenessProbe = *template.LivenessProbe
	} else {
		livenessProbe = corev1.Probe{
			TimeoutSeconds:      10,
			InitialDelaySeconds: 30,
			FailureThreshold:    5,
		}
	}
	// The handlers cannot be user-specified, so force them now
	livenessProbe.Handler = corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/checker/liveness",
			Port: intstr.FromString("health"),
		},
	}
	return livenessProbe
}

func computeReadinessProbe(template *api.StargateTemplate) corev1.Probe {
	var readinessProbe corev1.Probe
	if template.ReadinessProbe != nil {
		readinessProbe = *template.ReadinessProbe
	} else {
		readinessProbe = corev1.Probe{
			TimeoutSeconds:      10,
			InitialDelaySeconds: 30,
			FailureThreshold:    5,
		}
	}
	// The handlers cannot be user-specified, so force them now
	readinessProbe.Handler = corev1.Handler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/checker/readiness",
			Port: intstr.FromString("health"),
		},
	}
	return readinessProbe
}

func computeJvmOptions(template *api.StargateTemplate) string {
	heapSize := computeHeapSize(template)
	heapSizeInBytes := heapSize.Value()
	jvmOptions := fmt.Sprintf("-XX:+CrashOnOutOfMemoryError -Xms%v -Xmx%v", heapSizeInBytes, heapSizeInBytes)
	if template.CassandraConfigMapRef != nil {
		jvmOptions += fmt.Sprintf(
			" -Dstargate.unsafe.cassandra_config_path=%s",
			cassandraConfigPath,
		)
	}
	return jvmOptions
}

func computeHeapSize(template *api.StargateTemplate) resource.Quantity {
	if template.HeapSize != nil {
		return *template.HeapSize
	}
	return resource.MustParse("256Mi")
}

func computeVolumes(template *api.StargateTemplate) []corev1.Volume {
	if template.CassandraConfigMapRef != nil {
		return []corev1.Volume{{
			Name: "cassandra-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: *template.CassandraConfigMapRef,
				},
			},
		}}
	}
	return nil
}

func computeVolumeMounts(template *api.StargateTemplate) []corev1.VolumeMount {
	if template.CassandraConfigMapRef != nil {
		return []corev1.VolumeMount{{
			Name:      "cassandra-config",
			MountPath: cassandraConfigDir,
		}}
	}
	return nil
}

func computeServiceAccount(template *api.StargateTemplate) string {
	if template.ServiceAccount != nil {
		return *template.ServiceAccount
	}
	return "default"
}

func computeNodeSelector(template *api.StargateTemplate, dc *cassdcapi.CassandraDatacenter) map[string]string {
	if template.NodeSelector != nil {
		return template.NodeSelector
	} else if dc.Spec.NodeSelector != nil {
		return dc.Spec.NodeSelector
	} else if dc.Spec.PodTemplateSpec != nil {
		return dc.Spec.PodTemplateSpec.Spec.NodeSelector
	}
	return nil
}

func computeTolerations(template *api.StargateTemplate, dc *cassdcapi.CassandraDatacenter) []corev1.Toleration {
	if template.Tolerations != nil {
		return template.Tolerations
	}
	return dc.Spec.Tolerations
}

func computeAffinity(template *api.StargateTemplate, dc *cassdcapi.CassandraDatacenter, rack *cassdcapi.Rack) *corev1.Affinity {
	if template.Affinity != nil {
		return template.Affinity
	}
	allowStargateOnDataNodes := false
	if template != nil {
		allowStargateOnDataNodes = template.AllowStargateOnDataNodes
	}
	return &corev1.Affinity{
		NodeAffinity:    computeNodeAffinity(dc, rack.Name),
		PodAntiAffinity: computePodAntiAffinity(allowStargateOnDataNodes, dc, rack.Name),
	}
}
