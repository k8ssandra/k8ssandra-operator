package cassandra

import (
	"errors"
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	corev1 "k8s.io/api/core/v1"
)

const JmxInitContainer = "jmx-credentials"

// ApplyAuth modifies the dc config depending on whether auth is enabled in the cluster or not.
func ApplyAuth(dcConfig *DatacenterConfig, authEnabled bool) error {

	if dcConfig.PodTemplateSpec == nil {
		return errors.New("PodTemplateSpec was nil, cannot add auth settings")
	}

	ApplyAuthSettings(dcConfig.CassandraConfig.CassandraYaml, authEnabled)

	// By default, the Cassandra process will be started with LOCAL_JMX=yes, see cassandra-env.sh. This means that the
	// Cassandra process will only be accessible with JMX from localhost. This is the safest and preferred setup: you
	// still can use JMX by SSH'ing into the Cassandra pod, for example to run nodetool. But some components need remote
	// JMX access (Reaper, metrics, etc.). Such components and their controllers are responsible for setting
	// LOCAL_JMX=no whenever appropriate, to enable remote JMX access.
	// However, authentication will get in the way, even if it's an orthogonal concern. Indeed, with LOCAL_JMX=yes
	// cassandra-env.sh will infer that no JMX authentication should be used
	// (com.sun.management.jmxremote.authenticate=false), whereas with LOCAL_JMX=no it will infer that authentication is
	// required (com.sun.management.jmxremote.authenticate=true). We need to change that here and enable/disable
	// authentication based on what the user specified, not what the script infers.
	jmxAuthenticateOpt := fmt.Sprintf("-Dcom.sun.management.jmxremote.authenticate=%v", authEnabled)
	addOptionIfMissing(dcConfig, jmxAuthenticateOpt)

	// When auth is enabled in the cluster, tools that use JMX to communicate with Cassandra need to authenticate as
	// well, e.g. Reaper or nodetool. Note that Reaper will use its own JMX user secret to authenticate, see
	// pkg/reaper/datacenter.go. However, we need to take care of nodetool and other generic JMX clients as well. This
	// is done by adding an init container that injects the cluster superuser credentials into the jmxremote.password
	// file in each Cassandra pod. This means that it will be possible to use the superuser credentials to authenticate
	// any JMX client.
	// TODO use Cassandra internals for JMX authentication, see https://github.com/k8ssandra/k8ssandra/issues/323
	if authEnabled {
		image := dcConfig.JmxInitContainerImage.ApplyDefaults(DefaultJmxInitImage)
		dcConfig.PodTemplateSpec.Spec.ImagePullSecrets = images.CollectPullSecrets(image)
		UpdateInitContainer(dcConfig.PodTemplateSpec, JmxInitContainer, func(c *corev1.Container) {
			c.Image = image.String()
			c.ImagePullPolicy = image.PullPolicy
			c.Env = append(c.Env,
				corev1.EnvVar{
					Name: "SUPERUSER_JMX_USERNAME",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: dcConfig.SuperuserSecretRef,
							Key:                  "username",
						},
					},
				},
				corev1.EnvVar{
					Name: "SUPERUSER_JMX_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: dcConfig.SuperuserSecretRef,
							Key:                  "password",
						},
					},
				})
			c.VolumeMounts = []corev1.VolumeMount{{
				Name:      "server-config",
				MountPath: "/config",
			}}
			c.Args = []string{
				"/bin/sh",
				"-c",
				"echo \"$SUPERUSER_JMX_USERNAME $SUPERUSER_JMX_PASSWORD\" >> /config/jmxremote.password",
			}
		})
	}
	return nil
}

// ApplyAuthSettings modifies the given config and applies defaults for authenticator, authorizer and role manager,
// depending on whether auth is enabled or not, and only if these settings are empty in the input config. It also
// sets the com.sun.management.jmxremote.authenticate JVM option to the appropriate value.
func ApplyAuthSettings(config unstructured.Unstructured, authEnabled bool) {
	if authEnabled {
		config.PutIfAbsent("authenticator", "PasswordAuthenticator")
		config.PutIfAbsent("authorizer", "CassandraAuthorizer")
	} else {
		config.PutIfAbsent("authenticator", "AllowAllAuthenticator")
		config.PutIfAbsent("authorizer", "AllowAllAuthorizer")
	}
	config.PutIfAbsent("role_manager", "CassandraRoleManager")
}

// If auth is enabled in this cluster, we need to allow components to access the cluster through CQL. This is done by
// declaring a Cassandra user whose credentials are pulled from CassandraUserSecretRef.
func AddCqlUser(cassandraUserSecretRef corev1.LocalObjectReference, dcConfig *DatacenterConfig, cassandraUserSecretName string) {
	if cassandraUserSecretRef.Name == "" {
		cassandraUserSecretRef.Name = cassandraUserSecretName
	}
	dcConfig.Users = append(dcConfig.Users, cassdcapi.CassandraUser{
		SecretName: cassandraUserSecretRef.Name,
		Superuser:  true,
	})
}
