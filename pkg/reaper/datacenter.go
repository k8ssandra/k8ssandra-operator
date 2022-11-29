package reaper

import (
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	corev1 "k8s.io/api/core/v1"
)

func AddReaperSettingsToDcConfig(reaperTemplate *reaperapi.ReaperClusterTemplate, dcConfig *cassandra.DatacenterConfig, authEnabled bool) {
	enableRemoteJmxAccess(dcConfig)
	if authEnabled && !dcConfig.ExternalSecrets {
		cassandra.AddCqlUser(reaperTemplate.CassandraUserSecretRef, dcConfig, DefaultUserSecretName(dcConfig.Cluster))
	}
}

// By default, the Cassandra process will be started with LOCAL_JMX=yes, see cassandra-env.sh. This means that the
// Cassandra process will only be accessible with JMX from localhost. However, Reaper needs remote JMX access, so we
// need to change that to LOCAL_JMX=no here. Note that this change has implications on authentication that were handled
// already in pkg/cassandra/auth.go.
func enableRemoteJmxAccess(dcConfig *cassandra.DatacenterConfig) {
	cassandra.UpdateCassandraContainer(&dcConfig.PodTemplateSpec, func(c *corev1.Container) {
		c.Env = append(c.Env, corev1.EnvVar{Name: "LOCAL_JMX", Value: "no"})
	})
}
