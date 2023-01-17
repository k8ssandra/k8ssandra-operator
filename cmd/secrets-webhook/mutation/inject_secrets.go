package mutation

import (
	"sort"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// annotations should follow the format prefix-{secret}={mountPath}
// e.g. k8ssandra.io/inject-secret-cassandraSuperuser=/etc/credentials/cassandra
const secretNameAnnotationPrefix = "k8ssandra.io/inject-secret-"

// injectEnv is a container for the mutation injecting secretss
type injectSecrets struct {
	Logger *zap.Logger
}

// injectSecrets implements the podMutator interface
var _ podMutator = (*injectSecrets)(nil)

// Name returns the struct name
func (se injectSecrets) Name() string {
	return "inject_secrets"
}

// Mutate returns a new mutated pod according to set secret rules
func (se injectSecrets) Mutate(pod *corev1.Pod) (*corev1.Pod, error) {
	se.Logger = se.Logger.With(zap.String("mutation", se.Name()))
	mpod := pod.DeepCopy()

	secretKeys := getSecretInjectionAnnotations(pod.ObjectMeta.Annotations)
	if len(secretKeys) == 0 {
		se.Logger.Info("no secret annotations exist")
		return mpod, nil
	}

	for _, key := range secretKeys {
		// get secret name from annotation, which is in the format
		// inject-secrets-template-{secret}
		secretName := key[len(secretNameAnnotationPrefix):]
		mountPath := pod.ObjectMeta.Annotations[key]
		se.Logger.Debug("creating volume and volume mount for secret",
			zap.String("secret", secretName),
			zap.String("secret path", mountPath),
		)

		volume := corev1.Volume{
			Name: secretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		injectVolume(mpod, volume)

		volumeMount := corev1.VolumeMount{
			Name:      secretName,
			MountPath: mountPath,
		}
		injectVolumeMount(mpod, volumeMount)
		se.Logger.Debug("added volume and volumeMount to podSpec",
			zap.String("secret", secretName),
			zap.String("secret path", mountPath),
		)
	}

	return mpod, nil
}

func injectVolume(pod *corev1.Pod, volume corev1.Volume) {
	if !hasVolume(pod.Spec.Volumes, volume) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}
}

func hasVolume(volumes []corev1.Volume, volume corev1.Volume) bool {
	for _, v := range volumes {
		if v.Name == volume.Name {
			return true
		}
	}
	return false
}

func injectVolumeMount(pod *corev1.Pod, volumeMount corev1.VolumeMount) {
	for i, container := range pod.Spec.Containers {
		if !hasVolumeMount(container, volumeMount) {
			pod.Spec.Containers[i].VolumeMounts = append(container.VolumeMounts, volumeMount)
		}
	}
	for i, container := range pod.Spec.InitContainers {
		if !hasVolumeMount(container, volumeMount) {
			pod.Spec.Containers[i].VolumeMounts = append(container.VolumeMounts, volumeMount)
		}
	}
}

// hasVolumeMoujt returns true if environment variable exists false otherwise
func hasVolumeMount(container corev1.Container, volumeMount corev1.VolumeMount) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == volumeMount.Name {
			return true
		}
	}
	return false
}

// getSecretInjectionAnnotations retrieves the annotations that use the
// secretNameAnnotation prefix and returns them in a sorted list
func getSecretInjectionAnnotations(annotations map[string]string) []string {
	// verify there is at least one injection annotation
	var secretKeys []string
	for a, _ := range annotations {
		if strings.Contains(a, secretNameAnnotationPrefix) {
			secretKeys = append(secretKeys, a)
		}
	}

	sort.Strings(secretKeys)
	return secretKeys
}
