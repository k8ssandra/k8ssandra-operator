package utils

import (
	corev1 "k8s.io/api/core/v1"
)

func FindEnvVarInContainer(container *corev1.Container, name string) *corev1.EnvVar {
	for _, v := range container.Env {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func FindEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return &envVar
		}
	}
	return nil
}
