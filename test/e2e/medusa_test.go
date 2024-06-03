package e2e

import (
	"context"
	"fmt"
	"testing"

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
	backupName             = "backup1"
	clusterName            = "test"
	globalBucketSecretName = "global-bucket-key"
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
	kc := &api.K8ssandraCluster{}
	err := f.Get(ctx, kcKey, kc)
	require.NoError(err, "Error getting the K8ssandraCluster")
	backupKey := types.NamespacedName{Namespace: namespace, Name: backupName}
	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	// Check that Medusa's bucket key has been replicated to the current namespace
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkBucketKeyPresent(t, f, ctx, namespace, dcKey.K8sContext, kc)
	}

	// Check that both DCs are ready and have Medusa containers
	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		checkDatacenterReady(t, ctx, dcKey, f)
		checkMedusaContainersExist(t, ctx, namespace, dcKey, f, kc)
		checkPurgeCronJobExists(t, ctx, namespace, dcKey, f, kc)
		checkReplicatedSecretMounted(t, ctx, f, dcKey, dcKey.Namespace, kc)
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
	checkNoPurgeCronJob(t, ctx, namespace, dcKey, f, kc)
	createBackupJob(t, ctx, namespace, f, dcKey)
	verifyBackupJobFinished(t, ctx, f, dcKey, backupKey)
}

func checkBucketKeyPresent(t *testing.T, f *framework.E2eFramework, ctx context.Context, namespace string, k8sContext string, kc *k8ssandraapi.K8ssandraCluster) {
	require := require.New(t)

	// work out the name of the replicated bucket key. should be "clusterName-<original-bucket-key-name>"
	localBucketKeyName := "test-" + globalBucketSecretName

	// Check that the bucket key has been replicated to the current namespace
	bucketKey := &corev1.Secret{}
	require.Eventually(func() bool {
		err := f.Get(ctx, framework.NewClusterKey(k8sContext, namespace, localBucketKeyName), bucketKey)
		return err == nil
	}, polling.medusaConfigurationReady.timeout, polling.medusaConfigurationReady.interval,
		fmt.Sprintf("Error getting the Medusa bucket key secret. Context: %s, ClusterName: %s, Namespace: %s", k8sContext, kc.SanitizedName(), namespace),
	)
}

func checkReplicatedSecretMounted(t *testing.T, ctx context.Context, f *framework.E2eFramework, dcKey framework.ClusterKey, namespace string, kc *api.K8ssandraCluster) {
	require := require.New(t)
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(err, fmt.Sprintf("Error getting the CassandraDatacenter %s", dcKey.Name))
	index, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "medusa")
	require.True(found, fmt.Sprintf("%s doesn't have medusa container", dc.Name))
	medusaContainer := dc.Spec.PodTemplateSpec.Spec.Containers[index]
	hasMount := f.ContainerHasVolumeMount(medusaContainer, fmt.Sprintf("%s-%s", clusterName, globalBucketSecretName), "/etc/medusa-secrets")
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
	t.Log("Checking that all the Medusa related objects have been created and are in the expected state")
	// check that the cronjob exists
	cronJob := &batchv1.CronJob{}
	err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, medusapkg.MedusaPurgeCronJobName(kc.SanitizedName(), dc1.SanitizedName())), cronJob)
	require.NoErrorf(err, "Error getting the Medusa purge CronJob. ClusterName: %s, DatacenterName: %s", kc.SanitizedName(), dc1.SanitizedName())
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
	require.NoErrorf(err, "Error creating the Medusa purge Job. ClusterName: %s, DataceneterName: %s, Namespace: %s, JobName: test-purge-job", kc.SanitizedName(), dc1.SanitizedName(), namespace)
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
	// Get the Cassandra pod
	dc1 := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc1)
	// check medusa containers exist
	require.NoError(err, "Error getting the CassandraDatacenter")
	t.Log("Checking that all the Medusa related objects have been created and are in the expected state")
	// check that the cronjob exists
	cronJob := &batchv1.CronJob{}
	err = f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, medusapkg.MedusaPurgeCronJobName(kc.SanitizedName(), dc1.SanitizedName())), cronJob)
	require.NoErrorf(err, "Error getting the Medusa purge CronJob. ClusterName: %s, DatacenterName: %s", kc.SanitizedName(), dc1.SanitizedName())
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
	require.NoErrorf(err, "A Medusa purge Job was created when it should not have been. ClusterName: %s, DataceneterName: %s, Namespace: %s, JobName: test-no-purge-job", kc.SanitizedName(), dc1.SanitizedName(), namespace)
	// ensure the job was not created
	require.Never(func() bool {
		updated := &batchv1.Job{}
		err := f.Get(ctx, framework.NewClusterKey(dcKey.K8sContext, namespace, "test-purge-job"), updated)
		if err != nil {
			return false
		}
		return updated.Status.Succeeded == 1
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "Timeout checking for Medusa purge Job")
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
