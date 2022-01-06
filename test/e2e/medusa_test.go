package e2e

import (
	"context"
	"fmt"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusa "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	backupName  = "backup1"
	clusterName = "test"
)

func createSingleMedusa(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	kcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}
	kc := &api.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(t, err, "Error getting the K8ssandraCluster")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
	createBackup(t, ctx, namespace, f, dcKey)
	verifyBackupFinished(t, ctx, f, dcKey, backupKey)
	restoreBackup(t, ctx, namespace, f, dcKey, kcKey)
	verifyRestoreFinished(t, ctx, f, dcKey, backupKey)
}

func createMultiMedusa(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	kcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}
	kc := &api.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(t, err, "Error getting the K8ssandraCluster")
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}
	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	// Check that both DCs are ready and have Medusa containers
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkDatacenterReady(t, ctx, dcKey, f)
		checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
	}

	// Create a backup in each DC and verify their completion
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		createBackup(t, ctx, namespace, f, dcKey)
	}
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		verifyBackupFinished(t, ctx, f, dcKey, backupKey)
	}

	// Restore the backup in each DC and verify it finished correctly
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		restoreBackup(t, ctx, namespace, f, dcKey, kcKey)
	}
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		verifyRestoreFinished(t, ctx, f, dcKey, backupKey)
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

func createBackup(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework, dcKey framework.ClusterKey) {
	require := require.New(t)
	t.Log("creating CassandraBackup")

	backup := &medusa.CassandraBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      backupName,
		},
		Spec: medusa.CassandraBackupSpec{
			Name:                backupName,
			CassandraDatacenter: dcKey.Name,
		},
	}

	err := f.Create(ctx, dcKey, backup)
	require.NoError(err, "failed to create CassandraBackup")
}

func verifyBackupFinished(t *testing.T, ctx context.Context, f *framework.E2eFramework, dcKey framework.ClusterKey, backupKey types.NamespacedName) {
	require := require.New(t)
	t.Log("verify the backup finished")
	backupClusterKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: backupKey}
	require.Eventually(func() bool {
		updated := &medusa.CassandraBackup{}
		err := f.Get(ctx, backupClusterKey, updated)
		require.NoError(err, "failed to get CassandraBackup")

		t.Logf("backup finish time: %v", updated.Status.FinishTime)
		t.Logf("backup finished: %v", updated.Status.Finished)
		t.Logf("backup in progress: %v", updated.Status.InProgress)
		return !updated.Status.FinishTime.IsZero() && len(updated.Status.InProgress) == 0
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "backup didn't finish within timeout")
}

func restoreBackup(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework, dcKey framework.ClusterKey, kcKey framework.ClusterKey) {
	require := require.New(t)
	t.Log("restoring CassandraBackup")
	restore := &medusa.CassandraRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-restore",
		},
		Spec: medusa.CassandraRestoreSpec{
			Backup:   backupName,
			Shutdown: true,
			InPlace:  true,
			CassandraDatacenter: medusa.CassandraDatacenterConfig{
				Name:        dcKey.Name,
				ClusterName: clusterName,
			},
		},
	}

	restoreKey := types.NamespacedName{Namespace: namespace, Name: "test-restore"}
	restoreClusterKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: restoreKey}

	err := f.Create(ctx, restoreClusterKey, restore)
	require.NoError(err, "failed to restore CassandraBackup")

	// The datacenter should stop for the restore to happen
	checkDatacenterUpdating(t, ctx, dcKey, f)
}

func verifyRestoreFinished(t *testing.T, ctx context.Context, f *framework.E2eFramework, dcKey framework.ClusterKey, backupKey types.NamespacedName) {
	require := require.New(t)

	// The datacenter should then restart after the restore
	checkDatacenterReady(t, ctx, dcKey, f)
	restoreKey := types.NamespacedName{Namespace: backupKey.Namespace, Name: "test-restore"}
	restoreClusterKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: restoreKey}

	t.Log("verify the restore finished")
	require.Eventually(func() bool {
		restore := &medusa.CassandraRestore{}
		err := f.Get(ctx, restoreClusterKey, restore)
		if err != nil {
			return false
		}

		return !restore.Status.FinishTime.IsZero()
	}, polling.medusaRestoreDone.timeout, polling.medusaRestoreDone.interval, "restore didn't finish within timeout")
}
