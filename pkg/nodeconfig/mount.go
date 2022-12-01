package nodeconfig

import (
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	PerNodeConfigInitContainerName = "per-node-config"
	PerNodeConfigVolumeName        = "per-node-config"
)

// MountPerNodeConfig mounts the per-node-config ConfigMap, on all pods in the given datacenter and
// adds an init container that will merge the per-node config into the main config. This function
// should only be called for DCs having a per-node ConfigMap reference.
func MountPerNodeConfig(dcConfig *cassandra.DatacenterConfig) {
	// if the config-builder init container isn't found, declare a placeholder now to guarantee order of execution
	cassandra.UpdateInitContainer(dcConfig.PodTemplateSpec, reconciliation.ServerConfigContainerName, func(container *v1.Container) {})
	// add per-node-config init container
	_ = cassandra.AddInitContainersToPodTemplateSpec(dcConfig, newPerNodeConfigInitContainer(dcConfig.PerNodeInitContainerImage))
	// add per-node config volume to pod spec
	cassandra.AddVolumesToPodTemplateSpec(dcConfig, newPerNodeConfigVolume(dcConfig.PerNodeConfigMapRef.Name))
}

const (
	DefaultPerNodeImageRepository = "mikefarah"
	DefaultPerNodeImageName       = "yq"
	DefaultPerNodeImageTag        = "4"
)

var (
	defaultPerNodeImage = images.Image{
		Registry:   images.DefaultRegistry,
		Repository: DefaultPerNodeImageRepository,
		Name:       DefaultPerNodeImageName,
		Tag:        DefaultPerNodeImageTag,
	}
	// When changing the defaults above, please also change the kubebuilder marker in
	// apis/k8ssandra/v1alpha1/k8ssandracluster_types.go accordingly.
)

func newPerNodeConfigInitContainer(image *images.Image) v1.Container {
	var perNodeConfigInitContainer = v1.Container{
		Name: PerNodeConfigInitContainerName,
		Resources: v1.ResourceRequirements{
			Requests: v1.ResourceList{
				"cpu":    resource.MustParse("10m"),
				"memory": resource.MustParse("16Mi"),
			},
			Limits: v1.ResourceList{
				"cpu":    resource.MustParse("100m"),
				"memory": resource.MustParse("64Mi"),
			},
		},
		Env: []v1.EnvVar{
			{
				Name: "POD_NAME",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "server-config", // volume will be created by cass-operator
				MountPath: "/config",
			},
			{
				Name:      PerNodeConfigVolumeName,
				MountPath: "/per-node-config",
			},
		},
		Command: []string{
			"sh",
			"-c",
			"if [ -e /per-node-config/${POD_NAME}_* ]; then " +
				"for src in /per-node-config/${POD_NAME}_*; do " +
				"dest=/config/`echo $src | cut -d \"_\" -f2`; " +
				"yq ea '. as $item ireduce ({}; . * $item)' -i $dest $src && echo merged $src into $dest || exit 1; " +
				"done; " +
				"echo done merging per-node config for pod $POD_NAME; " +
				"else " +
				"echo no per-node config found for pod $POD_NAME; " +
				"fi",
		},
	}

	perNodeConfigImage := image.ApplyDefaults(defaultPerNodeImage)
	perNodeConfigInitContainer.Image = perNodeConfigImage.String()
	perNodeConfigInitContainer.ImagePullPolicy = perNodeConfigImage.PullPolicy
	return perNodeConfigInitContainer
}

func newPerNodeConfigVolume(perNodeConfigMapName string) v1.Volume {
	return v1.Volume{
		Name: PerNodeConfigVolumeName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: perNodeConfigMapName,
				},
			},
		},
	}
}
