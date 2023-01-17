package mutation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMutatePodPatch(t *testing.T) {
	m := NewMutator(logger())
	got, err := m.MutatePodPatch(pod())
	if err != nil {
		t.Fatal(err)
	}

	p := patch()
	g := string(got)
	assert.Equal(t, p, g)
}

func BenchmarkMutatePodPatch(b *testing.B) {
	m := NewMutator(logger())
	pod := pod()

	for i := 0; i < b.N; i++ {
		_, err := m.MutatePodPatch(pod)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func pod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "lifespan",
			Annotations: map[string]string{
				"k8ssandra.io/inject-secret-mySecret": "/my/secret/path",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "lifespan",
				Image: "busybox",
			}},
		},
	}
}

func patch() string {
	patch := `[
		{"op":"add","path":"/spec/containers/0/volumeMounts","value":[
			{"mountPath":"/my/secret/path","name":"mySecret"}]},
		{"op":"add","path":"/spec/volumes","value":[
			{"name":"mySecret","secret":{"secretName":"mySecret"}}]}
]`

	patch = strings.ReplaceAll(patch, "\n", "")
	patch = strings.ReplaceAll(patch, "\t", "")
	patch = strings.ReplaceAll(patch, " ", "")

	return patch
}

func logger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger.With(zap.String("logger", "test"))
}
