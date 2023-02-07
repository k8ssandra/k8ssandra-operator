package secrets_webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestInjectSecretsc(t *testing.T) {
	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": `[{"secretName": "mySecret", "path": "/my/secret/path"}]`,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "mySecret",
					MountPath: "/my/secret/path",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "mySecret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "mySecret",
					},
				},
			}},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": `[{"secretName": "mySecret", "path": "/my/secret/path"}]`,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
			}},
		},
	}

	p := &podSecretsInjector{}

	ctx := context.TODO()
	err := p.mutatePods(ctx, pod, log.FromContext(ctx))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, want, pod)
}

func TestInjectSecretsMultiMutate(t *testing.T) {
	injectionAnnotation := `[{"secretName": "mySecret", "path": "/my/secret/path"},
	 {"secretName": "myOtherSecret", "path": "/my/other/secret/path"}]`

	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": injectionAnnotation,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "mySecret",
						MountPath: "/my/secret/path",
					},
					{
						Name:      "myOtherSecret",
						MountPath: "/my/other/secret/path",
					},
				},
			}},
			Volumes: []corev1.Volume{
				{
					Name: "mySecret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "mySecret",
						},
					},
				},
				{
					Name: "myOtherSecret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "myOtherSecret",
						},
					},
				},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": injectionAnnotation,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
			}},
		},
	}

	p := &podSecretsInjector{}

	ctx := context.TODO()
	err := p.mutatePods(ctx, pod, log.FromContext(ctx))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, want, pod)
}
