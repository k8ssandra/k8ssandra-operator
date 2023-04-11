package secrets_webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
				"k8ssandra.io/inject-secret": `[{"secretName": "mySecret", "path": "/my/secret/path"}]`,
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
	// 3 patches for addition of init-container, volume, and volumeMount
	assert.Equal(t, len(resp.Patches), 3)
	sort.Slice(resp.Patches, func(i, j int) bool {
		return resp.Patches[i].Path < resp.Patches[j].Path
	})
	assert.Equal(t, resp.Patches[0].Operation, "add")
	assert.Equal(t, resp.Patches[0].Path, "/spec/containers/0/volumeMounts")
	assert.Equal(t, resp.Patches[1].Operation, "add")
	assert.Equal(t, resp.Patches[1].Path, "/spec/initContainers")
	assert.Equal(t, resp.Patches[2].Operation, "add")
	assert.Equal(t, resp.Patches[2].Path, "/spec/volumes")
}

func TestHandleInjectSecretNoPatch(t *testing.T) {
	p := setupSecretsInjector(t)

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"fake-annotation":                  `fake-val`,
				"k8ssandra.io/inject-secret-image": "my-custom-image", // spceifying image shouldn't trigger injection
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
	// no injection annotation, no patch
	assert.Equal(t, len(resp.Patches), 0)
}

func TestMutatePodsSingleSecret(t *testing.T) {
	secretsStr := `[{"secretName": "mySecret", "path": "/my/secret/path"}]`

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

	vm := []corev1.VolumeMount{{
		Name:      "mySecret",
		MountPath: "/my/secret/path/mySecret",
	}}

	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:        pod.Name,
			Annotations: pod.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:         pod.Spec.Containers[0].Name,
				VolumeMounts: vm,
			}},
			InitContainers: []corev1.Container{{
				Name:         defaultInjectContainerName,
				Image:        defaultInitContainerImage,
				Args:         []string{"mount", secretsStr},
				VolumeMounts: vm,
			}},
			Volumes: []corev1.Volume{{
				Name: "mySecret",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
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

func TestMutatePodsSingleSecretCustomImage(t *testing.T) {
	secretsStr := `[{"secretName": "mySecret", "path": "/my/secret/path"}]`
	image := "my-custom-image:latest"

	vm := []corev1.VolumeMount{{
		Name:      "mySecret",
		MountPath: "/my/secret/path/mySecret",
	}}

	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret":       secretsStr,
				"k8ssandra.io/inject-secret-image": image,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "test",
			}},
		},
	}

	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:        pod.Name,
			Annotations: pod.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:         pod.Spec.Containers[0].Name,
				VolumeMounts: vm,
			}},
			InitContainers: []corev1.Container{{
				Name:         defaultInjectContainerName,
				Image:        image,
				Args:         []string{"mount", secretsStr},
				VolumeMounts: vm,
			}},
			Volumes: []corev1.Volume{{
				Name: "mySecret",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
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

func TestMutatePodsMultiSecret(t *testing.T) {
	injectionAnnotation := `[{"secretName": "mySecret", "path": "/my/secret/path"},
	 {"secretName": "myOtherSecret", "path": "/my/other/secret/path"}]`

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

	vm := []corev1.VolumeMount{
		{
			Name:      "mySecret",
			MountPath: "/my/secret/path/mySecret",
		},
		{
			Name:      "myOtherSecret",
			MountPath: "/my/other/secret/path/myOtherSecret",
		},
	}

	want := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:        pod.Name,
			Annotations: pod.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:         pod.Spec.Containers[0].Name,
				VolumeMounts: vm,
			}},
			InitContainers: []corev1.Container{{
				Name:         defaultInjectContainerName,
				Image:        defaultInitContainerImage,
				Args:         []string{"mount", injectionAnnotation},
				VolumeMounts: vm,
			}},
			Volumes: []corev1.Volume{
				{
					Name: "mySecret",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "myOtherSecret",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
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
