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

func TestHandleInjectSecretSuccess(t *testing.T) {
	p := setupSecretsInjector(t)

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": `[{"name": "mySecret", "path": "/my/secret/path"}]`,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
			}},
		},
	}
	req := createRequest(t, pod)

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

func TestHandleInjectSecretNoPatch(t *testing.T) {
	p := setupSecretsInjector(t)

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
	req := createRequest(t, pod)

	resp := p.Handle(context.Background(), req)
	assert.Equal(t, true, resp.AdmissionResponse.Allowed)
	// no injection annotation, no patch
	assert.Equal(t, len(resp.Patches), 0)
}

func TestMutatePodsSingleSecret(t *testing.T) {
	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": `[{"name": "mySecret", "path": "/my/secret/path"}]`,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "mySecret-secret",
					MountPath: "/my/secret/path",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "mySecret-secret",
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
				"k8ssandra.io/inject-secret": `[{"name": "mySecret", "path": "/my/secret/path"}]`,
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
	injectionAnnotation := `[{"name": "mySecret", "path": "/my/secret/path"},
	 {"name": "myOtherSecret", "path": "/my/other/secret/path"}]`

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
						Name:      "mySecret-secret",
						MountPath: "/my/secret/path",
					},
					{
						Name:      "myOtherSecret-secret",
						MountPath: "/my/other/secret/path",
					},
				},
			}},
			Volumes: []corev1.Volume{
				{
					Name: "mySecret-secret",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "mySecret",
						},
					},
				},
				{
					Name: "myOtherSecret-secret",
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

func setupSecretsInjector(t *testing.T) *podSecretsInjector {
	p := &podSecretsInjector{}
	d, err := admission.NewDecoder(scheme.Scheme)
	if err != nil {
		t.Fatal(err)
	}
	p.InjectDecoder(d)
	return p
}

func createRequest(t *testing.T, pod *corev1.Pod) webhook.AdmissionRequest {
	pBytes, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}

	return webhook.AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
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
}

func TestMutatePodsExpandKey(t *testing.T) {
	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "cluster2-dc2-r1-sts-0",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": `[{"name": "mySecret-${POD_NAME}-${POD_NAMESPACE}-${POD_ORDINAL}", "path": "/my/secret/path"}]`,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "mySecret-cluster2-dc2-r1-sts-0-test-ns-0-secret",
					MountPath: "/my/secret/path",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "mySecret-cluster2-dc2-r1-sts-0-test-ns-0-secret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "mySecret-cluster2-dc2-r1-sts-0-test-ns-0",
					},
				},
			}},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      "cluster2-dc2-r1-sts-0",
			Namespace: "test-ns",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": `[{"name": "mySecret-${POD_NAME}-${POD_NAMESPACE}-${POD_ORDINAL}", "path": "/my/secret/path"}]`,
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

func TestMutatePodsSpecifyContainer(t *testing.T) {
	injectionAnnotation := `[{"name": "mySecret", "path": "/my/secret/path", "containers": ["test"]}]`

	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret": injectionAnnotation,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "mySecret-secret",
							MountPath: "/my/secret/path",
						},
					},
				},
				{
					Name: "other",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "mySecret-secret",
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
				"k8ssandra.io/inject-secret": injectionAnnotation,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test",
				},
				{
					Name: "other",
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
