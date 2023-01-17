package mutation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInjectSecretsMutate(t *testing.T) {
	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret-mySecret": "/my/secret/path",
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
				"k8ssandra.io/inject-secret-mySecret": "/my/secret/path",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
			}},
		},
	}

	got, err := injectSecrets{Logger: logger()}.Mutate(pod)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, want, got)
}

func TestInjectSecretsMultiMutate(t *testing.T) {
	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret-mySecret":      "/my/secret/path",
				"k8ssandra.io/inject-secret-myOtherSecret": "/my/other/secret/path",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "myOtherSecret",
						MountPath: "/my/other/secret/path",
					},
					{
						Name:      "mySecret",
						MountPath: "/my/secret/path",
					},
				},
			}},
			Volumes: []corev1.Volume{
				{
					Name: "myOtherSecret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "myOtherSecret",
						},
					},
				},
				{
					Name: "mySecret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "mySecret",
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
				"k8ssandra.io/inject-secret-mySecret":      "/my/secret/path",
				"k8ssandra.io/inject-secret-myOtherSecret": "/my/other/secret/path",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
			}},
		},
	}

	got, err := injectSecrets{Logger: logger()}.Mutate(pod)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, want, got)
}
