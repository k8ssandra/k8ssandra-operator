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
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusa "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	medusapkg "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
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
	kc := &api.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
	checkPurgeCronJobExists(t, ctx, namespace, dcKey, f, kc)
	checkDatacenterReadOnlyRootFS(t, ctx, dcKey, f, kc)

	// Disable purges
	err = f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	medusaPurgePatch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Medusa.PurgeBackups = ptr.To(false)
	err = f.Client.Patch(ctx, kc, medusaPurgePatch)
	require.NoError(err, "failed to patch K8ssandraCluster with purge modification in namespace %s", namespace)
	checkPurgeCronJobDeleted(t, ctx, namespace, dcKey, f, kc)

	createBackupJob(t, ctx, namespace, f, dcKey)
	verifyBackupJobFinished(t, ctx, f, dcKey, backupKey)
	checkPurgeTaskWasCreated(t, ctx, namespace, dcKey, f, kc)
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
	kc := &api.K8ssandraCluster{}
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
		checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
		checkPurgeCronJobExists(t, ctx, namespace, dcKey, f, kc)
		checkReplicatedSecretMounted(t, ctx, f, dcKey, localBucketSecretName)
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

func createMultiDcSingleMedusaJob(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	cluster2Name := "cluster2"
	kc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: cluster2Name}}
	kc := &api.K8ssandraCluster{}
	err := f.Get(ctx, kc2Key, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster2-dc1"}}
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}

	checkDatacenterReady(t, ctx, dcKey, f)
	checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
	createBackupJob(t, ctx, namespace, f, dcKey)
	verifyBackupJobFinished(t, ctx, f, dcKey, backupKey)
	checkNoPurgeCronJob(t, ctx, namespace, dcKey, f, kc)
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

func checkPurgeCronJobExists(t *testing.T, ctx context.Context, namespace string, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *api.K8ssandraCluster) {
	require := require.New(t)
	// Get the Cassandra pod
	dc1 := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc1)
	// check medusa containers exist
	require.NoError(err, "Error getting the CassandraDatacenter")
	t.Log("Checking that the purge Cron Job exists")
	// check that the cronjob exists
	cronJob := &batchv1.CronJob{}
	err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, medusapkg.MedusaPurgeCronJobName(kc.SanitizedName(), dc1.DatacenterName())), cronJob)
	require.NoErrorf(err, "Error getting the Medusa purge CronJob. ClusterName: %s, DatacenterName: %s", kc.SanitizedName(), dc1.LabelResourceName())
	require.Equal("k8ssandra-operator", cronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName, "Service account name is not correct")
	// create a Job from the cronjob spec
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-purge-job",
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}
	err = f.Create(ctx, dcKey, job)
	require.NoErrorf(err, "Error creating the Medusa purge Job. ClusterName: %s, DataceneterName: %s, Namespace: %s, JobName: test-purge-job", kc.SanitizedName(), dc1.LabelResourceName(), namespace)
	// ensure the job run was successful
	require.Eventually(func() bool {
		updated := &batchv1.Job{}
		err := f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, "test-purge-job"), updated)
		if err != nil {
			return false
		}
		return updated.Status.Succeeded == 1
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "Medusa purge Job didn't finish within timeout")
}

func checkNoPurgeCronJob(t *testing.T, ctx context.Context, namespace string, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *api.K8ssandraCluster) {
	require := require.New(t)
	t.Log("Checking that the purge Cron Job doesn't exist")
	// Get the Cassandra pod
	dc1 := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc1)
	// check medusa containers exist
	require.NoError(err, "Error getting the CassandraDatacenter")
	// ensure the cronjob was not created
	cronJob := &batchv1.CronJob{}
	err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, medusapkg.MedusaPurgeCronJobName(kc.SanitizedName(), dc1.DatacenterName())), cronJob)
	require.Error(err, "Cronjob should not exist")
}

func checkPurgeCronJobDeleted(t *testing.T, ctx context.Context, namespace string, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *api.K8ssandraCluster) {
	require := require.New(t)
	// Get the Cassandra pod
	dc1 := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc1)
	t.Log("Checking that the purge Cron Job was deleted")
	require.NoError(err, "Error getting the CassandraDatacenter")

	require.Eventually(func() bool {
		// ensure the cronjob was deleted
		cronJob := &batchv1.CronJob{}
		err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, medusapkg.MedusaPurgeCronJobName(kc.SanitizedName(), dc1.DatacenterName())), cronJob)
		return errors.IsNotFound(err)
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "Medusa purge CronJob wasn't deleted within timeout")
}

func checkPurgeTaskWasCreated(t *testing.T, ctx context.Context, namespace string, dcKey framework.ClusterKey, f *framework.E2eFramework, kc *api.K8ssandraCluster) {
	require := require.New(t)
	// list MedusaTask objects
	t.Log("Checking that the purge task was created")
	require.Eventually(func() bool {
		medusaTasks := &medusa.MedusaTaskList{}
		err := f.List(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, ""), medusaTasks)
		if err != nil {
			t.Logf("failed to list MedusaTasks: %v", err)
			return false
		}
		// check that the task is a purge task
		found := false
		for _, task := range medusaTasks.Items {
			if task.Spec.Operation == medusa.OperationTypePurge && task.Spec.CassandraDatacenter == dcKey.Name {
				found = true
				break
			}
		}
		return found
	}, 2*time.Minute, 5*time.Second, "Medusa purge task wasn't created within timeout")

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
