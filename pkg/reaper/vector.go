package reaper

import (
	"fmt"

	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandra "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	MetricsPort         = 8081
	VectorContainerName = "reaper-vector-agent"
	vectorConfigMap     = "reaper-vector"
)

// VectorAgentConfigMapName generates a ConfigMap name based on
// the K8s sanitized cluster name and datacenter name.
func VectorAgentConfigMapName(clusterName, dcName string) string {
	return fmt.Sprintf("%s-%s-%s", cassdcapi.CleanupForKubernetes(clusterName), cassdcapi.CleanupForKubernetes(dcName), vectorConfigMap)
}

func CreateVectorConfigMap(namespace, vectorToml string, dc cassdcapi.CassandraDatacenter) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VectorAgentConfigMapName(dc.Spec.ClusterName, dc.DatacenterName()),
			Namespace: namespace,
			Labels: utils.MergeMap(
				map[string]string{
					k8ssandra.NameLabel:      k8ssandra.NameLabelValue,
					k8ssandra.PartOfLabel:    k8ssandra.PartOfLabelValue,
					k8ssandra.ComponentLabel: k8ssandra.ComponentLabelValueReaper,
				},
				labels.CleanedUpByLabels(client.ObjectKey{Namespace: namespace, Name: dc.Labels[k8ssandra.K8ssandraClusterNameLabel]})),
		},
		Data: map[string]string{
			"vector.toml": vectorToml,
		},
	}
}

func configureVector(reaper *api.Reaper, template *corev1.PodTemplateSpec, dc *cassdcapi.CassandraDatacenter, logger logr.Logger) {
	if reaper.Spec.Telemetry.IsVectorEnabled() {
		logger.Info("Injecting Vector agent into Reaper deployments")
		vectorImage := vector.DefaultVectorImage
		if reaper.Spec.Telemetry.Vector.Image != "" {
			vectorImage = reaper.Spec.Telemetry.Vector.Image
		}

		// Default security context for vector container
		defaultVectorSecurityContext := &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			RunAsUser:                ptr.To[int64](1000),
			ReadOnlyRootFilesystem:   ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		}

		// Create the definition of the Vector agent container
		vectorAgentContainer := corev1.Container{
			Name:            VectorContainerName,
			Image:           vectorImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: defaultVectorSecurityContext,
			Env: []corev1.EnvVar{
				{Name: "VECTOR_CONFIG", Value: "/etc/vector/vector.toml"},
				{Name: "VECTOR_ENVIRONMENT", Value: "kubernetes"},
				{Name: "VECTOR_HOSTNAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "reaper-vector-config", MountPath: "/etc/vector"},
			},
			Resources: vector.VectorContainerResources(reaper.Spec.Telemetry),
		}
		// Create the definition of the Vector agent config map volume
		logger.Info("Creating Reaper Vector Agent Volume")
		vectorAgentVolume := corev1.Volume{
			Name: "reaper-vector-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: VectorAgentConfigMapName(dc.Spec.ClusterName, dc.DatacenterName())},
				},
			},
		}
		// Add the container and volume to the deployment
		cassandra.UpdateContainer(template, VectorContainerName, func(c *corev1.Container) {
			*c = vectorAgentContainer
		})
		cassandra.AddVolumesToPodTemplateSpec(template, vectorAgentVolume)
	}
}
