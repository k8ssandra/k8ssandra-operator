package secrets_webhook

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"net/http"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-v1-pod-secrets-inject,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kb.io,admissionReviewVersions=v1,sideEffects=None

func SetupSecretsInjectorWebhook(mgr ctrl.Manager) {
	mgr.GetWebhookServer().Register("/mutate-v1-pod-secrets-inject", &webhook.Admission{Handler: &podSecretsInjector{Client: mgr.GetClient()}})
}

// podSecretsInjector is an admission handler that mutates pod manifests
// to include mechanisms for mounting secrets
type podSecretsInjector struct {
	Client  client.Client
	decoder *admission.Decoder
}

// podSecretsInjector Implements admission.Handler.
var _ admission.Handler = &podSecretsInjector{}

// InjectDecoder injects the decoder into the podSecretsInjector
func (p *podSecretsInjector) InjectDecoder(d *admission.Decoder) error {
	p.decoder = d
	return nil
}

func (p *podSecretsInjector) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("podSecretsInjector", req.Namespace)

	pod := &corev1.Pod{}
	err := p.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	copy := pod.DeepCopy()

	err = p.mutatePods(ctx, copy, logger)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledPod, err := json.Marshal(copy)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// e.g. k8ssandra.io/inject-secret: '[{ "secretName": "test-secret", "path": "/etc/test/test-secret" }]'
const secretInjectionAnnotation = "k8ssandra.io/inject-secret"

type SecretInjection struct {
	SecretName string `json:"secretName"`
	Path       string `json:"path"`
}

// mutatePods injects the secret mounting configuration into the pod
func (p *podSecretsInjector) mutatePods(ctx context.Context, pod *corev1.Pod, logger logr.Logger) error {
	if pod.Annotations == nil {
		logger.Info("no annotations exist", "podName", pod.Name, "namespace", pod.Namespace)
		return nil
	}

	secretsStr := pod.Annotations[secretInjectionAnnotation]
	if len(secretsStr) == 0 {
		logger.Info("no secret annotation exists", "podName", pod.Name, "namespace", pod.Namespace)
		return nil
	}

	var secrets []SecretInjection
	if err := json.Unmarshal([]byte(secretsStr), &secrets); err != nil {
		logger.Error(err, "unable to unmarhsal secrets annotation",
			"annotation", secretsStr,
			"podName", pod.Name,
			"namespace", pod.Namespace,
		)
		return err
	}

	for _, secret := range secrets {
		// get secret name from injection annotation
		secretName := secret.SecretName
		mountPath := secret.Path
		logger.Info("creating volume and volume mount for secret",
			"secret", secretName,
			"secret path", mountPath,
			"podName", pod.Name,
			"namespace", pod.Namespace,
		)

		volume := corev1.Volume{
			Name: secretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		injectVolume(pod, volume)

		volumeMount := corev1.VolumeMount{
			Name:      secretName,
			MountPath: mountPath,
		}
		injectVolumeMount(pod, volumeMount)
		logger.Info("added volume and volumeMount to podSpec",
			"secret", secretName,
			"secret path", mountPath,
			"podName", pod.Name,
			"namespace", pod.Namespace,
		)
	}

	return nil
}

// injectVolume attaches a volume to the pod spec
func injectVolume(pod *corev1.Pod, volume corev1.Volume) {
	if !hasVolume(pod.Spec.Volumes, volume) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}
}

// injectVolumeMount attaches a volumeMount to all containers in the pod spec
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

// hasVolume returns true if volume exists, false otherwise
func hasVolume(volumes []corev1.Volume, volume corev1.Volume) bool {
	for _, v := range volumes {
		if v.Name == volume.Name {
			return true
		}
	}
	return false
}

// hasVolumeMount returns true if volume mount exists, false otherwise
func hasVolumeMount(container corev1.Container, volumeMount corev1.VolumeMount) bool {
	for _, mount := range container.VolumeMounts {
		if mount.Name == volumeMount.Name {
			return true
		}
	}
	return false
}
