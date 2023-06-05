package e2e

import (
	"context"
	"fmt"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusa "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	medusapkg "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	backupName  = "backup1"
	clusterName = "test"
)

func createSingleMedusaJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")
	require.NoError(f.InstallMinioOperator(), "Failed to install the MinIO operator")
	require.NoError(f.CreateMinioTenant(namespace), "Failed to create the MinIO tenant")
	kcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}
	kc := &api.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
	createBackupJob(t, ctx, namespace, f, dcKey)
	verifyBackupJobFinished(t, ctx, f, dcKey, backupKey)
	restoreBackupJob(t, ctx, namespace, f, dcKey)
	verifyRestoreJobFinished(t, ctx, f, dcKey, backupKey)
}

func createMultiMedusaJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")
	require.NoError(f.InstallMinioOperator(), "Failed to install the MinIO operator")
	require.NoError(f.CreateMinioTenant(namespace), "Failed to create the MinIO tenant")
	kcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}
	kc := &api.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}
	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	// Check that both DCs are ready and have Medusa containers
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkDatacenterReady(t, ctx, dcKey, f)
		checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
		checkMedusaStandaloneDeploymentExists(t, ctx, dcKey, f, kc)
		checkMedusaStandaloneServiceExists(t, ctx, dcKey, f, kc)
	}

	// Create a backup in each DC and verify their completion
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		createBackupJob(t, ctx, namespace, f, dcKey)
	}
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		verifyBackupJobFinished(t, ctx, f, dcKey, backupKey)
	}

	// Restore the backup in each DC and verify it finished correctly
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		restoreBackupJob(t, ctx, namespace, f, dcKey)
	}
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		verifyRestoreJobFinished(t, ctx, f, dcKey, backupKey)
	}
}

func checkMedusaContainersExist(t *testing.T, ctx context.Context, namespace string, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *api.K8ssandraCluster) {
	require := require.New(t)
	// Get the Cassandra pod
	dc1 := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc1)
	// check medusa containers exist
	require.NoError(err, "Error getting the CassandraDatacenter")
	t.Log("Checking that all the Medusa related objects have been created and are in the expected state")

	// Check containers presence
	_, found := cassandra.FindInitContainer(dc1.Spec.PodTemplateSpec, "medusa-restore")
	require.True(found, fmt.Sprintf("%s doesn't have medusa-restore init container", dc1.Name))
	_, foundConfig := cassandra.FindInitContainer(dc1.Spec.PodTemplateSpec, "server-config-init")
	require.True(foundConfig, fmt.Sprintf("%s doesn't have server-config-init container", dc1.Name))
	_, found = cassandra.FindContainer(dc1.Spec.PodTemplateSpec, "medusa")
	require.True(found, fmt.Sprintf("%s doesn't have medusa container", dc1.Name))
}

func createBackupJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework, dcKey framework.ClusterKey) {
	require := require.New(t)
	t.Log("creating MedusaBackupJob")

	backup := &medusa.MedusaBackupJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      backupName,
		},
		Spec: medusa.MedusaBackupJobSpec{
			CassandraDatacenter: dcKey.Name,
		},
	}

	err := f.Create(ctx, dcKey, backup)
	require.NoError(err, "failed to create MedusaBackupJob")
}

func verifyBackupJobFinished(t *testing.T, ctx context.Context, f *framework.E2eFramework, dcKey framework.ClusterKey, backupKey types.NamespacedName) {
	require := require.New(t)
	t.Log("verify the backup finished")
	backupClusterKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: backupKey}
	require.Eventually(func() bool {
		updated := &medusa.MedusaBackupJob{}
		err := f.Get(ctx, backupClusterKey, updated)
		require.NoError(err, "failed to get MedusaBackupJob")

		t.Logf("backup finish time: %v", updated.Status.FinishTime)
		t.Logf("backup finished: %v", updated.Status.Finished)
		t.Logf("backup in progress: %v", updated.Status.InProgress)
		return !updated.Status.FinishTime.IsZero() && len(updated.Status.InProgress) == 0
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "backup didn't finish within timeout")
}

func restoreBackupJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework, dcKey framework.ClusterKey) {
	require := require.New(t)
	t.Log("restoring MedusaBackup")
	restore := &medusa.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-restore",
		},
		Spec: medusa.MedusaRestoreJobSpec{
			Backup:              backupName,
			CassandraDatacenter: dcKey.Name,
		},
	}

	restoreKey := types.NamespacedName{Namespace: namespace, Name: "test-restore"}
	restoreClusterKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: restoreKey}

	err := f.Create(ctx, restoreClusterKey, restore)
	require.NoError(err, "failed to restore MedusaBackup")
}

func verifyRestoreJobFinished(t *testing.T, ctx context.Context, f *framework.E2eFramework, dcKey framework.ClusterKey, backupKey types.NamespacedName) {
	require := require.New(t)

	// The datacenter should then restart after the restore
	checkDatacenterReady(t, ctx, dcKey, f)
	restoreKey := types.NamespacedName{Namespace: backupKey.Namespace, Name: "test-restore"}
	restoreClusterKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: restoreKey}

	t.Log("verify the restore finished")
	require.Eventually(func() bool {
		restore := &medusa.MedusaRestoreJob{}
		err := f.Get(ctx, restoreClusterKey, restore)
		if err != nil {
			return false
		}

		return !restore.Status.FinishTime.IsZero()
	}, polling.medusaRestoreDone.timeout, polling.medusaRestoreDone.interval, "restore didn't finish within timeout")
}

func checkMedusaStandaloneDeploymentExists(t *testing.T, ctx context.Context, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *api.K8ssandraCluster) {
	t.Log("Checking that the Medusa standalone pod has been created")
	require := require.New(t)
	// Get the medusa standalone pod and check that it is running
	require.Eventually(func() bool {
		deployment := &appsv1.Deployment{}
		deploymentKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: types.NamespacedName{Namespace: dcKey.Namespace, Name: medusapkg.MedusaStandaloneDeploymentName(kc.Name, dcKey.Name)}}
		err := f.Get(ctx, deploymentKey, deployment)
		require.NoError(err, "Error getting the medusa standalone pod")
		return deployment.Status.ReadyReplicas == 1
	}, polling.medusaReady.timeout, polling.medusaReady.interval, "Medusa standalone pod is not running")
}

func checkMedusaStandaloneServiceExists(t *testing.T, ctx context.Context, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *api.K8ssandraCluster) {
	t.Log("Checking that the Medusa standalone service has been created")
	require := require.New(t)
	// Get the medusa standalone pod and check that it is running
	require.Eventually(func() bool {
		service := &corev1.Service{}
		serviceKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: types.NamespacedName{Namespace: dcKey.Namespace, Name: medusapkg.MedusaServiceName(kc.Name, dcKey.Name)}}
		err := f.Get(ctx, serviceKey, service)
		return err == nil
	}, polling.medusaReady.timeout, polling.medusaReady.interval, "Medusa standalone service doesn't exist")
}
