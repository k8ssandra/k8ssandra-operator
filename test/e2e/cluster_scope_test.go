package e2e

import (
	"context"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func multiDcMultiCluster(t *testing.T, ctx context.Context, klusterNamespace string, f *framework.E2eFramework) {
	require := require.New(t)

	dc1Namespace := "test-1"
	dc2Namespace := "test-2"
	reaperNamespace := "test-0"

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: klusterNamespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in operatorNamespace %s", klusterNamespace)

	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: dc1Namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dc1Key, f)
	checkBucketKeyPresent(t, f, ctx, klusterNamespace, dc1Key.K8sContext, k8ssandra)
	checkBucketKeyPresent(t, f, ctx, dc1Key.Namespace, dc1Key.K8sContext, k8ssandra)

	t.Log("check k8ssandra cluster status")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: klusterNamespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		cassandraStatus := getCassandraDatacenterStatus(k8ssandra, dc1Key.Name)
		if cassandraStatus == nil {
			return false
		}
		return cassandraDatacenterReady(cassandraStatus)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: dc2Namespace, Name: "dc2"}}
	checkDatacenterReady(t, ctx, dc2Key, f)
	checkBucketKeyPresent(t, f, ctx, dc2Namespace, dc2Key.K8sContext, k8ssandra)

	t.Log("check k8ssandra cluster status")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: klusterNamespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		cassandraStatus := getCassandraDatacenterStatus(k8ssandra, dc1Key.Name)
		if cassandraStatus == nil {
			return false
		}
		if !cassandraDatacenterReady(cassandraStatus) {
			return false
		}

		cassandraStatus = getCassandraDatacenterStatus(k8ssandra, dc2Key.Name)
		if cassandraStatus == nil {
			return false
		}
		return cassandraDatacenterReady(cassandraStatus)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	t.Log("check replicated secret mounted")
	checkReplicatedSecretMounted(t, ctx, f, dc1Key, dc1Namespace, k8ssandra)
	checkReplicatedSecretMounted(t, ctx, f, dc2Key, dc2Namespace, k8ssandra)

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], dc1Namespace, k8ssandra.SanitizedName())
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("check that nodes in dc1 see nodes in dc2")
	pod := DcPrefix(t, f, dc1Key) + "-rack1-sts-0"
	count := 4
	checkNodeToolStatus(t, f, f.DataPlaneContexts[0], dc1Namespace, pod, count, 0, "-u", username, "-pw", password)

	t.Log("check nodes in dc2 see nodes in dc1")
	pod = DcPrefix(t, f, dc2Key) + "-rack1-sts-0"
	checkNodeToolStatus(t, f, f.DataPlaneContexts[1], dc2Namespace, pod, count, 0, "-u", username, "-pw", password)

	t.Log("check that cluster was registered in Reaper")
	reaperKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: reaperNamespace, Name: "reaper1"}}
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: dc1Namespace, Name: "dc1"}}
	dcPrefix := DcPrefix(t, f, dcKey)
	checkReaperReady(t, f, ctx, reaperKey)
	createKeyspaceAndTable(t, f, ctx, f.DataPlaneContexts[0], dc1Namespace, k8ssandra.Name, dcPrefix+"-rack1-sts-0", "test_ks", "test_table", 2)

	t.Log("deploying Reaper ingress routes in context", f.ControlPlaneContext)
	reaperRestHostAndPort := ingressConfigs[f.ControlPlaneContext].ReaperRest
	f.DeployReaperIngresses(t, f.ControlPlaneContext, k8ssandra.Namespace, "reaper1-service", reaperRestHostAndPort)
	defer f.UndeployAllIngresses(t, f.ControlPlaneContext, k8ssandra.Namespace)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.ControlPlaneContext)
		secretKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: k8ssandra.Namespace, Name: "reaper-ui-secret"}}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.ControlPlaneContext, DcClusterName(t, f, dcKey), "test_ks", username, password)
	})
}
