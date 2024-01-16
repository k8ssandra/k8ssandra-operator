package reaper

import (
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"

	corev1 "k8s.io/api/core/v1"
)

const (
	jmxAuthEnvPasswordName  = "REAPER_JMX_AUTH_PASSWORD"
	jmxAuthEnvUsernameName  = "REAPER_JMX_AUTH_USERNAME"
	cassAuthEnvPasswordName = "REAPER_CASS_AUTH_PASSWORD"
	cassAuthEnvUsernameName = "REAPER_CASS_AUTH_USERNAME"
	uiAuthEnvPasswordName   = "REAPER_AUTH_PASSWORD"
	uiAuthEnvUsernameName   = "REAPER_AUTH_USER"
	envVarEnableCassAuth    = "REAPER_CASS_AUTH_ENABLED"
	envVarEnableAuth        = "REAPER_AUTH_ENABLED"
	secretUsernameName      = "username"
	secretPasswordName      = "password"
)

var EnableCassAuthVar = &corev1.EnvVar{
	Name:  envVarEnableCassAuth,
	Value: "true",
}

var EnableAuthVar = &corev1.EnvVar{
	Name:  envVarEnableAuth,
	Value: "true",
}

var DisableAuthVar = &corev1.EnvVar{
	Name:  envVarEnableAuth,
	Value: "false",
}

// DefaultUserSecretName generates a name for the Reaper CQL user, that is derived from the Cassandra cluster name.
func DefaultUserSecretName(clusterName string) string {
	return fmt.Sprintf("%v-reaper", cassdcapi.CleanupForKubernetes(clusterName))
}

func DefaultUiSecretName(clusterName string) string {
	return fmt.Sprintf("%v-reaper-ui", cassdcapi.CleanupForKubernetes(clusterName))
}

func GetAuthEnvironmentVars(secret *corev1.Secret, authType string) (*corev1.EnvVar, *corev1.EnvVar, error) {
	switch authType {
	case "cql":
		return secretToEnvVars(secret, cassAuthEnvUsernameName, cassAuthEnvPasswordName)
	case "jmx":
		return secretToEnvVars(secret, jmxAuthEnvUsernameName, jmxAuthEnvPasswordName)
	case "ui":
		return secretToEnvVars(secret, uiAuthEnvUsernameName, uiAuthEnvPasswordName)
	default:
		return nil, nil, fmt.Errorf("unsupported auth type %s", authType)
	}
}

func secretToEnvVars(secret *corev1.Secret, envUsernameParam, envPasswordParam string) (*corev1.EnvVar, *corev1.EnvVar, error) {
	if _, ok := secret.Data[secretUsernameName]; !ok {
		return nil, nil, fmt.Errorf("username key not found in auth secret %s", secret.Name)
	}

	if _, ok := secret.Data[secretPasswordName]; !ok {
		return nil, nil, fmt.Errorf("password key not found in auth secret %s", secret.Name)
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
