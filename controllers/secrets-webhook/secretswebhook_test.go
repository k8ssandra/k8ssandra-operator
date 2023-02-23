package secrets_webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestHandleSinceSecretSuccess(t *testing.T) {
	p := &podSecretsInjector{}
	d, err := admission.NewDecoder(scheme.Scheme)
	if err != nil {
		t.Fatal(err)
	}
	p.InjectDecoder(d)

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
	pBytes, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}

	req := webhook.AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
		UID:       "test123",
		Name:      "foo",
		Namespace: "bar",
		Resource: metav1.GroupVersionResource{
			Version:  "v1",
			Resource: "pods",
		},
		Operation: "CREATE",
		Object:    runtime.RawExtension{Raw: pBytes},
	}}

	resp := p.Handle(context.Background(), req)
	fmt.Println(fmt.Sprintf("%v", resp))
	assert.Equal(t, true, resp.AdmissionResponse.Allowed)
	// 2 patches for addition of volume and volumeMount
	assert.Equal(t, len(resp.Patches), 2)
	assert.Equal(t, resp.Patches[0].Operation, "add")
	assert.Equal(t, resp.Patches[0].Path, "/spec/volumes")
	assert.Equal(t, resp.Patches[1].Operation, "add")
	assert.Equal(t, resp.Patches[1].Path, "/spec/containers/0/volumeMounts")
}

func TestHandleSinceSecretNoPatch(t *testing.T) {
	p := &podSecretsInjector{}
	d, err := admission.NewDecoder(scheme.Scheme)
	if err != nil {
		t.Fatal(err)
	}
	p.InjectDecoder(d)

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"fake-annotation": `fake-val`,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
			}},
		},
	}
	pBytes, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}

	req := webhook.AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
		UID:       "test123",
		Name:      "foo",
		Namespace: "bar",
		Resource: metav1.GroupVersionResource{
			Version:  "v1",
			Resource: "pods",
		},
		Operation: "CREATE",
		Object:    runtime.RawExtension{Raw: pBytes},
	}}

	resp := p.Handle(context.Background(), req)
	fmt.Println(fmt.Sprintf("%v", resp))
	assert.Equal(t, true, resp.AdmissionResponse.Allowed)
	// no injection annotation, no patch
	assert.Equal(t, len(resp.Patches), 0)
}

func TestMutatePodsSingleSecret(t *testing.T) {
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

	ctx := context.Background()
	err := p.mutatePods(ctx, pod, log.FromContext(ctx))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, want, pod)
}

func TestMutatePodsMutliSecret(t *testing.T) {
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

	ctx := context.Background()
	err := p.mutatePods(ctx, pod, log.FromContext(ctx))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, want, pod)
}
