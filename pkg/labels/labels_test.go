package labels

import (
	"testing"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	k8ssandrameta "github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddCommonLabelsExistingLabels(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				Meta: k8ssandrameta.CassandraClusterMeta{
					CommonLabels: map[string]string{
						"newLabel":         "newValue",
						"overwrittenLabel": "valueFromK8ssandraSpec",
					},
				},
			},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"existingLabel":    "existingValue",
				"overwrittenLabel": "valueFromConfigMap",
			},
		},
	}

	AddCommonLabels(cm, k8c)
	assert.Equal(t, len(cm.Labels), 3)
	assert.Equal(t, cm.Labels["existingLabel"], "existingValue")
	assert.Equal(t, cm.Labels["newLabel"], "newValue")
	assert.Equal(t, cm.Labels["overwrittenLabel"], "valueFromConfigMap")
}

func TestAddCommonLabels(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				Meta: k8ssandrameta.CassandraClusterMeta{
					CommonLabels: map[string]string{"test": "test"},
				},
			},
		},
	}
	cm := &corev1.ConfigMap{}
	AddCommonLabels(cm, k8c)
	assert.Equal(t, len(cm.Labels), 1)
	assert.Equal(t, cm.Labels["test"], "test")
}

func TestAddCommonLabelsNoCommonMeta(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{}
	cm := &corev1.ConfigMap{}
	AddCommonLabels(cm, k8c)
	assert.Equal(t, len(cm.Labels), 0)
}

func TestAddCommonLabelsNoCassandra(t *testing.T) {
	k8c := &k8ssandraapi.K8ssandraCluster{}
	cm := &corev1.ConfigMap{}
	AddCommonLabels(cm, k8c)
	assert.Equal(t, len(cm.Labels), 0)
}
