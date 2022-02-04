package e2e

import (
	"context"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

// stopAndRestartDc begins with a multi-cluster setup with 2 dcs. Then it stops dc1 and verifies that dc2 is still
// accessible. Then it stops dc2 as well and verifies that the entire cluster is down. Then it starts dc1 and verifies
// that it becomes accessible again. Then it starts dc2 and verifies that the whole cluster is back to normal.
func stopAndRestartDc(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	t.Log("check that the K8ssandraCluster was created")
	kcKey := client.ObjectKey{Namespace: namespace, Name: "cluster1"}
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)

	k8sCtx0 := "kind-k8ssandra-0"
	k8sCtx1 := "kind-k8ssandra-1"

	dc1Key := framework.NewClusterKey(k8sCtx0, namespace, "dc1")
	sg1Key := framework.NewClusterKey(k8sCtx0, namespace, "cluster1-dc1-stargate")
	reaper1Key := framework.NewClusterKey(k8sCtx0, namespace, "cluster1-dc1-reaper")

	dc2Key := framework.NewClusterKey(k8sCtx1, namespace, "dc2")
	sg2Key := framework.NewClusterKey(k8sCtx1, namespace, "cluster1-dc2-stargate")
	reaper2Key := framework.NewClusterKey(k8sCtx1, namespace, "cluster1-dc2-reaper")

	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)
	checkStargateReady(t, f, ctx, sg1Key)
	checkReaperReady(t, f, ctx, reaper1Key)
	checkStargateReady(t, f, ctx, sg2Key)

	toggleDcStopped(t, f, ctx, kcKey, dc1Key, true)

	t.Logf("Check stargate1 stopped and reaper moved to dc2")
	checkStargateNotFound(t, f, ctx, sg1Key)
	checkReaperNotFound(t, f, ctx, reaper1Key)
	checkReaperReady(t, f, ctx, reaper2Key)

	username, password, err := f.RetrieveDatabaseCredentials(ctx, kcKey.Namespace, "cluster1")

	t.Log("deploying Stargate and Reaper ingress routes in " + k8sCtx1)
	f.DeployStargateIngresses(t, k8sCtx1, 1, namespace, "cluster1-dc2-stargate-service", username, password)
	f.DeployReaperIngresses(t, ctx, k8sCtx1, 1, namespace, "cluster1-dc2-reaper-service")

	defer f.UndeployAllIngresses(t, k8sCtx0, namespace)
	defer f.UndeployAllIngresses(t, k8sCtx1, namespace)

	pod1Name := "cluster1-dc1-default-sts-0"
	pod2Name := "cluster1-dc2-default-sts-0"

	t.Run("TestApisDc1Stopped", func(t *testing.T) {
		testStargateApis(t, ctx, k8sCtx0, 0, username, password, map[string]int{"dc2": 1})
		uiKey := framework.NewClusterKey(k8sCtx1, namespace, reaper.DefaultUiSecretName("cluster1"))
		uiUsername, uiPassword := retrieveCredentials(t, f, ctx, uiKey)
		testReaperApi(t, ctx, 1, "cluster1", reaperapi.DefaultKeyspace, uiUsername, uiPassword)
		checkNodeToolStatus(t, f, k8sCtx1, namespace, pod2Name, 1, 1, "-u", username, "-pw", password)
	})

	toggleDcStopped(t, f, ctx, kcKey, dc2Key, true)

	t.Logf("Check stargate2 stopped and reaper2 stopped")
	checkStargateNotFound(t, f, ctx, sg2Key)
	checkReaperNotFound(t, f, ctx, reaper2Key)

	toggleDcStopped(t, f, ctx, kcKey, dc1Key, false)

	t.Logf("Check stargate1 started and reaper moved to dc1")
	checkStargateReady(t, f, ctx, sg1Key)
	checkReaperReady(t, f, ctx, reaper1Key)

	t.Log("deploying Stargate and Reaper ingress routes in " + k8sCtx0)
	f.DeployStargateIngresses(t, k8sCtx0, 0, namespace, "cluster1-dc1-stargate-service", username, password)
	f.DeployReaperIngresses(t, ctx, k8sCtx0, 0, namespace, "cluster1-dc1-reaper-service")

	t.Run("TestApisDc2Stopped", func(t *testing.T) {
		testStargateApis(t, ctx, k8sCtx0, 0, username, password, map[string]int{"dc1": 1})
		uiKey := framework.NewClusterKey(k8sCtx0, namespace, reaper.DefaultUiSecretName("cluster1"))
		uiUsername, uiPassword := retrieveCredentials(t, f, ctx, uiKey)
		testReaperApi(t, ctx, 0, "cluster1", reaperapi.DefaultKeyspace, uiUsername, uiPassword)
		checkNodeToolStatus(t, f, k8sCtx0, namespace, pod1Name, 1, 1, "-u", username, "-pw", password)
	})

	toggleDcStopped(t, f, ctx, kcKey, dc2Key, false)

	t.Logf("Check stargate2 started and reaper remains on dc1")
	checkStargateReady(t, f, ctx, sg2Key)
	checkReaperNotFound(t, f, ctx, reaper2Key)

	t.Run("TestApisDcsRestarted", func(t *testing.T) {
		testStargateApis(t, ctx, k8sCtx0, 0, username, password, map[string]int{"dc1": 1, "dc2": 1})
		testStargateApis(t, ctx, k8sCtx1, 1, username, password, map[string]int{"dc1": 1, "dc2": 1})
		uiKey := framework.NewClusterKey(k8sCtx0, namespace, reaper.DefaultUiSecretName("cluster1"))
		uiUsername, uiPassword := retrieveCredentials(t, f, ctx, uiKey)
		testReaperApi(t, ctx, 0, "cluster1", reaperapi.DefaultKeyspace, uiUsername, uiPassword)
		checkNodeToolStatus(t, f, k8sCtx0, namespace, pod1Name, 2, 0, "-u", username, "-pw", password)
		checkNodeToolStatus(t, f, k8sCtx1, namespace, pod2Name, 2, 0, "-u", username, "-pw", password)
	})
}

func checkStargateNotFound(t *testing.T, f *framework.E2eFramework, ctx context.Context, sgKey framework.ClusterKey) {
	require.Eventually(t, func() bool {
		sg := &stargateapi.Stargate{}
		return errors.IsNotFound(f.Get(ctx, sgKey, sg))
	}, polling.stargateReady.timeout, polling.stargateReady.interval)
}

func checkReaperNotFound(t *testing.T, f *framework.E2eFramework, ctx context.Context, reaperKey framework.ClusterKey) {
	require.Eventually(t, func() bool {
		r := &reaperapi.Reaper{}
		return errors.IsNotFound(f.Get(ctx, reaperKey, r))
	}, polling.reaperReady.timeout, polling.reaperReady.interval)
}

func toggleDcStopped(t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey client.ObjectKey,
	dcKey framework.ClusterKey,
	stopped bool) {
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(t, err, "failed to get kc")
	dcIndex := -1
	for i, dc := range kc.Spec.Cassandra.Datacenters {
		if dc.Meta.Name == dcKey.Name {
			dcIndex = i
		}
	}
	if dcIndex == -1 {
		require.Fail(t, "no DC with key: %s", dcKey)
	}
	t.Logf("Setting %s stopped flag to %v", dcKey.Name, stopped)
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Cassandra.Datacenters[dcIndex].Stopped = stopped
	err = f.Client.Patch(ctx, kc, patch)
	require.NoError(t, err, "failed to patch kc")
	if stopped {
		checkDatacenterStopped(t, ctx, dcKey, f)
	} else {
		checkDatacenterReady(t, ctx, dcKey, f)
	}
}
