package k8ssandra

import (
	"context"
	"github.com/go-logr/logr/testr"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestK8ssandraClusterReconciler_reconcilePerNodeConfiguration(t *testing.T) {
	type args struct {
		kcKey        types.NamespacedName
		dcConfig     *cassandra.DatacenterConfig
		remoteClient client.Client
	}
	tests := []struct {
		name       string
		args       args
		wantConfig *cassandra.DatacenterConfig
		wantResult result.ReconcileResult
	}{
		{
			name: "per-node config not found",
			args: args{
				kcKey: types.NamespacedName{Namespace: "test", Name: "test"},
				dcConfig: &cassandra.DatacenterConfig{
					Meta: api.EmbeddedObjectMeta{
						Name: "dc1",
					},
					PodTemplateSpec: &corev1.PodTemplateSpec{},
				},
				remoteClient: func() client.Client {
					fakeClient, _ := test.NewFakeClient()
					return fakeClient
				}(),
			},
			wantConfig: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name: "dc1",
				},
				PodTemplateSpec: &corev1.PodTemplateSpec{},
			},
			wantResult: result.Continue(),
		},
		{
			name: "per-node config found",
			args: args{
				kcKey: types.NamespacedName{Namespace: "test", Name: "test"},
				dcConfig: &cassandra.DatacenterConfig{
					Meta: api.EmbeddedObjectMeta{
						Name:      "dc1",
						Namespace: "dc1-ns",
					},
					PodTemplateSpec: &corev1.PodTemplateSpec{},
				},
				remoteClient: func() client.Client {
					var perNodeConfig = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-dc1-per-node-config",
							Namespace: "dc1-ns",
						},
						Data: map[string]string{
							"test-dc1-default-sts-0_cassandra.yaml": "irrelevant",
							"test-dc1-default-sts-1_cassandra.yaml": "irrelevant",
							"test-dc1-default-sts-2_cassandra.yaml": "irrelevant",
						},
					}
					fakeClient, _ := test.NewFakeClient(perNodeConfig)
					return fakeClient
				}(),
			},
			wantConfig: &cassandra.DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name:      "dc1",
					Namespace: "dc1-ns",
				},
				PodTemplateSpec: &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: reconciliation.ServerConfigContainerName},
							*perNodeConfigInitContainer,
						},
						Volumes: []corev1.Volume{
							newPerNodeConfigVolume("test-dc1-per-node-config"),
						},
					},
				},
			},
			wantResult: result.Continue(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &K8ssandraClusterReconciler{}
			testLogger := testr.New(t)
			gotResult := r.reconcilePerNodeConfiguration(context.Background(), tt.args.kcKey, tt.args.dcConfig, tt.args.remoteClient, testLogger)
			assert.Equal(t, tt.wantConfig, tt.args.dcConfig)
			assert.Equal(t, tt.wantResult, gotResult)
		})
	}
}
