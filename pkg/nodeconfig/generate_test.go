package nodeconfig

import (
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func TestNewDefaultPerNodeConfigMap(t *testing.T) {
	tests := []struct {
		name  string
		kcKey types.NamespacedName
		dc    *cassandra.DatacenterConfig
		want  *corev1.ConfigMap
	}{
		{
			name: "no per-node config",
			kcKey: types.NamespacedName{
				Namespace: "ns1",
				Name:      "kc1",
			},
			dc: &cassandra.DatacenterConfig{
				Cluster: "cluster1",
				Meta: api.EmbeddedObjectMeta{
					Name: "dc1",
				},
			},
			want: nil,
		},
		{
			name: "with per-node config",
			kcKey: types.NamespacedName{
				Namespace: "ns1",
				Name:      "kc1",
			},
			dc: &cassandra.DatacenterConfig{
				Cluster: "cluster1",
				Meta: api.EmbeddedObjectMeta{
					Name: "dc1",
				},
				InitialTokensByPodName: map[string][]string{
					"pod1": {"token1", "token2"},
					"pod2": {"token3", "token4"},
				},
			},
			want: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns1",
					Name:      "kc1-dc1-per-node-config",
					Labels: map[string]string{
						"app.kubernetes.io/created-by":   "k8ssandracluster-controller",
						"app.kubernetes.io/name":         "k8ssandra-operator",
						"app.kubernetes.io/component":    "cassandra",
						"app.kubernetes.io/part-of":      "k8ssandra",
						"k8ssandra.io/cluster-name":      "kc1",
						"k8ssandra.io/cluster-namespace": "ns1",
					},
				},
				Data: map[string]string{
					"pod1_cassandra.yaml": "\ninitial_token: token1,token2",
					"pod2_cassandra.yaml": "\ninitial_token: token3,token4",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDefaultPerNodeConfigMap(tt.kcKey, tt.dc)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_addInitialTokens(t *testing.T) {
	tests := []struct {
		name string
		dc   *cassandra.DatacenterConfig
		want *corev1.ConfigMap
	}{
		{
			name: "tokens present",
			dc: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name: "dc1",
				},
				InitialTokensByPodName: map[string][]string{
					"pod1": {"token1", "token2"},
					"pod2": {"token3", "token4"},
				},
			},
			want: &corev1.ConfigMap{
				Data: map[string]string{
					"pod1_cassandra.yaml": "\ninitial_token: token1,token2",
					"pod2_cassandra.yaml": "\ninitial_token: token3,token4",
				},
			},
		},
		{
			name: "tokens absent",
			dc:   &cassandra.DatacenterConfig{},
			want: &corev1.ConfigMap{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &corev1.ConfigMap{}
			addInitialTokens(tt.dc, input)
			assert.Equal(t, tt.want, input)
		})
	}
}
