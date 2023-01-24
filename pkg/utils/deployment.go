package utils

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func FindContainer(deployment *appsv1.Deployment, name string) (int, bool) {
	if deployment != nil {
		for i, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == name {
				return i, true
			}
		}
	}
	return -1, false
}

func FindAndGetContainer(deployment *appsv1.Deployment, name string) *corev1.Container {
	idx, found := FindContainer(deployment, name)
	if found {
		return &deployment.Spec.Template.Spec.Containers[idx]
	}
	return nil
}

func FindVolumeMount(container *corev1.Container, name string) *corev1.VolumeMount {
	for _, v := range container.VolumeMounts {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func FindVolume(deployment *appsv1.Deployment, name string) (int, bool) {
	if deployment != nil {
		for i, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.Name == name {
				return i, true
			}
		}
	}
	return -1, false

}

func FindAndGetVolume(deployment *appsv1.Deployment, name string) *corev1.Volume {
	idx, found := FindVolume(deployment, name)
	if found {
		return &deployment.Spec.Template.Spec.Volumes[idx]
	}
	return nil
}
