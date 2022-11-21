package reaper

import (
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	corev1 "k8s.io/api/core/v1"
)

func AddReaperSettingsToDcConfig(reaperTemplate *reaperapi.ReaperClusterTemplate, dcConfig *cassandra.DatacenterConfig, authEnabled bool) {
	if dcConfig.PodTemplateSpec == nil {
		dcConfig.PodTemplateSpec = &corev1.PodTemplateSpec{}
	}
	enableRemoteJmxAccess(dcConfig)
	if authEnabled && !reaperTemplate.UseExternalSecrets() {
		cassandra.AddCqlUser(reaperTemplate.CassandraUserSecretRef, dcConfig, DefaultUserSecretName(dcConfig.Cluster))
		enableJmxAuth(reaperTemplate, dcConfig)
	}
}

// By default, the Cassandra process will be started with LOCAL_JMX=yes, see cassandra-env.sh. This means that the
// Cassandra process will only be accessible with JMX from localhost. However, Reaper needs remote JMX access, so we
// need to change that to LOCAL_JMX=no here. Note that this change has implications on authentication that were handled
// already in pkg/cassandra/auth.go.
func enableRemoteJmxAccess(dcConfig *cassandra.DatacenterConfig) {
	cassandra.UpdateCassandraContainer(dcConfig.PodTemplateSpec, func(c *corev1.Container) {
		c.Env = append(c.Env, corev1.EnvVar{Name: "LOCAL_JMX", Value: "no"})
	})
}

// If auth is enabled in this cluster, then a JMX init container is already present in the pod definition, see
// pkg/cassandra/auth.go. Here we want to modify this init container to append the Reaper JMX credentials to the
// jmxremote.password file.
func enableJmxAuth(reaperTemplate *reaperapi.ReaperClusterTemplate, dcConfig *cassandra.DatacenterConfig) {
	jmxUserSecretRef := reaperTemplate.JmxUserSecretRef
	if jmxUserSecretRef.Name == "" {
		jmxUserSecretRef.Name = DefaultJmxUserSecretName(dcConfig.Cluster)
	}
	cassandra.UpdateInitContainer(dcConfig.PodTemplateSpec, cassandra.JmxInitContainer, func(c *corev1.Container) {
		c.Env = append(c.Env,
			corev1.EnvVar{
				Name: "REAPER_JMX_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: jmxUserSecretRef,
						Key:                  "username",
					},
				},
			},
			corev1.EnvVar{
				Name: "REAPER_JMX_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: jmxUserSecretRef,
						Key:                  "password",
					},
				},
			},
		)
		c.Args[2] = c.Args[2] + " && echo \"$REAPER_JMX_USERNAME $REAPER_JMX_PASSWORD\" >> /config/jmxremote.password"
	})
}
