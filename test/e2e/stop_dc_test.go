package e2e

import (
	"context"
	"fmt"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
	dc2Key := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, "dc2")

	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)

	dc1Prefix := DcPrefix(t, f, dc1Key)
	dc2Prefix := DcPrefix(t, f, dc2Key)
	reaperStargate2Prefix := DcPrefixOverride(t, f, dc2Key)
	sg1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, fmt.Sprintf("%s-stargate", dc1Prefix))
	reaper1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, fmt.Sprintf("%s-reaper", dc1Prefix))
	sg2Key := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, fmt.Sprintf("%s-stargate", reaperStargate2Prefix))
	reaper2Key := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, fmt.Sprintf("%s-reaper", reaperStargate2Prefix))

	checkStargateReady(t, f, ctx, sg1Key)
	checkReaperReady(t, f, ctx, reaper1Key)
	checkStargateReady(t, f, ctx, sg2Key)

	toggleDcStopped(t, f, ctx, kcKey, dc1Key, true)

	t.Logf("Check stargate1 stopped and reaper moved to dc2")
	// checkStargateNotFound(t, f, ctx, sg1Key)
	checkReaperNotFound(t, f, ctx, reaper1Key)
	checkReaperReady(t, f, ctx, reaper2Key)

	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], kcKey.Namespace, "cluster1")
	require.NoError(t, err)

	t.Log("deploying Stargate and Reaper ingress routes in", f.DataPlaneContexts[1])
	// stargateRestHostAndPort := ingressConfigs[f.DataPlaneContexts[1]].StargateRest
	// stargateGrpcHostAndPort := ingressConfigs[f.DataPlaneContexts[1]].StargateGrpc
	// stargateCqlHostAndPort := ingressConfigs[f.DataPlaneContexts[1]].StargateCql
	reaperRestHostAndPort := ingressConfigs[f.DataPlaneContexts[1]].ReaperRest
	// f.DeployStargateIngresses(t, f.DataPlaneContexts[1], namespace, fmt.Sprintf("%s-stargate-service", reaperStargate2Prefix), stargateRestHostAndPort, stargateGrpcHostAndPort)
	f.DeployReaperIngresses(t, f.DataPlaneContexts[1], namespace, fmt.Sprintf("%s-reaper-service", reaperStargate2Prefix), reaperRestHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[1], namespace)
	// checkStargateApisReachable(t, ctx, f.DataPlaneContexts[1], namespace, reaperStargate2Prefix, stargateRestHostAndPort, stargateGrpcHostAndPort, stargateCqlHostAndPort, username, password, false, f)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	pod1Name := fmt.Sprintf("%s-default-sts-0", dc1Prefix)
	pod2Name := fmt.Sprintf("%s-default-sts-0", dc2Prefix)

	checkKeyspaceReplicationsUnaltered(t, f, ctx, f.DataPlaneContexts[1], namespace, pod2Name, DcName(t, f, dc1Key), DcName(t, f, dc2Key))

	t.Run("TestApisDc1Stopped", func(t *testing.T) {
		// testStargateApis(t, f, ctx, f.DataPlaneContexts[1], namespace, reaperStargate2Prefix, username, password, false, map[string]int{DcName(t, f, dc2Key): 1})
		uiKey := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, reaper.DefaultUiSecretName("cluster1"))
		uiUsername, uiPassword := retrieveCredentials(t, f, ctx, uiKey)
		connectReaperApi(t, ctx, f.DataPlaneContexts[1], "cluster1", uiUsername, uiPassword)
		checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod2Name, 1, 1, "-u", username, "-pw", password)
	})

	toggleDcStopped(t, f, ctx, kcKey, dc2Key, true)

	t.Logf("Check stargate2 stopped and reaper2 stopped")
	// checkStargateNotFound(t, f, ctx, sg2Key)
	checkReaperNotFound(t, f, ctx, reaper2Key)

	toggleDcStopped(t, f, ctx, kcKey, dc1Key, false)

	t.Logf("Check stargate1 started and reaper moved to dc1")
	checkStargateReady(t, f, ctx, sg1Key)
	checkReaperReady(t, f, ctx, reaper1Key)

	t.Log("deploying Stargate and Reaper ingress routes in", f.DataPlaneContexts[0])
	// stargateRestHostAndPort = ingressConfigs[f.DataPlaneContexts[0]].StargateRest
	// stargateGrpcHostAndPort = ingressConfigs[f.DataPlaneContexts[0]].StargateGrpc
	// stargateCqlHostAndPort = ingressConfigs[f.DataPlaneContexts[0]].StargateCql
	reaperRestHostAndPort = ingressConfigs[f.DataPlaneContexts[0]].ReaperRest
	// f.DeployStargateIngresses(t, f.DataPlaneContexts[0], namespace, fmt.Sprintf("%s-stargate-service", dc1Prefix), stargateRestHostAndPort, stargateGrpcHostAndPort)
	f.DeployReaperIngresses(t, f.DataPlaneContexts[0], namespace, fmt.Sprintf("%s-reaper-service", dc1Prefix), reaperRestHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	// checkStargateApisReachable(t, ctx, f.DataPlaneContexts[0], namespace, dc1Prefix, stargateRestHostAndPort, stargateGrpcHostAndPort, stargateCqlHostAndPort, username, password, false, f)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	checkKeyspaceReplicationsUnaltered(t, f, ctx, f.DataPlaneContexts[0], namespace, pod1Name, DcName(t, f, dc1Key), DcName(t, f, dc2Key))

	t.Run("TestApisDc2Stopped", func(t *testing.T) {
		// testStargateApis(t, f, ctx, f.DataPlaneContexts[0], namespace, dc1Prefix, username, password, false, map[string]int{DcName(t, f, dc1Key): 1})
		uiKey := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, reaper.DefaultUiSecretName("cluster1"))
		uiUsername, uiPassword := retrieveCredentials(t, f, ctx, uiKey)
		connectReaperApi(t, ctx, f.DataPlaneContexts[0], "cluster1", uiUsername, uiPassword)
		checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, pod1Name, 1, 1, "-u", username, "-pw", password)
	})

	toggleDcStopped(t, f, ctx, kcKey, dc2Key, false)

	t.Logf("Check stargate2 started and reaper remains on dc1")
	checkStargateReady(t, f, ctx, sg2Key)
	checkReaperNotFound(t, f, ctx, reaper2Key)

	t.Run("TestApisDcsRestarted", func(t *testing.T) {
		// testStargateApis(t, f, ctx, f.DataPlaneContexts[0], namespace, dc1Prefix, username, password, false, map[string]int{DcName(t, f, dc1Key): 1, DcName(t, f, dc2Key): 1})
		// testStargateApis(t, f, ctx, f.DataPlaneContexts[1], namespace, reaperStargate2Prefix, username, password, false, map[string]int{DcName(t, f, dc1Key): 1, DcName(t, f, dc2Key): 1})
		uiKey := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, reaper.DefaultUiSecretName("cluster1"))
		uiUsername, uiPassword := retrieveCredentials(t, f, ctx, uiKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], "cluster1", reaperapi.DefaultKeyspace, uiUsername, uiPassword)
		checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, pod1Name, 2, 0, "-u", username, "-pw", password)
		checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod2Name, 2, 0, "-u", username, "-pw", password)
	})
}

// func checkStargateNotFound(t *testing.T, f *framework.E2eFramework, ctx context.Context, sgKey framework.ClusterKey) {
// 	require.Eventually(t, func() bool {
// 		sg := &stargateapi.Stargate{}
// 		return errors.IsNotFound(f.Get(ctx, sgKey, sg))
// 	}, polling.stargateReady.timeout, polling.stargateReady.interval)
// }

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
	patch := client.MergeFrom(kc.DeepCopy())
	kc.Spec.Cassandra.Datacenters[dcIndex].Stopped = stopped
	err = f.Client.Patch(ctx, kc, patch)
	require.NoError(t, err, "failed to patch kc")
	if stopped {
		checkDatacenterStopped(t, ctx, dcKey, f)
	} else {
		checkDatacenterReady(t, ctx, dcKey, f)
	}
}

func checkKeyspaceReplicationsUnaltered(t *testing.T, f *framework.E2eFramework, ctx context.Context, k8sContext string, namespace string, podName, dc1Name, dc2Name string) {
	t.Log("checking that keyspace replications didn't change for system and Stargate auth keyspaces")
	replication := map[string]int{dc1Name: 1, dc2Name: 1}
	checkKeyspaceReplication(t, f, ctx, k8sContext, namespace, "cluster1", podName, "system_auth", replication)
	checkKeyspaceReplication(t, f, ctx, k8sContext, namespace, "cluster1", podName, "system_traces", replication)
	checkKeyspaceReplication(t, f, ctx, k8sContext, namespace, "cluster1", podName, "system_distributed", replication)
	checkKeyspaceReplication(t, f, ctx, k8sContext, namespace, "cluster1", podName, stargate.AuthKeyspace, replication)
	checkKeyspaceReplication(t, f, ctx, k8sContext, namespace, "cluster1", podName, reaperapi.DefaultKeyspace, replication)
}
