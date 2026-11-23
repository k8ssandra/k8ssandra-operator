package k8ssandra

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateDatacenterConfigsEncryption(t *testing.T) {
	tests := []struct {
		name          string
		objects       []client.Object
		expectedError string
	}{
		{
			name:    "propagates password init image",
			objects: newControllerEncryptionSecrets(),
		},
		{
			name: "rejects missing secret",
			objects: func() []client.Object {
				secrets := newControllerEncryptionSecrets()
				return secrets[:len(secrets)-1]
			}(),
			expectedError: "server-truststore-secret",
		},
		{
			name: "rejects missing password key",
			objects: func() []client.Object {
				secrets := newControllerEncryptionSecrets()
				secrets[0].(*corev1.Secret).Data = map[string][]byte{"keystore": []byte("store")}
				return secrets
			}(),
			expectedError: `missing required key "keystore-password"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remoteClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(tt.objects...).
				Build()
			cache := clientcache.New(remoteClient, remoteClient, scheme.Scheme)
			reconciler := &K8ssandraClusterReconciler{
				ClientCache:   cache,
				ImageRegistry: getTestImageRegistry(),
			}

			configs, err := reconciler.createDatacenterConfigs(
				context.Background(),
				newControllerEncryptionCluster(),
				testr.New(t),
				nil,
			)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				require.Nil(t, configs)
				return
			}

			require.NoError(t, err)
			require.Len(t, configs, 1)
			config := configs[0]
			initContainerIndex, found := cassandra.FindInitContainer(&config.PodTemplateSpec, "k8ssandra-encryption-passwords")
			require.True(t, found)
			require.Equal(
				t,
				reconciler.ImageRegistry.GetClientImage(),
				config.PodTemplateSpec.Spec.InitContainers[initContainerIndex].Image,
			)
			_, found = cassandra.FindVolume(&config.PodTemplateSpec, "k8ssandra-encryption-passwords")
			require.True(t, found)
			_, found = config.CassandraConfig.CassandraYaml.Get("client_encryption_options/keystore_password")
			require.False(t, found)
			_, found = config.CassandraConfig.CassandraYaml.Get("server_encryption_options/truststore_password")
			require.False(t, found)
		})
	}
}

func newControllerEncryptionCluster() *api.K8ssandraCluster {
	return &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				ServerType: api.ServerDistributionCassandra,
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "4.0.18",
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{{
					Meta: api.EmbeddedObjectMeta{Name: "dc1"},
					Size: 3,
					DatacenterOptions: api.DatacenterOptions{
						CassandraConfig: &api.CassandraConfig{
							CassandraYaml: unstructured.Unstructured{
								"client_encryption_options": map[string]interface{}{
									"enabled":             true,
									"keystore_password":   "plaintext-password",
									"truststore_password": "plaintext-password",
								},
								"server_encryption_options": map[string]interface{}{
									"internode_encryption": "all",
									"keystore_password":    "plaintext-password",
									"truststore_password":  "plaintext-password",
								},
							},
						},
					},
				}},
				ClientEncryptionStores: &encryption.Stores{
					KeystoreSecretRef:   controllerEncryptionSelector("client-keystore-secret"),
					TruststoreSecretRef: controllerEncryptionSelector("client-truststore-secret"),
				},
				ServerEncryptionStores: &encryption.Stores{
					KeystoreSecretRef:   controllerEncryptionSelector("server-keystore-secret"),
					TruststoreSecretRef: controllerEncryptionSelector("server-truststore-secret"),
				},
			},
		},
	}
}

func newControllerEncryptionSecrets() []client.Object {
	return []client.Object{
		controllerEncryptionSecret("client-keystore-secret", "keystore", "keystore-password"),
		controllerEncryptionSecret("client-truststore-secret", "truststore", "truststore-password"),
		controllerEncryptionSecret("server-keystore-secret", "keystore", "keystore-password"),
		controllerEncryptionSecret("server-truststore-secret", "truststore", "truststore-password"),
	}
}

func controllerEncryptionSecret(name, storeKey, passwordKey string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test"},
		Data: map[string][]byte{
			storeKey:    []byte("store"),
			passwordKey: []byte("password"),
		},
	}
}

func controllerEncryptionSelector(name string) *encryption.SecretKeySelector {
	return &encryption.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
	}
}
