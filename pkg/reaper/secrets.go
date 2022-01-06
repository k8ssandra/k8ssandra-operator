package reaper

import (
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	corev1 "k8s.io/api/core/v1"
)

const (
	jmxAuthEnvPasswordName  = "REAPER_JMX_AUTH_PASSWORD"
	jmxAuthEnvUsernameName  = "REAPER_JMX_AUTH_USERNAME"
	cassAuthEnvPasswordName = "REAPER_CASS_AUTH_PASSWORD"
	cassAuthEnvUsernameName = "REAPER_CASS_AUTH_USERNAME"
	envVarEnableCassAuth    = "REAPER_CASS_AUTH_ENABLED"
	secretUsernameName      = "username"
	secretPasswordName      = "password"
)

var EnableCassAuthVar = &corev1.EnvVar{
	Name:  envVarEnableCassAuth,
	Value: "true",
}

// DefaultUserSecretName generates a name for the Reaper CQL user, that is derived from the Cassandra cluster name.
func DefaultUserSecretName(clusterName string) string {
	return fmt.Sprintf("%v-reaper", secret.SanitizeSecretName(clusterName))
}

// DefaultJmxUserSecretName generates a name for the Reaper JMX user, that is derived from the Cassandra cluster name.
func DefaultJmxUserSecretName(clusterName string) string {
	return fmt.Sprintf("%v-reaper-jmx", secret.SanitizeSecretName(clusterName))
}

func GetCassandraAuthEnvironmentVars(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error) {
	return secretToEnvVars(secret, cassAuthEnvUsernameName, cassAuthEnvPasswordName)
}

func GetJmxAuthEnvironmentVars(secret *corev1.Secret) (*corev1.EnvVar, *corev1.EnvVar, error) {
	return secretToEnvVars(secret, jmxAuthEnvUsernameName, jmxAuthEnvPasswordName)
}

func secretToEnvVars(secret *corev1.Secret, envUsernameParam, envPasswordParam string) (*corev1.EnvVar, *corev1.EnvVar, error) {
	if _, ok := secret.Data[secretUsernameName]; !ok {
		return nil, nil, fmt.Errorf("username key not found in jmx auth secret %s", secret.Name)
	}

	if _, ok := secret.Data[secretPasswordName]; !ok {
		return nil, nil, fmt.Errorf("password key not found in jmx auth secret %s", secret.Name)
	}

	usernameEnvVar := corev1.EnvVar{
		Name: envUsernameParam,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secret.Name,
				},
				Key: "username",
			},
		},
	}

	passwordEnvVar := corev1.EnvVar{
		Name: envPasswordParam,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secret.Name,
				},
				Key: "password",
			},
		},
	}

	return &usernameEnvVar, &passwordEnvVar, nil
}
