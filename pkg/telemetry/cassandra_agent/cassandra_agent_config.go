package cassandra_agent

import (
	"context"
	"path/filepath"
	"time"

	"github.com/adutra/goalesce"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	agentConfigLocation = "/opt/management-api/configs/metrics-collector.yaml"
	defaultAgentConfig  = telemetryapi.CassandraAgentSpec{
		Relabels: []promapi.RelabelConfig{
			{
				SourceLabels: []string{"table"},
				Regex:        ".+",
				TargetLabel:  "should_drop",
				Replacement:  "true",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_live_ss_table_count",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_live_disk_space_used",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_memtable.*",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_all_memtables.*",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_compaction_bytes_written",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_pending_compactions",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_read_.*",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_write_.*",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_range.*",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_coordinator_.*",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"__name__"},
				Regex:        "org_apache_cassandra_metrics_table_dropped_mutations",
				TargetLabel:  "should_drop",
				Replacement:  "false",
			},
			{
				SourceLabels: []string{"should_drop"},
				Regex:        "true",
				Action:       "drop",
			},
		},
	}
)

type Configurator struct {
	TelemetrySpec telemetryapi.TelemetrySpec
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
		mergedSpec := goalesce.MustDeepMerge(&defaultAgentConfig, c.TelemetrySpec.Cassandra)
		yamlData, err = yaml.Marshal(&mergedSpec)
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
			Name:      c.configMapName(),
		},
		Data: map[string]string{filepath.Base(agentConfigLocation): string(yamlData)},
	}
	return &cm, nil
}

func (c Configurator) configMapName() string {
	return cassdcapi.CleanupForKubernetes(c.Kluster.CassClusterName() + "-" + c.DcName + "-metrics-agent-config")
}

func (c Configurator) ReconcileTelemetryAgentConfig(dc *cassdcapi.CassandraDatacenter) result.ReconcileResult {
	//Reconcile the agent's ConfigMap
	desiredCm, err := c.GetTelemetryAgentConfigMap()
	if err != nil {
		return result.Error(err)
	}
	KlKey := types.NamespacedName{
		Name:      c.Kluster.Name,
		Namespace: c.Kluster.Namespace,
	}
	desiredCm.SetLabels(labels.CleanedUpByLabels(KlKey))

	recRes := reconciliation.ReconcileObject(c.Ctx, c.RemoteClient, c.RequeueDelay, *desiredCm)
	switch {
	case recRes.IsError():
		return recRes
	case recRes.IsRequeue():
		return recRes
	}

	if err := c.AddVolumeSource(dc); err != nil {
		return result.Error(err)
	}

	return result.Done()
}

func (c Configurator) AddVolumeSource(dc *cassdcapi.CassandraDatacenter) error {
	dc.Spec.StorageConfig.AdditionalVolumes = append(dc.Spec.StorageConfig.AdditionalVolumes, cassdcapi.AdditionalVolumes{
		Name:      "metrics-agent-config",
		MountPath: "/opt/management-api/configs",
		VolumeSource: &corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				Items: []corev1.KeyToPath{
					{
						Key:  filepath.Base(agentConfigLocation),
						Path: filepath.Base(agentConfigLocation),
					},
				},
				LocalObjectReference: corev1.LocalObjectReference{
					Name: c.configMapName(),
				},
			},
		},
	})

	return nil
}
