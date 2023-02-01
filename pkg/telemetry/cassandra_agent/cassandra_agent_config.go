package cassandra_agent

import (
	"context"
	"path/filepath"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	agentConfigLocation = "/opt/management-api/metric-collector.yaml"
	defaultAgentConfig  = telemetryapi.CassandraAgentSpec{
		Endpoint: telemetryapi.Endpoint{
			Port:    "9000",
			Address: "127.0.0.1",
		},
	}
)

type Configurator struct {
	TelemetrySpec telemetryapi.CassandraTelemetrySpec
	Kluster       *k8ssandraapi.K8ssandraCluster
	Ctx           context.Context
	RemoteClient  client.Client
	RequeueDelay  time.Duration
	DcNamespace   string
	DcName        string
}

func (c Configurator) GetTelemetryAgentConfigMap() (*corev1.ConfigMap, error) {
	var yamlData []byte
	var err error
	if c.TelemetrySpec.Cassandra != nil {
		yamlData, err = yaml.Marshal(&c.TelemetrySpec.Cassandra)
		if err != nil {
			return &corev1.ConfigMap{}, err
		}
	} else {
		yamlData, err = yaml.Marshal(&defaultAgentConfig)
		if err != nil {
			return &corev1.ConfigMap{}, err
		}
	}

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.DcNamespace,
			Name:      c.Kluster.Name + "-" + c.DcName + "-metrics-agent-config",
		},
		Data: map[string]string{filepath.Base(agentConfigLocation): string(yamlData)},
	}
	return &cm, nil
}

func (c Configurator) ReconcileTelemetryAgentConfig(dc *cassdcapi.CassandraDatacenter) result.ReconcileResult {
	//Reconcile the agent's ConfigMap
	desiredCm, err := c.GetTelemetryAgentConfigMap()
	if err != nil {
		return result.Error(err)
	}
	cmObjectKey := types.NamespacedName{
		Name:      c.Kluster.Name + "-" + c.DcName + "-metrics-agent-config",
		Namespace: c.DcNamespace,
	}
	labels.SetManagedBy(desiredCm, cmObjectKey)
	KlKey := types.NamespacedName{
		Name:      c.Kluster.Name,
		Namespace: c.Kluster.Namespace,
	}
	partOfLabels := labels.PartOfLabels(KlKey)
	desiredCm.SetLabels(partOfLabels)
	annotations.AddHashAnnotation(desiredCm)

	currentCm := &corev1.ConfigMap{}

	err = c.RemoteClient.Get(c.Ctx, cmObjectKey, currentCm)

	if err != nil {
		if errors.IsNotFound(err) {
			if err := c.RemoteClient.Create(c.Ctx, desiredCm); err != nil {
				return result.Error(err)
			}
			return result.RequeueSoon(c.RequeueDelay)
		} else {
			return result.Error(err)
		}
	}

	if !annotations.CompareHashAnnotations(currentCm, desiredCm) {
		resourceVersion := currentCm.GetResourceVersion()
		desiredCm.DeepCopyInto(currentCm)
		currentCm.SetResourceVersion(resourceVersion)
		if err := c.RemoteClient.Update(c.Ctx, currentCm); err != nil {
			return result.Error(err)
		}
		return result.RequeueSoon(c.RequeueDelay)
	}

	c.AddStsVolumes(dc)

	return result.Done()
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
						Name: c.Kluster.Name + "-" + c.DcName + "-metrics-agent-config",
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
					MountPath: agentConfigLocation,
					SubPath:   filepath.Base(agentConfigLocation),
				}
				c.VolumeMounts = append(c.VolumeMounts, vm)
			})

	}
	return nil
}
