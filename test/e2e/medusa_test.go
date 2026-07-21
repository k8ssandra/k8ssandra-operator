package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusa "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	medusapkg "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	backupName                   = "backup1"
	clusterName                  = "test"
	localBucketSecretName        = "medusa-bucket-key"
	multiClusterBucketSecretName = "multicluster-medusa-bucket-key"
)

func createSingleMedusaJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")
	kcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}
	kc := &k8ssandraapi.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkMedusaContainersExist(t, ctx, dcKey, f)
	checkPurgeBackupScheduleExists(t, ctx, dcKey, f, kc)
	checkDatacenterReadOnlyRootFS(t, ctx, dcKey, f, kc)

	// Disable purges
	err = f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	medusaPurgePatch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Medusa.PurgeBackups = ptr.To(false)
	err = f.Client.Patch(ctx, kc, medusaPurgePatch)
	require.NoError(err, "failed to patch K8ssandraCluster with purge modification in namespace %s", namespace)
	checkPurgeBackupScheduleDeleted(t, ctx, dcKey, f, kc)

	createBackupJob(t, ctx, namespace, f, dcKey)
	verifyBackupJobFinished(t, ctx, f, dcKey, backupKey)
	restoreBackupJob(t, ctx, namespace, f, dcKey)
	verifyRestoreJobFinished(t, ctx, f, dcKey, backupKey)

	// Scale the cluster to verify that the previous restore won't break the new pod
	// Not doing this for DSE tests as it takes too long
	if kc.Spec.Cassandra.ServerType == "cassandra" {
		t.Log("Scaling the cluster to 3 nodes")
		err = f.Get(ctx, kcKey, kc)
		require.NoError(err, "Error getting the K8ssandraCluster")
		kcPatch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
		kc.Spec.Cassandra.Datacenters[0].Size = 3
		err = f.Client.Patch(ctx, kc, kcPatch)
		require.NoError(err, "Error scaling the cluster")
		checkDatacenterReady(t, ctx, dcKey, f)
	}
}

func createMultiMedusaJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")
	kcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}
	kc := &k8ssandraapi.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}
	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	// Check that Medusa's bucket key has been replicated to the current namespace
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		verifyBucketKeyPresent(t, f, ctx, kc, namespace, dcKey.K8sContext, localBucketSecretName)
	}

	// Check that both DCs are ready and have Medusa containers
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkDatacenterReady(t, ctx, dcKey, f)
		checkMedusaContainersExist(t, ctx, dcKey, f)
		checkPurgeBackupScheduleExists(t, ctx, dcKey, f, kc)
		checkReplicatedSecretMounted(t, ctx, f, dcKey, kc.SanitizedName()+"-"+localBucketSecretName)
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
		verifyRestoreJobFinished(t, ctx, f, dcKey, backupKey)
	}
}

func createMultiDcSingleMedusaJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	cluster2Name := "cluster2"
	kc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: cluster2Name}}
	kc := &k8ssandraapi.K8ssandraCluster{}
	err := f.Get(ctx, kc2Key, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster2-dc1"}}
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkMedusaContainersExist(t, ctx, dcKey, f)
	createBackupJob(t, ctx, namespace, f, dcKey)
	verifyBackupJobFinished(t, ctx, f, dcKey, backupKey)
	checkNoPurgeBackupSchedule(t, ctx, dcKey, f, kc)
}

func createMultiDatacenterMedusaCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	// dc1 is intentionally deployed to a namespace separate from the K8ssandraCluster to
	// exercise the cross-namespace DC scenario. The namespace is pre-created here because
	// the fixture references it before the operator has a chance to create it.
	const dc1Namespace = "separate-namespace"
	err := f.Client.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dc1Namespace}})
	require.NoError(err, "failed to create separate DC namespace %s", dc1Namespace)

	t.Log("check that the K8ssandraCluster was created")
	kc := &k8ssandraapi.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: clusterName}
	err = f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	// dc1 lives in dc1Namespace (separate from the KC); dc2 lives in the KC namespace.
	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: dc1Namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	// Test 1: Check that both datacenters are ready
	t.Log("checking that both datacenters are ready")
	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dc1Key.Name, dc2Key.Name)

	// Test 2: Verify Medusa storage secret is replicated into each DC's namespace
	t.Log("verifying that Medusa bucket key has been replicated to all contexts")
	verifyBucketKeyPresent(t, f, ctx, kc, dc1Namespace, dc1Key.K8sContext, kc.SanitizedName()+"-"+localBucketSecretName)
	verifyBucketKeyPresent(t, f, ctx, kc, namespace, dc2Key.K8sContext, kc.SanitizedName()+"-"+localBucketSecretName)

	// Test 3: Check that Medusa containers exist in both datacenters
	t.Log("checking that Medusa containers exist in both datacenters")
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkMedusaContainersExist(t, ctx, dcKey, f)
	}

	// Test 4: Verify that the replicated secret is mounted on Cassandra pods
	t.Log("verifying that replicated secret is mounted on Cassandra pods in both datacenters")
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkReplicatedSecretMounted(t, ctx, f, dcKey, kc.SanitizedName()+"-"+localBucketSecretName)
	}

	// Test 5: Check purge backup schedules exist
	t.Log("checking that purge backup schedules exist in both datacenters")
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkPurgeBackupScheduleExists(t, ctx, dcKey, f, kc)
	}

	// Test 6: Check ReplicatedSecret has correct labels — it lives in the KC namespace.
	t.Log("checking that ReplicatedSecret has been created in control plane with correct labels and annotations")
	kcClusterKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: kcKey}
	checkMedusaReplicatedSecretLabels(t, ctx, f, kcClusterKey, kc.Name)

	// Test 7: Retrieve database credentials — superuser secret is in the KC namespace.
	t.Log("retrieving database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName())
	require.NoError(err, "failed to retrieve database credentials")

	// Calculate expected node count dynamically from datacenter specs
	dc1 := &cassdcapi.CassandraDatacenter{}
	dc2 := &cassdcapi.CassandraDatacenter{}
	require.NoError(f.Get(ctx, dc1Key, dc1), "failed to get dc1")
	require.NoError(f.Get(ctx, dc2Key, dc2), "failed to get dc2")
	expectedNodeCount := int(dc1.Spec.Size + dc2.Spec.Size)

	// Test 8: Verify multi-DC topology — pods are in each DC's own namespace.
	t.Log("checking that nodes in dc1 see nodes in dc2")
	pod := DcPrefix(t, f, dc1Key) + "-default-sts-0"
	checkNodeToolStatus(t, f, f.DataPlaneContexts[0], dc1Namespace, pod, expectedNodeCount, 0, "-u", username, "-pw", password)

	t.Log("checking that nodes in dc2 see nodes in dc1")
	pod = DcPrefix(t, f, dc2Key) + "-default-sts-0"
	checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod, expectedNodeCount, 0, "-u", username, "-pw", password)

	// Test 9: Create backup in dc1 and verify completion — backup jobs go into the DC's namespace.
	t.Log("creating backup in dc1")
	dc1BackupKey := types.NamespacedName{Namespace: dc1Namespace, Name: backupName}
	createBackupJob(t, ctx, dc1Namespace, f, dc1Key)
	verifyBackupJobFinished(t, ctx, f, dc1Key, dc1BackupKey)

	// Test 10: Create backup in dc2 and verify completion
	t.Log("creating backup in dc2")
	dc2BackupKey := types.NamespacedName{Namespace: namespace, Name: backupName}
	createBackupJob(t, ctx, namespace, f, dc2Key)
	verifyBackupJobFinished(t, ctx, f, dc2Key, dc2BackupKey)

	// Test 11: Restore backup in dc1 and verify completion
	t.Log("restoring backup in dc1")
	restoreBackupJob(t, ctx, dc1Namespace, f, dc1Key)
	verifyRestoreJobFinished(t, ctx, f, dc1Key, dc1BackupKey)

	// Test 12: Restore backup in dc2 and verify completion
	t.Log("restoring backup in dc2")
	restoreBackupJob(t, ctx, namespace, f, dc2Key)
	verifyRestoreJobFinished(t, ctx, f, dc2Key, dc2BackupKey)

	// Test 13: Verify datacenters are still ready after restore
	t.Log("verifying that both datacenters are ready after restore")
	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dc1Key.Name, dc2Key.Name)

	// Test 14: Verify multi-DC topology is still intact after restore
	t.Log("verifying multi-DC topology after restore")

	t.Log("checking that nodes in dc1 see nodes in dc2")
	pod = DcPrefix(t, f, dc1Key) + "-default-sts-0"
	checkNodeToolStatus(t, f, f.DataPlaneContexts[0], dc1Namespace, pod, expectedNodeCount, 0, "-u", username, "-pw", password)

	t.Log("checking that nodes in dc2 see nodes in dc1")
	pod = DcPrefix(t, f, dc2Key) + "-default-sts-0"
	checkNodeToolStatus(t, f, f.DataPlaneContexts[1], namespace, pod, expectedNodeCount, 0, "-u", username, "-pw", password)

	t.Log("multi-datacenter Medusa cluster test completed successfully")
}

func verifyBucketKeyPresent(t *testing.T, f *framework.E2eFramework, ctx context.Context, kc *k8ssandraapi.K8ssandraCluster, namespace, k8sContext, secretName string) {
	require := require.New(t)

	bucketKey := &corev1.Secret{}
	require.Eventually(func() bool {
		err := f.Get(ctx, framework.NewClusterKey(k8sContext, namespace, secretName), bucketKey)
		return err == nil
	}, polling.medusaConfigurationReady.timeout, polling.medusaConfigurationReady.interval,
		fmt.Sprintf("Error getting the Medusa bucket key secret. Context: %s, ClusterName: %s, Namespace: %s", k8sContext, kc.SanitizedName(), namespace),
	)
}

func checkReplicatedSecretMounted(t *testing.T, ctx context.Context, f *framework.E2eFramework, dcKey framework.ClusterKey, secretName string) {
	require := require.New(t)
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(err, fmt.Sprintf("Error getting the CassandraDatacenter %s", dcKey.Name))
	index, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "medusa")
	require.True(found, fmt.Sprintf("%s doesn't have medusa container", dc.Name))
	medusaContainer := dc.Spec.PodTemplateSpec.Spec.Containers[index]
	hasMount := f.ContainerHasVolumeMount(medusaContainer, secretName, "/etc/medusa-secrets")
	assert.True(t, hasMount, "Missing Volume Mount for Medusa bucket key")
}

func checkMedusaContainersExist(t *testing.T, ctx context.Context, dcKey framework.ClusterKey, f *framework.E2eFramework) {
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

func checkPurgeBackupScheduleExists(t *testing.T, ctx context.Context, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *k8ssandraapi.K8ssandraCluster) {
	require := require.New(t)
	dcNamespace := dcKey.Namespace
	if dcNamespace == "" {
		dcNamespace = kc.Namespace
	}
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(err, "Error getting the CassandraDatacenter")
	t.Log("Checking that the purge schedule exists")
	scheduleName := medusapkg.MedusaPurgeScheduleName(kc.SanitizedName(), dc.DatacenterName())
	require.Eventually(func() bool {
		backupSchedule := &medusa.MedusaBackupSchedule{}
		err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, dcNamespace, scheduleName), backupSchedule)
		return err == nil
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "Medusa purge schedule %s wasn't created within timeout. ClusterName: %s, DatacenterName: %s", scheduleName, kc.SanitizedName(), dc.DatacenterName())
}

func checkNoPurgeBackupSchedule(t *testing.T, ctx context.Context, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *k8ssandraapi.K8ssandraCluster) {
	require := require.New(t)
	dcNamespace := dcKey.Namespace
	if dcNamespace == "" {
		dcNamespace = kc.Namespace
	}
	// Get the Cassandra pod
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(err, "Error getting the CassandraDatacenter")
	t.Log("Checking that the purge schedule doesn't exist")
	backupSchedule := &medusa.MedusaBackupSchedule{}
	err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, dcNamespace, medusapkg.MedusaPurgeScheduleName(kc.SanitizedName(), dc.DatacenterName())), backupSchedule)
	require.Error(err, "MedusaBackupSchedule for purge should not exist for datacenter %s", dcKey.Name)
}

func checkPurgeBackupScheduleDeleted(t *testing.T, ctx context.Context, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *k8ssandraapi.K8ssandraCluster) {
	require := require.New(t)
	dcNamespace := dcKey.Namespace
	if dcNamespace == "" {
		dcNamespace = kc.Namespace
	}
	// Get the Cassandra pod
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	t.Log("Checking that the purge schedule was deleted")
	require.NoError(err, "Error getting the CassandraDatacenter")

	require.Eventually(func() bool {
		// ensure the cronjob was deleted
		backupSchedule := &medusa.MedusaBackupSchedule{}
		err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, dcNamespace, medusapkg.MedusaPurgeScheduleName(kc.SanitizedName(), dc.DatacenterName())), backupSchedule)
		return errors.IsNotFound(err)
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "Medusa purge backup schedule wasn't deleted within timeout")
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

	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(err, "failed to get CassandraDatacenter")

	medusaBackupKey := framework.ClusterKey{K8sContext: dcKey.K8sContext, NamespacedName: types.NamespacedName{Namespace: backupKey.Namespace, Name: backupName}}
	medusaBackup := &medusa.MedusaBackup{}
	err = f.Get(ctx, medusaBackupKey, medusaBackup)
	require.NoError(err, "failed to get MedusaBackup")
	require.Equal(dc.Spec.Size, medusaBackup.Status.TotalNodes, "backup total nodes doesn't match dc nodes")
	require.Equal(dc.Spec.Size, medusaBackup.Status.FinishedNodes, "backup finished nodes doesn't match dc nodes")
	require.Equal(int(dc.Spec.Size), len(medusaBackup.Status.Nodes), "backup topology doesn't match dc topology")
	require.Equal(medusapkg.StatusType_SUCCESS.String(), medusaBackup.Status.Status, "backup topology doesn't match dc topology")
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
	// Wait for the pods to restart
	t.Log("waiting for the pods to restart")
	require.Eventually(func() bool {
		// Check the pods lifetime is no more than 1 minute
		pods, err := f.GetCassandraDatacenterPods(t, ctx, dcKey, dcKey.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				t.Logf("failed to get CassandraDatacenter pods: %v", err)
				return false
			}
			return false
		}
		for _, pod := range pods {
			if pod.Status.StartTime.IsZero() || time.Since(pod.Status.StartTime.Time) > 1*time.Minute {
				return false
			}
		}
		return true
	}, polling.medusaRestoreDone.timeout, polling.medusaRestoreDone.interval, "pods didn't restart within timeout")

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

	require.Eventually(func() bool {
		dc := &cassdcapi.CassandraDatacenter{}
		err := f.Get(ctx, dcKey, dc)
		if err != nil {
			t.Log(err)
			return false
		}
		superUserSecret := dc.Spec.SuperuserSecretName
		if dc.Spec.SuperuserSecretName == "" {
			superUserSecret = cassdcapi.CleanupForKubernetes(dcKey.Name) + "-superuser"
		}
		secret := &corev1.Secret{}
		err = f.Get(ctx, framework.NewClusterKey(restoreClusterKey.K8sContext, restoreClusterKey.Namespace, superUserSecret), secret)
		if err != nil {
			t.Log(err)
			return false
		}
		_, exists := secret.Annotations[k8ssandraapi.RefreshAnnotation]
		return exists
	}, polling.medusaRestoreDone.timeout, polling.medusaRestoreDone.interval, "superuser secret wasn't updated with refresh annotation")
}

func createMedusaConfiguration(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	medusaConfig := &medusa.MedusaConfiguration{}
	medusaConfigKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "config1"}}
	err := f.Get(ctx, medusaConfigKey, medusaConfig)
	require.NoError(err, "Error getting the MedusaConfiguration")

	require.Eventually(func() bool {
		updated := &medusa.MedusaConfiguration{}
		err := f.Get(ctx, medusaConfigKey, updated)
		if err != nil {
			t.Logf("failed to get medusa configuration: %v", err)
			return false
		}
		for _, condition := range updated.Status.Conditions {
			if condition.Type == string(medusa.ControlStatusReady) {
				return condition.Status == metav1.ConditionTrue
			}
		}
		return false
	}, polling.medusaConfigurationReady.timeout, polling.medusaConfigurationReady.interval)
}

func checkMedusaReplicatedSecretLabels(t *testing.T, ctx context.Context, f *framework.E2eFramework, dcKey framework.ClusterKey, kcName string) {
	replicatedSecretName := types.NamespacedName{
		Namespace: dcKey.Namespace,
		Name:      kcName + "-medusa-storage-credentials",
	}
	replicatedSecretKey := framework.ClusterKey{
		NamespacedName: replicatedSecretName,
		K8sContext:     dcKey.K8sContext,
	}
	repSec := &replicationapi.ReplicatedSecret{}
	err := f.Get(ctx, replicatedSecretKey, repSec)
	require.NoError(t, err)
	require.Equal(t, kcName, repSec.Labels[k8ssandraapi.K8ssandraClusterNameLabel])
	require.Equal(t, dcKey.Namespace, repSec.Labels[k8ssandraapi.K8ssandraClusterNamespaceLabel])
}
