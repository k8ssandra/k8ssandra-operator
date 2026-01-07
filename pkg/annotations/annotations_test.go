package annotations

import (
	"testing"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	k8ssandrameta "github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddCommonAnnotationsExistingAnnotations(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				Meta: k8ssandrameta.CassandraClusterMeta{
					CommonAnnotations: map[string]string{
						"newAnnotation":         "newValue",
						"overwrittenAnnotation": "valueFromK8ssandraSpec",
					},
				},
			},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"existingAnnotation":    "existingValue",
				"overwrittenAnnotation": "valueFromConfigMap",
			},
		},
	}
	AddCommonAnnotations(cm, k8c)
	assert.Equal(t, len(cm.Annotations), 3)
	assert.Equal(t, cm.Annotations["newAnnotation"], "newValue")
	assert.Equal(t, cm.Annotations["existingAnnotation"], "existingValue")
	assert.Equal(t, cm.Annotations["overwrittenAnnotation"], "valueFromConfigMap")
}

func TestAddCommonAnnotations(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				Meta: k8ssandrameta.CassandraClusterMeta{
					CommonAnnotations: map[string]string{
						"test": "test",
					},
				},
			},
		},
	}
	cm := &corev1.ConfigMap{}
	AddCommonAnnotations(cm, k8c)
	assert.Equal(t, cm.Annotations["test"], "test")
}

func TestAddCommonAnnotationsNoCommonMeta(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{},
		},
	}
	cm := &corev1.ConfigMap{}
	AddCommonAnnotations(cm, k8c)
	assert.Equal(t, len(cm.Annotations), 0)
}

func TestAddCommonAnnotationsNoCassandra(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{}
	cm := &corev1.ConfigMap{}
	AddCommonAnnotations(cm, k8c)
	assert.Equal(t, len(cm.Annotations), 0)
}
