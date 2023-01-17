package mutation

import (
	"encoding/json"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// e.g. k8ssandra.io/inject-secret=[{secretName=my-secret, path=/etc/credentials/cassandra}]
const secretInjectionAnnotation = "k8ssandra.io/inject-secret"

type SecretInjection struct {
	SecretName string `json:"secretName"`
	Path       string `json:"path"`
}

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

	secretsStr := pod.ObjectMeta.Annotations[secretInjectionAnnotation]
	if len(secretsStr) == 0 {
		se.Logger.Info("no secret annotation exists")
		return mpod, nil
	}

	var secrets []SecretInjection
	if err := json.Unmarshal([]byte(secretsStr), &secrets); err != nil {
		se.Logger.Error("unable to unmarhsal secrets annotation", zap.String("annotation", secretsStr))
		return mpod, err
	}

	for _, secret := range secrets {
		// get secret name from injection annotation
		secretName := secret.SecretName
		mountPath := secret.Path
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
