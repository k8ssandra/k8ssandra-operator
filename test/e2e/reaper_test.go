package e2e

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"

	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createSingleReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	kc := &api.K8ssandraCluster{}
	require.NoError(f.Client.Get(ctx, kcKey, kc), "Failed to get K8ssandraCluster")
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}

	checkDatacenterReady(t, ctx, dcKey, f)
	dcPrefix := DcPrefix(t, f, dcKey)
	reaperKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: dcPrefix + "-reaper"}}
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)

	// check that the Reaper Vector container and config map exist
	checkContainerPresence(t, ctx, f, reaperKey, kc, getPodTemplateSpec, reaper.VectorContainerName)
	checkVectorAgentConfigMapPresence(t, ctx, f, dcKey, reaper.VectorAgentConfigMapName)

	t.Logf("check that if Reaper Vector is disabled, the agent and configmap are deleted")
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)
	reaperVectorPatch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Reaper.Telemetry.Vector.Enabled = ptr.To(false)
	err = f.Client.Patch(ctx, kc, reaperVectorPatch)
	require.NoError(err, "failed to patch K8ssandraCluster in namespace %s", namespace)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)
	checkFinalizerRbacRule(t, f, ctx, namespace)
	checkContainerDeleted(t, ctx, f, reaperKey, kc, getPodTemplateSpec, reaper.VectorContainerName)
	checkVectorConfigMapDeleted(t, ctx, f, dcKey, reaper.VectorAgentConfigMapName)

	t.Logf("check that if Reaper Vector is enabled, the agent and configmap are re-created")
	err = f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)
	reaperVectorPatch = client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Reaper.Telemetry.Vector.Enabled = ptr.To(true)
	err = f.Client.Patch(ctx, kc, reaperVectorPatch)
	require.NoError(err, "failed to patch K8ssandraCluster in namespace %s", namespace)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)
	checkContainerPresence(t, ctx, f, reaperKey, kc, getPodTemplateSpec, reaper.VectorContainerName)
	checkVectorAgentConfigMapPresence(t, ctx, f, dcKey, reaper.VectorAgentConfigMapName)

	t.Log("check Reaper app type")
	checkReaperAppType(t, ctx, f, reaperKey, kc)

	t.Log("check Reaper keyspace created")
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), dcPrefix+"-default-sts-0", "reaper_db")

	testDeleteReaperManually(t, f, ctx, kcKey, dcKey, reaperKey)
	testRemoveReaperFromK8ssandraCluster(t, f, ctx, kcKey, dcKey, reaperKey)

	t.Log("deploying Reaper ingress routes in", f.DataPlaneContexts[0])
	reaperRestHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[0], namespace, dcPrefix+"-reaper-service", reaperRestHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	createEmptyKeyspaceTable(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), dcPrefix+"-default-sts-0", "test_ks", "test_table")

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.DataPlaneContexts[0])
		reaperUiSecretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "mycluster-reaper-ui"}}
		username, password := retrieveCredentials(t, f, ctx, reaperUiSecretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], DcClusterName(t, f, dcKey), "test_ks", username, password)
	})
}

func checkFinalizerRbacRule(t *testing.T, f *framework.E2eFramework, ctx context.Context, namespace string) {
	require := require.New(t)
	roleKey := types.NamespacedName{Namespace: namespace, Name: "k8ssandra-operator"}
	role := &rbacv1.Role{}
	require.NoError(f.Client.Get(ctx, roleKey, role), "Failed to get Role %s", roleKey)
	found := false
OuterLoop:
	for ruleIdx := range role.Rules {
		rule := &role.Rules[ruleIdx]
		if rule.Resources[0] == "reapers/finalizers" {
			for _, verb := range rule.Verbs {
				if verb == "update" {
					found = true
					break OuterLoop
				}
			}
		}
	}
	require.True(found, "Failed to find reaper finalizer update rule in Role %s", roleKey)
}

func createSingleReaperWithEncryption(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}

	checkDatacenterReady(t, ctx, dcKey, f)
	dcPrefix := DcPrefix(t, f, dcKey)
	reaperKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: dcPrefix + "-reaper"}}
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)

	t.Log("deploying Reaper ingress routes in context", f.DataPlaneContexts[0])
	reaperRestHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[0], namespace, dcPrefix+"-reaper-service", reaperRestHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.DataPlaneContexts[0])
		reaperUiSecretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-reaper-ui"}}
		username, password := retrieveCredentials(t, f, ctx, reaperUiSecretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], DcClusterName(t, f, dcKey), "reaper_db", username, password)
	})
}

func createMultiReaper(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	uiSecretKey := types.NamespacedName{Namespace: namespace, Name: "reaper-ui-secret"}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	kc := &api.K8ssandraCluster{}
	require.NoError(f.Client.Get(ctx, kcKey, kc), "Failed to get K8ssandraCluster")

	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)

	dc1Prefix := DcPrefix(t, f, dc1Key)
	dc2Prefix := DcPrefix(t, f, dc2Key)
	reaper2Prefix := DcPrefixOverride(t, f, dc2Key)

	reaper1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: dc1Prefix + "-reaper"}}
	reaper2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: reaper2Prefix + "-reaper"}}
	t.Log("check Reaper custom keyspace created in both clusters")
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), dc1Prefix+"-default-sts-0", "reaper_ks")
	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[1], namespace, kc.SanitizedName(), dc2Prefix+"-default-sts-0", "reaper_ks")

	checkReaperReady(t, f, ctx, reaper1Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc1Key)

	checkReaperReady(t, f, ctx, reaper2Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc2Key)

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName())
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("check that nodes in dc1 see nodes in dc2")
	checkNodeToolStatus(t, f, f.DataPlaneContexts[0], namespace, dc1Prefix+"-default-sts-0", 2, 0, "-u", username, "-pw", password)

	t.Log("check nodes in dc2 see nodes in dc1")
	checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, dc2Prefix+"-default-sts-0", 2, 0, "-u", username, "-pw", password)

	t.Log("deploying  Reaper ingress routes in all data plane clusters")
	reaperRestHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[0], namespace, dc1Prefix+"-reaper-service", reaperRestHostAndPort)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	reaperRestHostAndPort = ingressConfigs[f.DataPlaneContexts[1]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[1], namespace, reaper2Prefix+"-reaper-service", reaperRestHostAndPort)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[1], namespace)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], DcClusterName(t, f, dc1Key), "reaper_ks", username, password)
	})
	t.Run("TestReaperApi[1]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[1], DcClusterName(t, f, dc2Key), "reaper_ks", username, password)
	})
}

func createMultiReaperWithEncryption(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	uiSecretKey := types.NamespacedName{Namespace: namespace, Name: "reaper-ui-secret"}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}

	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)

	dc1Prefix := DcPrefix(t, f, dc1Key)
	reaper2Prefix := DcPrefixOverride(t, f, dc2Key)

	reaper1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: dc1Prefix + "-reaper"}}
	reaper2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: reaper2Prefix + "-reaper"}}

	checkReaperReady(t, f, ctx, reaper1Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc1Key)

	checkReaperReady(t, f, ctx, reaper2Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc2Key)

	t.Log("deploying Reaper ingress routes in both clusters")
	reaperRestHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[0], namespace, dc1Prefix+"-reaper-service", reaperRestHostAndPort)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)
	reaperRestHostAndPort = ingressConfigs[f.DataPlaneContexts[1]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[1], namespace, reaper2Prefix+"-reaper-service", reaperRestHostAndPort)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[1], namespace)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], DcClusterName(t, f, dc1Key), "reaper_ks", username, password)
	})
	t.Run("TestReaperApi[1]", func(t *testing.T) {
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: uiSecretKey}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[1], DcClusterName(t, f, dc2Key), "reaper_ks", username, password)
	})
}

func createReaperAndDatacenter(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	reaperKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "reaper1"}}

	checkDatacenterReady(t, ctx, dcKey, f)
	dcPrefix := DcPrefix(t, f, dcKey)
	t.Log("create Reaper keyspace")
	_, err := f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, "test", dcPrefix+"-rack1-sts-0",
		"CREATE KEYSPACE reaper_db WITH REPLICATION = {'class' : 'NetworkTopologyStrategy', '"+DcName(t, f, dcKey)+"' : 3} ")
	require.NoError(t, err, "failed to create Reaper keyspace")

	checkKeyspaceExists(t, f, ctx, f.DataPlaneContexts[0], namespace, "test", dcPrefix+"-rack1-sts-0", "reaper_db")

	checkReaperReady(t, f, ctx, reaperKey)

	t.Log("deploying Reaper ingress routes in context", f.DataPlaneContexts[0])
	reaperRestHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].ReaperRest
	f.DeployReaperIngresses(t, f.DataPlaneContexts[0], namespace, "reaper1-service", reaperRestHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.DataPlaneContexts[0])
		secretKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "reaper-ui-secret"}}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.DataPlaneContexts[0], DcClusterName(t, f, dcKey), "reaper_db", username, password)
	})
}

func createControlPlaneReaperAndDatacenter(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	reaperKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "reaper1"}}
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}

	checkReaperReady(t, f, ctx, reaperKey)
	checkDatacenterReady(t, ctx, dcKey, f)

	dcPrefix := DcPrefix(t, f, dcKey)

	createKeyspaceAndTable(t, f, ctx, f.DataPlaneContexts[0], namespace, "e2etestcluster", dcPrefix+"-default-sts-0", "test_ks", "test_table", 2)

	t.Log("deploying Reaper ingress routes in context", f.ControlPlaneContext)
	reaperRestHostAndPort := ingressConfigs[f.ControlPlaneContext].ReaperRest
	f.DeployReaperIngresses(t, f.ControlPlaneContext, namespace, "reaper1-service", reaperRestHostAndPort)
	defer f.UndeployAllIngresses(t, f.ControlPlaneContext, namespace)
	checkReaperApiReachable(t, ctx, reaperRestHostAndPort)

	t.Run("TestReaperApi[0]", func(t *testing.T) {
		t.Log("test Reaper API in context", f.ControlPlaneContext)
		secretKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "reaper-ui-secret"}}
		username, password := retrieveCredentials(t, f, ctx, secretKey)
		testReaperApi(t, ctx, f.ControlPlaneContext, DcClusterName(t, f, dcKey), "test_ks", username, password)
	})

	t.Log("verify Reaper keyspace absent")
	checkKeyspaceNeverCreated(t, f, ctx, f.DataPlaneContexts[0], namespace, "e2etestcluster", dcPrefix+"-default-sts-0", "reaper_db")
}

func checkReaperReady(t *testing.T, f *framework.E2eFramework, ctx context.Context, reaperKey framework.ClusterKey) {
	t.Logf("check that Reaper %s in cluster %s is ready", reaperKey.Name, reaperKey.K8sContext)
	withReaper := f.NewWithReaper(ctx, reaperKey)
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return reaper.Status.Progress == reaperapi.ReaperProgressRunning && reaper.Status.IsReady()
	}), polling.reaperReady.timeout, polling.reaperReady.interval)
}

func checkReaperK8cStatusReady(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dcKey framework.ClusterKey,
) {
	t.Log("check k8ssandra cluster status updated for Reaper")
	assert.Eventually(t, func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		}
		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		return found &&
			kdcStatus.Cassandra != nil &&
			cassandraDatacenterReady(kdcStatus.Cassandra) &&
			kdcStatus.Reaper != nil &&
			kdcStatus.Reaper.Progress == reaperapi.ReaperProgressRunning &&
			kdcStatus.Reaper.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")
}

func testDeleteReaperManually(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dcKey framework.ClusterKey,
	reaperKey framework.ClusterKey,
) {
	t.Log("check that if Reaper is deleted directly it gets re-created")
	reaper := &reaperapi.Reaper{}
	err := f.Client.Get(ctx, reaperKey.NamespacedName, reaper)
	require.NoError(t, err, "failed to get Reaper in namespace %s", reaperKey.Namespace)
	err = f.Client.Delete(ctx, reaper)
	require.NoError(t, err, "failed to delete Reaper in namespace %s", reaperKey.Namespace)
	checkDatacenterReady(t, ctx, dcKey, f)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)
}

func testRemoveReaperFromK8ssandraCluster(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dcKey framework.ClusterKey,
	reaperKey framework.ClusterKey,
) {
	t.Log("delete Reaper in k8ssandracluster CRD")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", kcKey.Namespace)
	patch := client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	reaperTemplate := k8ssandra.Spec.Reaper
	k8ssandra.Spec.Reaper = nil
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster in namespace %s", kcKey.Namespace)

	t.Log("check Reaper deleted")
	require.Eventually(t, func() bool {
		reaper := &reaperapi.Reaper{}
		err := f.Client.Get(ctx, reaperKey.NamespacedName, reaper)
		return err != nil && errors.IsNotFound(err)
	}, polling.reaperReady.timeout, polling.reaperReady.interval)

	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("check Reaper status deleted in k8ssandracluster resource")
	require.Eventually(t, func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		}
		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		return found && kdcStatus.Reaper == nil
	}, polling.reaperReady.timeout, polling.reaperReady.interval)

	t.Log("re-create Reaper in k8ssandracluster resource")
	err = f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", kcKey.Namespace)
	patch = client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	k8ssandra.Spec.Reaper = reaperTemplate.DeepCopy()
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster in namespace %s", kcKey.Namespace)

	t.Log("check Reaper re-created")
	withReaper := f.NewWithReaper(ctx, reaperKey)
	require.Eventually(t, withReaper(func(reaper *reaperapi.Reaper) bool {
		return true
	}), polling.reaperReady.timeout, polling.reaperReady.interval)

	checkDatacenterReady(t, ctx, dcKey, f)
	checkReaperReady(t, f, ctx, reaperKey)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)
}

func connectReaperApi(t *testing.T, ctx context.Context, k8sContext, clusterName, username, password string) reaperclient.Client {
	t.Logf("Testing Reaper API in context %v...", k8sContext)
	var reaperURL, _ = url.Parse(fmt.Sprintf("http://%s", ingressConfigs[k8sContext].ReaperRest))
	var reaperClient = reaperclient.NewClient(reaperURL)
	if username != "" {
		t.Logf("Logging into Reaper API in context %v...", k8sContext)
		err := reaperClient.Login(ctx, username, password)
		require.NoError(t, err, "failed to login into Reaper")
	}
	checkClusterIsRegisteredInReaper(t, ctx, clusterName, reaperClient)
	return reaperClient
}

func testReaperApi(t *testing.T, ctx context.Context, k8sContext, clusterName, keyspace, username, password string) {
	sanitizedClusterName := cassdcapi.CleanupForKubernetes(clusterName)
	reaperClient := connectReaperApi(t, ctx, k8sContext, sanitizedClusterName, username, password)
	repairId := triggerRepair(t, ctx, sanitizedClusterName, keyspace, reaperClient)
	t.Log("waiting for one segment to be repaired, then canceling run")
	waitForOneSegmentToBeDone(t, ctx, repairId, reaperClient)
	t.Log("a segment has completed")
	err := reaperClient.AbortRepairRun(ctx, repairId)
	require.NoErrorf(t, err, "Failed to abort repair run %s: %s", repairId, err)
	t.Log("repair aborted")
}

func checkClusterIsRegisteredInReaper(t *testing.T, ctx context.Context, clusterName string, reaperClient reaperclient.Client) {
	require.Eventually(t, func() bool {
		_, err := reaperClient.GetCluster(ctx, clusterName)
		return err == nil
	}, polling.reaperReady.timeout, polling.reaperReady.interval, "Cluster wasn't properly registered in Reaper")
}

func triggerRepair(t *testing.T, ctx context.Context, clusterName, keyspace string, reaperClient reaperclient.Client) uuid.UUID {
	t.Log("Starting a repair")
	options := &reaperclient.RepairRunCreateOptions{SegmentCountPerNode: 5}
	repairId, err := reaperClient.CreateRepairRun(ctx, clusterName, keyspace, "k8ssandra", options)
	require.NoErrorf(t, err, "Failed to create repair run: %s", err)
	// Start the previously created repair run
	err = reaperClient.StartRepairRun(ctx, repairId)
	require.NoErrorf(t, err, "Failed to start repair run %s: %s", repairId, err)
	return repairId
}

func waitForOneSegmentToBeDone(t *testing.T, ctx context.Context, repairId uuid.UUID, reaperClient reaperclient.Client) {
	require.Eventually(t, func() bool {
		if segments, err := reaperClient.RepairRunSegments(ctx, repairId); err == nil {
			for _, segment := range segments {
				if segment.State == reaperclient.RepairSegmentStateDone {
					return true
				}
			}
		}
		return false
	}, polling.reaperReady.timeout, polling.reaperReady.interval, "No repair segment was fully processed within timeout")
}
