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

func ContainerHasEnvVar(container *corev1.Container, name, value string) bool {
	envVar := FindEnvVarInContainer(container, name)

	if envVar == nil {
		return false
	}

	return envVar.Value == value
}

func FindEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return &envVar
		}
	}
	return nil
}

func GetEnvVarIndex(name string, envVars []corev1.EnvVar) int {
	for i, envVar := range envVars {
		if envVar.Name == name {
			return i
		}
	}
	return -1
}
