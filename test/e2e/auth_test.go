package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func multiDcAuthOnOff(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	kcKey := types.NamespacedName{Namespace: namespace, Name: "cluster1"}

	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	reaper1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc1-reaper"}}
	reaper2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-real-dc2-reaper"}}

	reaperUiSecretKey := types.NamespacedName{Namespace: namespace, Name: reaper.DefaultUiSecretName("cluster1")}

	// cluster has auth turned off initially
	waitForAllComponentsReady(t, f, ctx, kcKey, dc1Key, dc2Key, reaper1Key, reaper2Key)

	t.Log("deploying Reaper ingress routes in both clusters")
	reaperRestHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].ReaperRest

	f.DeployReaperIngresses(t, f.DataPlaneContexts[0], namespace, fmt.Sprintf("%s-service", reaper1Key.Name), reaperRestHostAndPort)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	reaperRestHostAndPort = ingressConfigs[f.DataPlaneContexts[1]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[1], namespace, fmt.Sprintf("%s-service", reaper2Key.Name), reaperRestHostAndPort)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[1], namespace)

	pod1Name := fmt.Sprintf("%s-default-sts-0", DcPrefix(t, f, dc1Key))
	pod2Name := fmt.Sprintf("%s-default-sts-0", DcPrefix(t, f, dc2Key))
	replication := map[string]int{DcName(t, f, dc1Key): 1, DcName(t, f, dc2Key): 1}

	testAuthenticationDisabled(t, f, ctx, namespace, replication, pod1Name, pod2Name)

	// turn auth on
	toggleAuthentication(t, f, ctx, kcKey, true)
	waitForAllComponentsReady(t, f, ctx, kcKey, dc1Key, dc2Key, reaper1Key, reaper2Key)
	testAuthenticationEnabled(t, f, ctx, namespace, kcKey, reaperUiSecretKey, replication, pod1Name, pod2Name, DcPrefix(t, f, dc1Key), DcPrefixOverride(t, f, dc2Key))
}

func waitForAllComponentsReady(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dc1Key, dc2Key framework.ClusterKey,
	reaper1Key, reaper2Key framework.ClusterKey,
) {
	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)
	checkReaperReady(t, f, ctx, reaper1Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc1Key)
	checkReaperReady(t, f, ctx, reaper2Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc2Key)
	// we need to wait until the deployments are fully rolled out before continuing, to avoid hitting an old
	// pod that has authentication enabled while we just turned it off.
	options1 := kubectl.Options{Namespace: kcKey.Namespace, Context: f.DataPlaneContexts[0]}
	options2 := kubectl.Options{Namespace: kcKey.Namespace, Context: f.DataPlaneContexts[1]}

	assert.NoError(t, kubectl.RolloutStatus(ctx, options1, "deployment", reaper1Key.Name))
	assert.NoError(t, kubectl.RolloutStatus(ctx, options2, "deployment", reaper2Key.Name))
}

func toggleAuthentication(t *testing.T, f *framework.E2eFramework, ctx context.Context, kcKey types.NamespacedName, on bool) {
	t.Logf("toggle authentication %v", on)
	var kc v1alpha1.K8ssandraCluster
	err := f.Client.Get(ctx, kcKey, &kc)
	require.NoError(t, err, "failed to get K8ssandraCluster %v", kcKey)
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Auth = ptr.To(on)
	err = f.Client.Patch(ctx, &kc, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster %v", kcKey)
}

func testAuthenticationDisabled(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	namespace string,
	replication map[string]int,
	pod1Name, pod2Name string,
) {
	t.Run("TestJmxAccessAuthDisabled", func(t *testing.T) {
		t.Run("Local", func(t *testing.T) {
			t.Log("check that nodes in different dcs can see each other (auth disabled, local JMX)")
			checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, pod1Name, 2, 0)
			checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod2Name, 2, 0)
			t.Log("check that we can communicate through CQL without auth")
			_, err := f.ExecuteCqlNoAuth(f.DataPlaneContexts[0], namespace, pod1Name,
				"SELECT * FROM system.local")
			require.NoError(t, err, "expected CQL query without auth to succeed")
			_, err = f.ExecuteCqlNoAuth(f.DataPlaneContexts[1], namespace, pod2Name,
				"SELECT * FROM system.local")
			require.NoError(t, err, "expected CQL query without auth to succeed")
		})
		// t.Run("Remote", func(t *testing.T) {
		// 	t.Log("check that nodes in different dcs can see each other (auth disabled, remote JMX)")
		// 	pod1IP, pod2IP := getPodIPs(t, f, namespace, pod1Name, pod2Name)
		// 	checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, pod1Name, 2, 0, "-h", pod2IP)
		// 	checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod2Name, 2, 0, "-h", pod1IP)
		// })
	})
	t.Run("TestApisAuthDisabled", func(t *testing.T) {
		t.Run("Reaper", func(t *testing.T) {
			testReaperApi(t, ctx, f.DataPlaneContexts[0], "cluster1", reaperapi.DefaultKeyspace, "", "")
			testReaperApi(t, ctx, f.DataPlaneContexts[1], "cluster1", reaperapi.DefaultKeyspace, "", "")
		})
	})
}

func testAuthenticationEnabled(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	namespace string,
	kcKey, reaperUiSecretKey types.NamespacedName,
	replication map[string]int,
	pod1Name, pod2Name string,
	dc1Prefix, dc2Prefix string,
) {
	t.Log("retrieve superuser credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], kcKey.Namespace, "cluster1")
	require.NoError(t, err, "failed to retrieve superuser credentials")
	t.Run("TestJmxAccessAuthEnabled", func(t *testing.T) {
		t.Run("Local", func(t *testing.T) {
			t.Log("check that nodes in different dcs can see each other (auth enabled, local JMX)")
			checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, pod1Name, 2, 0, "-u", username, "-pw", password)
			checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod2Name, 2, 0, "-u", username, "-pw", password)
			t.Log("check that we cannot communicate through CQL without auth")
			_, err := f.ExecuteCqlNoAuth(f.DataPlaneContexts[0], namespace, pod1Name,
				"SELECT * FROM system.local")
			require.Error(t, err, "expected CQL query without auth to fail")
			_, err = f.ExecuteCqlNoAuth(f.DataPlaneContexts[1], namespace, pod2Name,
				"SELECT * FROM system.local")
			require.Error(t, err, "expected CQL query without auth to fail")
			t.Log("check that we can communicate through CQL with auth")
			_, err = f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, kcKey.Name, pod1Name,
				"SELECT * FROM system.local")
			require.NoError(t, err, "expected CQL query with auth to succeed")
			_, err = f.ExecuteCql(ctx, f.DataPlaneContexts[1], namespace, kcKey.Name, pod2Name,
				"SELECT * FROM system.local")
			require.NoError(t, err, "expected CQL query with auth to succeed")
			// checkLocalJmxFailsWithNoCredentials(t, f, f.DataPlaneContexts[0], namespace, pod1Name)
			// checkLocalJmxFailsWithNoCredentials(t, f, f.DataPlaneContexts[1], namespace, pod2Name)
			// checkLocalJmxFailsWithWrongCredentials(t, f, f.DataPlaneContexts[0], namespace, pod1Name)
			// checkLocalJmxFailsWithWrongCredentials(t, f, f.DataPlaneContexts[1], namespace, pod2Name)
		})
		// t.Run("Remote", func(t *testing.T) {
		// 	t.Log("check that nodes in different dcs can see each other (auth enabled, remote JMX)")
		// 	pod1IP, pod2IP := getPodIPs(t, f, namespace, pod1Name, pod2Name)
		// 	checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, pod1Name, 2, 0, "-h", pod2IP, "-u", username, "-pw", password)
		// 	checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod2Name, 2, 0, "-h", pod1IP, "-u", username, "-pw", password)
		// 	checkRemoteJmxFailsWithNoCredentials(t, f, f.DataPlaneContexts[0], namespace, pod1Name, pod2IP)
		// 	checkRemoteJmxFailsWithNoCredentials(t, f, f.DataPlaneContexts[1], namespace, pod2Name, pod1IP)
		// 	checkRemoteJmxFailsWithWrongCredentials(t, f, f.DataPlaneContexts[0], namespace, pod1Name, pod2IP)
		// 	checkRemoteJmxFailsWithWrongCredentials(t, f, f.DataPlaneContexts[1], namespace, pod2Name, pod1IP)
		// })
	})
	t.Run("TestApisAuthEnabled", func(t *testing.T) {
		t.Run("Reaper", func(t *testing.T) {
			username, password := retrieveCredentials(t, f, ctx, framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: reaperUiSecretKey})
			testReaperApi(t, ctx, f.DataPlaneContexts[0], "cluster1", reaperapi.DefaultKeyspace, username, password)
			testReaperApi(t, ctx, f.DataPlaneContexts[1], "cluster1", reaperapi.DefaultKeyspace, username, password)
		})
	})
}

/*
func checkLocalJmxFailsWithNoCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod string) {
	_, _, err := f.GetNodeToolStatus(k8sContext, namespace, pod)
	if assert.Error(t, err, "expected unauthenticated local JMX connection on pod %v to fail", pod) {
		assert.Contains(t, err.Error(), "Required key 'username' is missing")
	}
}

func checkLocalJmxFailsWithWrongCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod string) {
	_, _, err := f.GetNodeToolStatus(k8sContext, namespace, pod, "-u", "nonexistent", "-pw", "irrelevant")
	if assert.Error(t, err, "expected local JMX connection with wrong credentials on pod %v to fail", pod) {
		assert.Contains(t, err.Error(), "Provided username nonexistent and/or password are incorrect")
	}
}

func checkRemoteJmxFailsWithNoCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod, host string) {
	_, _, err := f.GetNodeToolStatus(k8sContext, namespace, pod, "-h", host)
	if assert.Error(t, err, "expected unauthenticated remote JMX connection from pod %v to host %v to fail", pod, host) {
		assert.Contains(t, err.Error(), "Required key 'username' is missing")
	}
}

func checkRemoteJmxFailsWithWrongCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod, host string) {
	_, _, err := f.GetNodeToolStatus(k8sContext, namespace, pod, "-u", "nonexistent", "-pw", "irrelevant", "-h", host)
	if assert.Error(t, err, "expected remote JMX connection with wrong credentials from pod %v to host %v to fail", pod) {
		assert.Contains(t, err.Error(), "Provided username nonexistent and/or password are incorrect")
	}
}

func getPodIPs(t *testing.T, f *framework.E2eFramework, namespace, pod1Name, pod2Name string) (string, string) {
	pod1IP, err := f.GetPodIP(f.DataPlaneContexts[0], namespace, pod1Name)
	require.NoError(t, err, "failed to get pod %s IP in context %s", pod1Name, f.DataPlaneContexts[0])
	pod2IP, err := f.GetPodIP(f.DataPlaneContexts[1], namespace, pod2Name)
	require.NoError(t, err, "failed to get pod %s IP in context %s", pod2Name, f.DataPlaneContexts[1])
	return pod1IP, pod2IP
}
*/
