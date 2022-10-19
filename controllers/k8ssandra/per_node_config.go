package k8ssandra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcilePerNodeConfiguration(
	ctx context.Context,
	kcKey types.NamespacedName,
	dcConfig *cassandra.DatacenterConfig,
	remoteClient client.Client,
	logger logr.Logger,
) result.ReconcileResult {

	perNodeConfigKey := newPerNodeConfigKey(kcKey, dcConfig)
	logger = logger.WithValues("PerNodeConfigMap", perNodeConfigKey)

	logger.V(1).Info("Probing for per-node ConfigMap")
	perNodeConfig := &corev1.ConfigMap{}
	if err := remoteClient.Get(ctx, perNodeConfigKey, perNodeConfig); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("Per-node ConfigMap not found")
			return result.Continue()
		}
		logger.Error(err, "Failed to get per-node ConfigMap")
		return result.Error(err)
	}

	logger.Info("Found per-node ConfigMap, mounting")

	// if the config-builder init container isn't found, declare a placeholder now to guarantee order of execution
	cassandra.UpdateInitContainer(dcConfig.PodTemplateSpec, reconciliation.ServerConfigContainerName, func(container *corev1.Container) {})

	// add per-node-config init container
	_ = cassandra.AddInitContainersToPodTemplateSpec(dcConfig, *perNodeConfigInitContainer)

	// add per-node config volume to pod spec
	cassandra.AddVolumesToPodTemplateSpec(dcConfig, newPerNodeConfigVolume(perNodeConfigKey.Name))

	return result.Continue()
}

const perNodeConfigVolumeName = "per-node-config"

var perNodeConfigInitContainer = &corev1.Container{
	Name:  "per-node-config",
	Image: "mikefarah/yq:4",
	Resources: corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("10m"),
			"memory": resource.MustParse("16Mi"),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("100m"),
			"memory": resource.MustParse("64Mi"),
		},
	},
	Env: []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	},
	VolumeMounts: []corev1.VolumeMount{
		{
			Name:      "server-config", // volume will be created by cass-operator
			MountPath: "/config",
		},
		{
			Name:      perNodeConfigVolumeName,
			MountPath: "/per-node-config",
		},
	},
	Command: []string{
		"sh",
		"-c",
		"for src in /per-node-config/${POD_NAME}_*; do " +
			"dest=/config/`echo $src | cut -d \"_\" -f2`; " +
			"touch $dest; " +
			"yq ea '. as $item ireduce ({}; . * $item)' -i $dest $src; " +
			"echo merged $src into $dest; " +
			"done; " +
			"echo done",
	},
}

func newPerNodeConfigKey(kcKey types.NamespacedName, dcConfig *cassandra.DatacenterConfig) types.NamespacedName {
	return types.NamespacedName{
		Namespace: cassandra.DatacenterNamespace(kcKey, dcConfig),
		Name:      fmt.Sprintf("%s-%s-per-node-config", kcKey.Name, dcConfig.Meta.Name),
	}
}

func newPerNodeConfigVolume(perNodeConfigName string) corev1.Volume {
	return corev1.Volume{
		Name: perNodeConfigVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: perNodeConfigName,
				},
			},
		},
	}
}
