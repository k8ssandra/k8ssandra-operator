package controllers

import (
	"context"
	"reflect"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	targetCopyToCluster = "cluster-1"
	targetNoCopyCluster = "cluster-2"
	testEnv             *MultiClusterTestEnv
)

func testSecretController(ctx context.Context, t *testing.T) {
	ctx, cancel := context.WithCancel(ctx)
	testEnv = &MultiClusterTestEnv{}
	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		return (&SecretSyncController{
			ClientCache: clientCache,
		}).SetupWithManager(mgr, clusters)
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	defer testEnv.Stop(t)
	defer cancel()

	// Secret controller tests
	t.Run("SingleClusterDoNothingToSecretsTest", testEnv.ControllerTest(ctx, wrongClusterIgnoreCopy))
	t.Run("MultiClusterSyncSecretsTest", testEnv.ControllerTest(ctx, copySecretsFromClusterToCluster))
}

// copySecretsFromClusterToCluster Tests:
// 	* Copy secret to another cluster (not existing one)
// 	* Modify the secret in main cluster - see that it is updated to slave cluster also
// 	* Modify the secret in the slave cluster - see that it is replaced by the main cluster data
func copySecretsFromClusterToCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	var empty struct{}

	err := f.Client.Create(ctx, generateReplicatedSecret(namespace))
	require.NoError(err, "failed to create replicated secret to main cluster")

	generatedSecrets := generateSecrets(namespace)
	for _, s := range generatedSecrets {
		err := f.Client.Create(ctx, s)
		require.NoError(err, "failed to create secret to main cluster")
	}

	t.Log("check that the secrets were copied to other cluster(s)")
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
		})
	}, timeout, interval)

	t.Log("check that secret not match by replicated secret was not copied")
	require.Never(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[1].Name: empty,
		})
	}, 3, interval)

	t.Log("check that nothing was copied to cluster not match by replicated secret")
	require.Never(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetNoCopyCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
			generatedSecrets[1].Name: empty,
		})
	}, 3, interval)

	t.Log("modify the secret in the main cluster")
	toModifySecret := &corev1.Secret{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: generatedSecrets[0].Name, Namespace: namespace}, toModifySecret)
	require.NoError(err, "failed to fetch modified secret from the main cluster")
	toModifySecret.Data["newKey"] = []byte("my-new-value")
	err = f.Client.Update(ctx, toModifySecret)
	require.NoError(err, "failed to modify secret in the main cluster")

	t.Log("verify it was modified in the target cluster also")
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
		})
	}, timeout, interval)

	t.Log("modify the secret in target cluster")
	modifierClient := testEnv.Clients[targetCopyToCluster]
	targetSecrets := &corev1.SecretList{}
	err = modifierClient.List(ctx, targetSecrets)
	require.NoError(err)
	for _, targetSecret := range targetSecrets.Items {
		if targetSecret.Name == generatedSecrets[0].Name {
			phantomSecret := targetSecret.DeepCopy()
			phantomSecret.Data["be-gone-key"] = []byte("my-phantom-value")
			err = modifierClient.Update(ctx, phantomSecret)
			require.NoError(err)
			break
		}
	}

	t.Log("verify it was returned to original form")
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
		})
	}, timeout, interval)
}

// wrongClusterIgnoreCopy tests that the secrets are not copied if written to the target cluster instead of local cluster
func wrongClusterIgnoreCopy(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	err := f.Client.Create(ctx, generateReplicatedSecret(namespace))
	require.NoError(err, "failed to create replicated secret to main cluster")

	generatedSecrets := generateSecrets(namespace)

	for _, s := range generatedSecrets {
		err := testEnv.Clients[targetCopyToCluster].Create(ctx, s)
		require.NoError(err, "failed to create secret to main cluster")
	}

	t.Log("check that the secrets were not copied to other cluster(s)")
	for _, s := range generatedSecrets {
		require.Never(func() bool {
			return verifySecretCopied(t, ctx, targetCopyToCluster, s, nil)
		}, timeout, interval)
	}
}

func generateSecrets(namespace string) []*corev1.Secret {
	return []*corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "test-secret-first",
				Labels: map[string]string{
					"secret-controller": "test",
				},
			},
			Type: "Opaque",
			StringData: map[string]string{
				"key": "value",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "test-secret-second",
				Labels: map[string]string{
					"secret-controller": "nomatch",
				},
			},
			Type: "Opaque",
			StringData: map[string]string{
				"key": "value",
			},
		},
	}
}

func generateReplicatedSecret(namespace string) *api.ReplicatedSecret {
	return &api.ReplicatedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "fetch-secrets",
		},
		Spec: api.ReplicatedSecretSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"secret-controller": "test"},
			},
			TargetContexts: []string{"cluster-1"},
		},
	}
}

// verifySecretCopied checks that the same key is copied to other clusters
func verifySecretCopied(t *testing.T, ctx context.Context, origCluster string, originalSecret *corev1.Secret, verify func(*testing.T, *corev1.Secret, *corev1.Secret)) bool {

	secretKey := types.NamespacedName{Namespace: originalSecret.Namespace, Name: originalSecret.Name}
	for clusterKey, testClient := range testEnv.Clients {
		if clusterKey == origCluster {
			continue
		}
		secretCopy := &corev1.Secret{}
		err := testClient.Get(ctx, secretKey, secretCopy)
		if err != nil {
			// All errors are false
			return false
		}

		if verify != nil {
			verify(t, originalSecret, secretCopy)
		}
	}
	// In a single cluster setup, no copies is fine
	return true
}

// verifySecretsMatch checks that the same secret is copied to other clusters
func verifySecretsMatch(t *testing.T, ctx context.Context, localClient client.Client, remoteClusters []string, secrets map[string]struct{}) bool {
	secretList := &corev1.SecretList{}
	err := localClient.List(ctx, secretList)
	if err != nil {
		return false
	}

	for _, remoteCluster := range remoteClusters {
		testClient := testEnv.Clients[remoteCluster]

		targetSecretList := &corev1.SecretList{}
		err := testClient.List(ctx, targetSecretList)
		if err != nil {
			return false
		}

		for _, s := range secretList.Items {
			if _, toCheck := secrets[s.Name]; toCheck {
				// Find the corresponding item from targetSecretList - or fail if it's not there
				found := false
				for _, ts := range targetSecretList.Items {
					if s.Name == ts.Name {
						found = true
						if !reflect.DeepEqual(s.Data, ts.Data) {
							t.Logf("Differ: %v vs %v", s.Data, ts.Data)
							return false
						}
						break
					}
				}
				if !found {
					return false
				}
			}
		}
	}

	return true
}
