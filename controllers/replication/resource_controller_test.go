package replication

import (
	"context"
	"fmt"
	"testing"
	"time"

	coreapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestConfigMapController(t *testing.T) {
	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)
	testEnv = &testutils.MultiClusterTestEnv{}
	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		scheme = mgr.GetScheme()
		logger = mgr.GetLogger()
		return (&ConfigMapSyncController{
			ReconcilerConfig: config.InitConfig(),
			ClientCache:      clientCache,
		}).SetupWithManager(mgr, clusters)
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	defer testEnv.Stop(t)
	defer cancel()

	// ConfigMap controller tests
	t.Run("MultiClusterSyncConfigMapTest", testEnv.ControllerTest(ctx, copyConfigMapFromClusterToCluster))
}

// copyConfigMapsFromClusterToCluster Tests:
// 	* Copy configMap to another cluster (not existing one)
// 	* Modify the configMap in main cluster - see that it is updated to slave cluster also
// 	* Modify the configMap in the slave cluster - see that it is replaced by the main cluster data
//	* Verify the status has been updated
func copyConfigMapFromClusterToCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	// assert := assert.New(t)
	var empty struct{}

	rsec := generateReplicatedConfigMap(f.K8sContext(1), namespace)
	rsec.Name = "broke"
	err := f.Client.Create(ctx, rsec)
	require.NoError(err, "failed to create replicated configMap to main cluster")

	generatedConfigMaps := generateConfigMaps(namespace)
	for i, s := range generatedConfigMaps {
		s.Name = fmt.Sprintf("broken-configMap-%d", i)
		err := f.Client.Create(ctx, s)
		require.NoError(err, "failed to create configMap to main cluster")
	}

	startTime := time.Now()

	t.Log("check that the configMaps were copied to other cluster(s)")
	require.Eventually(func() bool {
		return verifyConfigMapsMatch(t, ctx, f.Client, []string{f.K8sContext(targetCopyToCluster)}, map[string]struct{}{
			generatedConfigMaps[0].Name: empty,
		}, generatedConfigMaps[0].Namespace)
	}, timeout, interval)

	t.Log("check that configMap not match by replicated configMap was not copied")
	require.Never(func() bool {
		return verifyConfigMapsMatch(t, ctx, f.Client, []string{f.K8sContext(targetCopyToCluster)}, map[string]struct{}{
			generatedConfigMaps[1].Name: empty,
		}, generatedConfigMaps[0].Namespace)
	}, 3, interval)

	t.Log("check that nothing was copied to cluster not match by replicated configMap")
	require.Never(func() bool {
		return verifyConfigMapsMatch(t, ctx, f.Client, []string{f.K8sContext(targetNoCopyCluster)}, map[string]struct{}{
			generatedConfigMaps[0].Name: empty,
			generatedConfigMaps[1].Name: empty,
		}, generatedConfigMaps[0].Namespace)
	}, 3, interval)

	t.Log("modify the configMap in the main cluster")
	toModifyConfigMap := &corev1.ConfigMap{}
	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: generatedConfigMaps[0].Name, Namespace: namespace}, toModifyConfigMap)
	require.NoError(err, "failed to fetch modified configMap from the main cluster")
	toModifyConfigMap.Data["newKey"] = "my-new-value"
	err = f.Client.Update(ctx, toModifyConfigMap)
	require.NoError(err, "failed to modify configMap in the main cluster")

	t.Log("verify it was modified in the target cluster also")
	require.Eventually(func() bool {
		return verifyConfigMapsMatch(t, ctx, f.Client, []string{f.K8sContext(targetCopyToCluster)}, map[string]struct{}{
			generatedConfigMaps[0].Name: empty,
		}, generatedConfigMaps[0].Namespace)
	}, timeout, interval)

	t.Log("modify the configMap in target cluster")
	modifierClient := testEnv.Clients[f.K8sContext(targetCopyToCluster)]
	targetConfigMaps := &corev1.ConfigMapList{}
	err = modifierClient.List(ctx, targetConfigMaps, client.InNamespace(generatedConfigMaps[0].Namespace))
	require.NoError(err)
	for _, targetConfigMap := range targetConfigMaps.Items {
		if targetConfigMap.Name == generatedConfigMaps[0].Name {
			phantomConfigMap := targetConfigMap.DeepCopy()
			phantomConfigMap.Data["be-gone-key"] = "my-phantom-value"
			targetConfigMap.GetAnnotations()[coreapi.ResourceHashAnnotation] = "XXXXXX"
			err = modifierClient.Update(ctx, phantomConfigMap)
			require.NoError(err)
			break
		}
	}

	t.Log("verify it was returned to original form")
	require.Eventually(func() bool {
		return verifyConfigMapsMatch(t, ctx, f.Client, []string{f.K8sContext(targetCopyToCluster)}, map[string]struct{}{
			generatedConfigMaps[0].Name: empty,
		}, generatedConfigMaps[0].Namespace)
	}, timeout, interval)

	t.Log("check status is set to complete")
	// Get updated status
	require.Eventually(func() bool {
		updatedRSec := &api.ReplicatedConfigMap{}
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

	t.Log("delete the replicated configMap")
	err = f.Client.Delete(ctx, rsec)
	require.NoError(err, "failed to delete replicated configMap from main cluster")

	t.Log("verify the replicated configMaps are gone from the remote cluster")
	remoteClient := testEnv.Clients[f.K8sContext(targetCopyToCluster)]
	require.Eventually(func() bool {
		t.Logf("checking for configMap deletion: %v", types.NamespacedName{Name: generatedConfigMaps[0].Name, Namespace: rsec.Namespace})
		remoteConfigMap := &corev1.ConfigMap{}
		err := remoteClient.Get(context.TODO(), types.NamespacedName{Name: generatedConfigMaps[0].Name, Namespace: rsec.Namespace}, remoteConfigMap)
		if err != nil {
			if !errors.IsNotFound(err) {
				t.Logf("Failed to get configMap: %v", err)
			}
			return errors.IsNotFound(err)
		}
		return false
	}, timeout, interval)
}

// verifySecretsMatch checks that the same secret is copied to other clusters
func verifyConfigMapsMatch(t *testing.T, ctx context.Context, localClient client.Client, remoteClusters []string, secrets map[string]struct{}, namespace string) bool {
	configMapList := &corev1.ConfigMapList{}
	err := localClient.List(ctx, configMapList, client.InNamespace(namespace))
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

		for _, s := range configMapList.Items {
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

func generateConfigMaps(namespace string) []*corev1.ConfigMap {
	return []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "test-configMap-first",
				Labels: map[string]string{
					"configMap-controller":                 "test",
					coreapi.K8ssandraClusterNamespaceLabel: namespace,
					coreapi.K8ssandraClusterNameLabel:      "k8sssandra",
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "test-configMap-second",
				Labels: map[string]string{
					"configMap-controller": "nomatch",
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		},
	}
}

func generateReplicatedConfigMap(k8sContext, namespace string) *api.ReplicatedConfigMap {
	return &api.ReplicatedConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "fetch-configMaps",
		},
		Spec: api.ReplicatedConfigMapSpec{
			ReplicatedResourceSpec: &api.ReplicatedResourceSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"configMap-controller":                 "test",
						coreapi.K8ssandraClusterNamespaceLabel: namespace,
						coreapi.K8ssandraClusterNameLabel:      "k8sssandra",
					},
				},
				ReplicationTargets: []api.ReplicationTarget{
					{
						K8sContextName: k8sContext,
					},
				},
			},
		},
	}
}
