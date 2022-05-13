package shared

import (
	corev1 "k8s.io/api/core/v1"
)

// An enum of the possible modes for medusa backups
type BackupType string

const (
	FullBackup         BackupType = "full"
	DifferentialBackup BackupType = "differential"
)

const (
	BackupSidecarPort = 50051
	BackupSidecarName = "medusa"
)

func IsMedusaDeployed(pods []corev1.Pod) bool {
	for _, pod := range pods {
		if !hasMedusaSidecar(&pod) {
			return false
		}
	}
	return true
}

func hasMedusaSidecar(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == BackupSidecarName {
			return true
		}
	}
	return false
}
