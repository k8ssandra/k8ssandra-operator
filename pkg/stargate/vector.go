package stargate

import (
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandra "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetricsPort         = 8084
	VectorContainerName = "stargate-vector-agent"
	vectorConfigMap     = "stargate-vector"
)

// VectorAgentConfigMapName generates a ConfigMap name based on
// the K8s sanitized cluster name and datacenter name.
func VectorAgentConfigMapName(clusterName, dcName string) string {
	return fmt.Sprintf("%s-%s-%s", cassdcapi.CleanupForKubernetes(clusterName), dcName, vectorConfigMap)
}

func CreateVectorConfigMap(namespace, vectorToml string, dc cassdcapi.CassandraDatacenter) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VectorAgentConfigMapName(dc.Spec.ClusterName, dc.Name),
			Namespace: namespace,
			Labels: utils.MergeMap(
				map[string]string{
					k8ssandra.NameLabel:      k8ssandra.NameLabelValue,
					k8ssandra.PartOfLabel:    k8ssandra.PartOfLabelValue,
					k8ssandra.ComponentLabel: k8ssandra.ComponentLabelValueStargate,
				},
				labels.CleanedUpByLabels(client.ObjectKey{Namespace: namespace, Name: dc.Labels[k8ssandra.K8ssandraClusterNameLabel]})),
		},
		Data: map[string]string{
			"vector.toml": vectorToml,
		},
	}
}

func configureVector(stargate *api.Stargate, deployment *appsv1.Deployment, dc *cassdcapi.CassandraDatacenter, logger logr.Logger) {
	if stargate.Spec.Telemetry.IsVectorEnabled() {
		logger.Info("Injecting Vector agent into Stargate deployments")
		vectorImage := vector.DefaultVectorImage
		if stargate.Spec.Telemetry.Vector.Image != "" {
			vectorImage = stargate.Spec.Telemetry.Vector.Image
		}

		// Create the definition of the Vector agent container
		vectorAgentContainer := corev1.Container{
			Name:            VectorContainerName,
			Image:           vectorImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env: []corev1.EnvVar{
				{Name: "VECTOR_CONFIG", Value: "/etc/vector/vector.toml"},
				{Name: "VECTOR_ENVIRONMENT", Value: "kubernetes"},
				{Name: "VECTOR_HOSTNAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "stargate-vector-config", MountPath: "/etc/vector"},
			},
			Resources: vector.VectorContainerResources(stargate.Spec.Telemetry),
		}
		// Create the definition of the Vector agent config map volume
		logger.Info("Creating Stargate Vector Agent Volume")
		vectorAgentVolume := corev1.Volume{
			Name: "stargate-vector-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: VectorAgentConfigMapName(dc.Spec.ClusterName, dc.Name)},
				},
			},
		}
		// Add the container and volume to the deployment
		cassandra.UpdateContainer(&deployment.Spec.Template, VectorContainerName, func(c *corev1.Container) {
			*c = vectorAgentContainer
		})
		cassandra.AddVolumesToPodTemplateSpec(&deployment.Spec.Template, vectorAgentVolume)
	}
}
