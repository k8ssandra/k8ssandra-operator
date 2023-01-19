package utils

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// UpdateContainer finds the container with the given name, passes it to f, and then adds it
// back to the PodTemplateSpec. The Container object is created if necessary before calling
// f. Only the Name field is initialized.
func UpdateContainer(d *appsv1.Deployment, name string, f func(c *corev1.Container)) {
	idx, found := FindContainer(d, name)
	container := &corev1.Container{}

	if !found {
		idx = len(d.Spec.Template.Spec.Containers)
		container = &corev1.Container{Name: name}
		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, *container)
	} else {
		container = &d.Spec.Template.Spec.Containers[idx]
	}

	f(container)
	d.Spec.Template.Spec.Containers[idx] = *container
}

func AddOrUpdateVolume(deployment *appsv1.Deployment, volume *corev1.Volume) {
	idx, found := FindVolume(deployment, volume.Name)
	if !found {
		// volume doesn't exist, we need to add it
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, *volume)
	} else {
		// Overwrite existing volume
		deployment.Spec.Template.Spec.Volumes[idx] = *volume
	}
}

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
