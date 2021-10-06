package replication

import (
	"context"
	"testing"
	"time"

	coreapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
)

const (
	timeout  = time.Second * 5
	interval = time.Millisecond * 500
)

var (
	targetCopyToCluster = "cluster-1"
	targetNoCopyCluster = "cluster-2"
	testEnv             *testutils.MultiClusterTestEnv
)

func TestSecretController(t *testing.T) {
	ctx := testutils.TestSetup(t)
	// ctx, cancel := context.WithCancel(ctx)
	testEnv = &testutils.MultiClusterTestEnv{}
	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		return (&SecretSyncController{
			ReconcilerConfig: config.InitConfig(),
			ClientCache:      clientCache,
		}).SetupWithManager(mgr, clusters)
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	defer testEnv.Stop(t)
	// defer cancel()

	// Secret controller tests
	t.Run("SingleClusterDoNothingToSecretsTest", testEnv.ControllerTest(ctx, wrongClusterIgnoreCopy))
	t.Run("MultiClusterSyncSecretsTest", testEnv.ControllerTest(ctx, copySecretsFromClusterToCluster))
	t.Run("VerifyFinalizerInMultiCluster", testEnv.ControllerTest(ctx, verifySecretIsDeleted))

	t.Run("ManagedReplicatedSecret", testEnv.ControllerTest(ctx, managedReplicatedSecret))
}

// copySecretsFromClusterToCluster Tests:
// 	* Copy secret to another cluster (not existing one)
// 	* Modify the secret in main cluster - see that it is updated to slave cluster also
// 	* Modify the secret in the slave cluster - see that it is replaced by the main cluster data
//	* Verify the status has been updated
func copySecretsFromClusterToCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	// assert := assert.New(t)
	var empty struct{}

	rsec := generateReplicatedSecret(namespace)
	err := f.Client.Create(ctx, rsec)
	require.NoError(err, "failed to create replicated secret to main cluster")

	generatedSecrets := generateSecrets(namespace)
	for _, s := range generatedSecrets {
		err := f.Client.Create(ctx, s)
		require.NoError(err, "failed to create secret to main cluster")
	}

	startTime := time.Now()

	t.Log("check that the secrets were copied to other cluster(s)")
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
		}, generatedSecrets[0].Namespace)
	}, timeout, interval)

	t.Log("check that secret not match by replicated secret was not copied")
	require.Never(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[1].Name: empty,
		}, generatedSecrets[0].Namespace)
	}, 3, interval)

	t.Log("check that nothing was copied to cluster not match by replicated secret")
	require.Never(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetNoCopyCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
			generatedSecrets[1].Name: empty,
		}, generatedSecrets[0].Namespace)
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
		}, generatedSecrets[0].Namespace)
	}, timeout, interval)

	t.Log("modify the secret in target cluster")
	modifierClient := testEnv.Clients[targetCopyToCluster]
	targetSecrets := &corev1.SecretList{}
	err = modifierClient.List(ctx, targetSecrets, client.InNamespace(generatedSecrets[0].Namespace))
	require.NoError(err)
	for _, targetSecret := range targetSecrets.Items {
		if targetSecret.Name == generatedSecrets[0].Name {
			phantomSecret := targetSecret.DeepCopy()
			phantomSecret.Data["be-gone-key"] = []byte("my-phantom-value")
			targetSecret.GetAnnotations()[coreapi.ResourceHashAnnotation] = "XXXXXX"
			err = modifierClient.Update(ctx, phantomSecret)
			require.NoError(err)
			break
		}
	}

	t.Log("verify it was returned to original form")
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
		}, generatedSecrets[0].Namespace)
	}, timeout, interval)

	t.Log("check status is set to complete")
	// Get updated status
	require.Eventually(func() bool {
		updatedRSec := &api.ReplicatedSecret{}
		err = f.Client.Get(context.TODO(), types.NamespacedName{Name: rsec.Name, Namespace: rsec.Namespace}, updatedRSec)
		if err != nil {
			return false
		}
		// require.NoError(err)

		// We only copy to a single target cluster
		if len(updatedRSec.Status.Conditions) != 1 {
			return false
		}
		for _, cond := range updatedRSec.Status.Conditions {
			if !(cond.LastTransitionTime.After(startTime) &&
				cond.Cluster != "" &&
				cond.Type == api.ReplicationDone &&
				cond.Status == corev1.ConditionTrue) {
				return false
			}
		}
		return true
	}, timeout, interval)

	t.Log("delete the replicated secret")
	err = f.Client.Delete(ctx, rsec)
	require.NoError(err, "failed to delete replicated secret from main cluster")

	t.Log("verify the replicated secrets are gone from the remote cluster")
	remoteClient := testEnv.Clients[targetCopyToCluster]
	require.Eventually(func() bool {
		remoteSecret := &corev1.Secret{}
		err := remoteClient.Get(context.TODO(), types.NamespacedName{Name: generatedSecrets[0].Name, Namespace: rsec.Namespace}, remoteSecret)
		if err != nil {
			return errors.IsNotFound(err)
		}
		return false
	}, timeout, interval)
}

// verifySecretIsDeleted checks that the finalizer functionality works
// 	* Create secret and ReplicatedSecret
//	* Verify it is copied to another cluster
//  * Delete ReplicatedSecret from main cluster
//  * Verify the secrets are deleted from the remote cluster (but not local)
func verifySecretIsDeleted(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	var empty struct{}

	rsec := generateReplicatedSecret(namespace)
	err := f.Client.Create(ctx, rsec)
	require.NoError(err, "failed to create replicated secret to main cluster")

	generatedSecrets := generateSecrets(namespace)
	for _, s := range generatedSecrets {
		err := f.Client.Create(ctx, s)
		require.NoError(err, "failed to create secret to main cluster")
	}

	t.Log("check that the secret was copied to other cluster(s)")
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{targetCopyToCluster}, map[string]struct{}{
			generatedSecrets[0].Name: empty,
		}, generatedSecrets[0].Namespace)
	}, timeout, interval)

	t.Log("delete the replicated secret")
	err = f.Client.Delete(ctx, rsec)
	require.NoError(err, "failed to delete replicated secret from main cluster")

	t.Log("verify the replicated secrets are gone from the remote cluster")
	remoteClient := testEnv.Clients[targetCopyToCluster]
	require.Eventually(func() bool {
		remoteSecret := &corev1.Secret{}
		err := remoteClient.Get(context.TODO(), types.NamespacedName{Name: generatedSecrets[0].Name, Namespace: rsec.Namespace}, remoteSecret)
		if err != nil {
			return errors.IsNotFound(err)
		}
		return false
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
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"secret-controller": "test"},
			},
			ReplicationTargets: []api.ReplicationTarget{
				{
					K8sContextName: "cluster-1",
				},
			},
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
func verifySecretsMatch(t *testing.T, ctx context.Context, localClient client.Client, remoteClusters []string, secrets map[string]struct{}, namespace string) bool {
	secretList := &corev1.SecretList{}
	err := localClient.List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return false
	}

	for _, remoteCluster := range remoteClusters {
		testClient := testEnv.Clients[remoteCluster]

		targetSecretList := &corev1.SecretList{}
		err := testClient.List(ctx, targetSecretList, client.InNamespace(namespace))
		if err != nil {
			return false
		}

		for _, s := range secretList.Items {
			if _, exists := secrets[s.Name]; exists {
				// Find the corresponding item from targetSecretList - or fail if it's not there
				found := false
				for _, ts := range targetSecretList.Items {
					if s.Name == ts.Name {
						found = true
						if s.GetAnnotations()[coreapi.ResourceHashAnnotation] != ts.GetAnnotations()[coreapi.ResourceHashAnnotation] {
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

// TestSyncSecrets verifies that the original properties from source secret are always maintained in the target secret
func TestSyncSecrets(t *testing.T) {
	assert := assert.New(t)

	dest := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
		},
	}

	orig := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
			Labels: map[string]string{
				"label1": "value1",
			},
			Annotations: map[string]string{
				coreapi.ResourceHashAnnotation: "12345678",
			},
		},
		Data: map[string][]byte{
			"first-key": []byte("firstVal"),
		},
	}

	syncSecrets(orig, dest)

	assert.Equal(orig.GetAnnotations(), dest.GetAnnotations())
	assert.Equal(orig.GetLabels(), dest.GetLabels())

	assert.Equal(orig.Data, dest.Data)

	dest.GetLabels()[OrphanResourceAnnotation] = "true"

	dest.GetAnnotations()[coreapi.ResourceHashAnnotation] = "9876555"

	syncSecrets(orig, dest)

	// Verify additional orphan annotation was not removed
	assert.Contains(dest.GetLabels(), OrphanResourceAnnotation)

	// Verify original labels and their values are set
	for k, v := range orig.GetLabels() {
		assert.Equal(v, dest.GetLabels()[k])
	}

	// Verify original annotations and their values are set (modified hash annotation is overwritten)
	for k, v := range orig.GetAnnotations() {
		assert.Equal(v, dest.GetAnnotations()[k])
	}
}

func TestRequiresUpdate(t *testing.T) {
	assert := assert.New(t)

	dest := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
			UID:       "1",
		},
	}

	orig := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a",
			Namespace: "b",
			UID:       "a",
			Annotations: map[string]string{
				coreapi.ResourceHashAnnotation: "",
			},
		},
		Data: map[string][]byte{
			"first-key": []byte("firstVal"),
		},
	}

	orig.GetAnnotations()[coreapi.ResourceHashAnnotation] = utils.DeepHashString(orig.Data)

	// Secrets don't match
	assert.True(requiresUpdate(orig, dest))

	syncSecrets(orig, dest)

	assert.False(requiresUpdate(orig, dest))

	// Modify target data without fixing the hash annotation, this should cause update requirement
	dest.Data["secondKey"] = []byte("thisValWillBeGone")
	assert.True(requiresUpdate(orig, dest))

	syncSecrets(orig, dest)
	assert.False(requiresUpdate(orig, dest))
}

func managedReplicatedSecret(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	targetCtxs := make([]string, 0, len(testEnv.Clients))

	for k, _ := range testEnv.Clients {
		targetCtxs = append(targetCtxs, k)
	}

	err := secret.ReconcileReplicatedSecret(ctx, f.Client, "test", namespace, targetCtxs)
	require.NoError(err)

	replSecret := &api.ReplicatedSecret{}
	replSecretKey := types.NamespacedName{Name: "test", Namespace: namespace}
	err = f.Client.Get(context.Background(), replSecretKey, replSecret)
	require.NoError(err)

	// Verify data in it

	val, exists := replSecret.Labels[coreapi.ManagedByLabel]
	assert.True(exists)
	assert.Equal(coreapi.NameLabelValue, val)
	val, exists = replSecret.Labels[coreapi.K8ssandraClusterLabel]
	assert.True(exists)
	assert.Equal("test", val)

	assert.Equal(len(testEnv.Clients), len(replSecret.Spec.ReplicationTargets))

	// Create superuserSecret and verify it is correctly replicated also
	err = secret.ReconcileSuperuserSecret(ctx, f.Client, "test-superuser", "test", namespace)
	require.NoError(err)

	var empty struct{}
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, targetCtxs, map[string]struct{}{
			"test-superuser": empty,
		}, namespace)
	}, timeout, interval)
}
