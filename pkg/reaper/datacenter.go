package reaper

import (
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

const jmxAuthDisabledOption = "-Dcom.sun.management.jmxremote.authenticate=false"

func AddReaperSettingsToDcConfig(reaperTemplate *reaperapi.ReaperClusterTemplate, dcConfig *cassandra.DatacenterConfig, authEnabled bool) {
	enableRemoteJmxAccess(dcConfig, authEnabled)
	if authEnabled && !dcConfig.ExternalSecrets {
		cassandra.AddCqlUser(reaperTemplate.CassandraUserSecretRef, dcConfig, DefaultUserSecretName(dcConfig.Cluster))
	}
}

func enableRemoteJmxAccess(dcConfig *cassandra.DatacenterConfig, authEnabled bool) {
	cassandra.UpdateCassandraContainer(&dcConfig.PodTemplateSpec, func(c *corev1.Container) {

		// By default, the Cassandra process will be started with LOCAL_JMX=yes, see cassandra-env.sh. This means that
		// the Cassandra process will only be accessible with JMX from localhost. Reaper needs remote JMX access, so we
		// need to change that to LOCAL_JMX=no here.
		c.Env = append(c.Env, corev1.EnvVar{Name: "LOCAL_JMX", Value: "no"})

		// However, setting LOCAL_JMX=no also enables JMX authentication. If auth is disabled on the K8ssandraCluster,
		// we don't want that so rectify it with a system property:
		if !authEnabled {
			if !utils.SliceContains(dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, jmxAuthDisabledOption) {
				dcConfig.CassandraConfig.JvmOptions.AdditionalOptions = append(
					[]string{jmxAuthDisabledOption},
					dcConfig.CassandraConfig.JvmOptions.AdditionalOptions...,
				)
			}
		}
	})
}
