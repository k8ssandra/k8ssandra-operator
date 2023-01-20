package cassandra_agent

import (
	"context"
	"path/filepath"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	agentConfigLocation = "/config/metric-collector.yaml"
)

type Configurator struct {
	TelemetrySpec telemetryapi.TelemetrySpec
	Kluster       *k8ssandraapi.K8ssandraCluster
	Ctx           context.Context
	RemoteClient  client.Client
}

func (c Configurator) GetTelemetryAgentConfigMap() (corev1.ConfigMap, error) {
	yamlData, err := yaml.Marshal(&c.TelemetrySpec.Cassandra)
	if err != nil {
		return corev1.ConfigMap{}, err
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.Kluster.Namespace,
			Name:      c.Kluster.Name + "metrics-agent-config",
		},
		Data: map[string]string{filepath.Base(agentConfigLocation): string(yamlData)},
	}
	return cm, nil
}

func (c Configurator) ReconcileTelemetryAgentConfig(dc *cassdcapi.CassandraDatacenter) error {
	cm, err := c.GetTelemetryAgentConfigMap()
	if err != nil {
		return err
	}
	if err := c.RemoteClient.Create(c.Ctx, &cm); err != nil {
		return err
	}
	c.AddStsVolumes(dc)

	return nil
}

func (c Configurator) AddStsVolumes(dc *cassdcapi.CassandraDatacenter) error {
	if dc.Spec.PodTemplateSpec == nil {
		dc.Spec.PodTemplateSpec = &corev1.PodTemplateSpec{}
	}
	_, found := cassandra.FindVolume(dc.Spec.PodTemplateSpec, "metrics-agent-config")
	if !found {
		v := corev1.Volume{
			Name: "metrics-agent-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					Items: []corev1.KeyToPath{
						{
							Key:  filepath.Base(agentConfigLocation),
							Path: filepath.Base(agentConfigLocation),
						},
					},
					LocalObjectReference: corev1.LocalObjectReference{
						Name: c.Kluster.Name + "metrics-agent-config",
					},
				},
			},
		}
		// We don't check that the volume mount exists before appending because we assume that the existence of the volume
		// is a sufficient signal that reconciliation has run.
		dc.Spec.PodTemplateSpec.Spec.Volumes = append(dc.Spec.PodTemplateSpec.Spec.Volumes, v)
		cassandra.UpdateCassandraContainer(
			dc.Spec.PodTemplateSpec,
			func(c *corev1.Container) {
				vm := corev1.VolumeMount{
					Name:      "metrics-agent-config",
					MountPath: filepath.Base(filepath.Dir(agentConfigLocation)),
				}
				c.VolumeMounts = append(c.VolumeMounts, vm)
			})

	}
	return nil
}

// Do we need this? If the agent is always enabled I guess we must always want to create the ConfigMap?
func (c Configurator) RemoveStsVolumes(dc *cassdcapi.CassandraDatacenter) error {
	volumeIdx, found := cassandra.FindVolume(dc.Spec.PodTemplateSpec, "metrics-agent-config")
	if found {
		dc.Spec.PodTemplateSpec.Spec.Volumes = append(
			dc.Spec.PodTemplateSpec.Spec.Volumes[:volumeIdx],
			dc.Spec.PodTemplateSpec.Spec.Volumes[(volumeIdx+1):]...,
		)
	}
	cassandraContainerIdx, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "cassandra")
	if found {
		mountIdx := -1
		found := false
		for i, mount := range dc.Spec.PodTemplateSpec.Spec.Containers[cassandraContainerIdx].VolumeMounts {
			if mount.Name == "metrics-agent-config" {
				mountIdx = i
				found = true
			}
			if found {
				dc.Spec.PodTemplateSpec.Spec.Containers[cassandraContainerIdx].VolumeMounts = append(
					dc.Spec.PodTemplateSpec.Spec.Containers[cassandraContainerIdx].VolumeMounts[:mountIdx],
					dc.Spec.PodTemplateSpec.Spec.Containers[cassandraContainerIdx].VolumeMounts[(mountIdx+1):]...)
			}
		}
	}
	return nil

}
