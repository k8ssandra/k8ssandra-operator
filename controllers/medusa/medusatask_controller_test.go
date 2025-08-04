package medusa

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	backup1 = "good-backup1"
	backup2 = "good-backup2"
	backup3 = "good-backup3"
	backup4 = "good-backup4"
	backup5 = "bad-backup5"
	backup6 = "missing-backup6"
)

func testMedusaTasks(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	k8sCtx0 := f.DataPlaneContexts[0]

	kc := &k8ss.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: k8ss.K8ssandraClusterSpec{
			Cassandra: &k8ss.CassandraClusterTemplate{
				Datacenters: []k8ss.CassandraDatacenterTemplate{
					{
						Meta: k8ss.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: k8sCtx0,
						Size:       3,
						DatacenterOptions: k8ss.DatacenterOptions{
							ServerVersion: "3.11.14",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
							},
						},
					},
					{
						Meta: k8ss.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: k8sCtx0,
						Size:       3,
						DatacenterOptions: k8ss.DatacenterOptions{
							ServerVersion: "3.11.14",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
							},
						},
					},
				},
			},
			Medusa: &api.MedusaClusterTemplate{
				ContainerImage: &images.Image{
					Repository: medusaImageRepo,
				},
				StorageProperties: api.Storage{
					StorageProvider: "s3_compatible",
					BucketName:      "not-real",
					StorageSecretRef: corev1.LocalObjectReference{
						Name: cassandraUserSecret,
					},
					MaxBackupCount: 1,
				},
				ServiceProperties: api.Service{
					GrpcPort: 7890,
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: cassandraUserSecret,
				},
			},
		},
	}

	t.Log("Creating k8ssandracluster test1 with Medusa")
	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	kcKey := framework.NewClusterKey(f.ControlPlaneContext, namespace, "test")

	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
	dc2Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc2")

	reconcileReplicatedSecret(ctx, t, f, kc)

	for _, dcKey := range []framework.ClusterKey{dc1Key, dc2Key} {
		t.Logf("check that %s was created", dcKey.Name)
		require.Eventually(f.DatacenterExists(ctx, dcKey), timeout, interval)

		t.Log("update datacenter status to scaling up")
		err = f.PatchDatacenterStatus(ctx, dcKey, func(dc *cassdcapi.CassandraDatacenter) {
			dc.SetCondition(cassdcapi.DatacenterCondition{
				Type:               cassdcapi.DatacenterScalingUp,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			})
		})
		require.NoError(err, "failed to patch datacenter status")

		t.Log("check that the K8ssandraCluster status is updated")
		require.Eventually(func() bool {
			kc := &k8ss.K8ssandraCluster{}
			err = f.Get(ctx, kcKey, kc)

			if err != nil {
				t.Logf("failed to get K8ssandraCluster: %v", err)
				return false
			}

			if len(kc.Status.Datacenters) == 0 {
				return false
			}

			k8ssandraStatus, found := kc.Status.Datacenters[dcKey.Name]
			if !found {
				t.Logf("status for datacenter %s not found", dcKey)
				return false
			}
			condition := findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
			return condition != nil && condition.Status == corev1.ConditionTrue
		}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

		err = f.SetDatacenterStatusReady(ctx, dcKey)
		require.NoError(err, "failed to set dcKey status ready")
	}

	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	backup1Created := createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backup1)
	require.True(backup1Created, "failed to create backup1")
	backup2Created := createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backup2)
	require.True(backup2Created, "failed to create backup2")
	backup3Created := createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backup3)
	require.True(backup3Created, "failed to create backup3")
	backup4Created := createAndVerifyMedusaBackup(dc2Key, dc2, f, ctx, require, t, namespace, backup4)
	require.True(backup4Created, "failed to create backup4")
	backup5Created := createAndVerifyMedusaBackup(dc2Key, dc2, f, ctx, require, t, namespace, backup5)
	require.False(backup5Created, "failed to fail backup5")
	backup6Created := createAndVerifyMedusaBackup(dc2Key, dc2, f, ctx, require, t, namespace, backup6)
	require.False(backup6Created, "failed to fail backup6")

	// Ensure that 6 backups jobs, but only 4 backups were created (two jobs did not succeed on some pods)
	checkBackupsAndJobs(require, ctx, 6, 4, namespace, f, []string{})

	checkSyncTask(require, ctx, namespace, "dc2", f)

	// Ensure the sync task did not create backups for the failed jobs
	checkBackupsAndJobs(require, ctx, 6, 4, namespace, f, []string{})

	// Purge backups and verify that only one out of three remains
	t.Log("purge backups")

	purgeTask := &api.MedusaTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "purge-backups",
			Labels: map[string]string{
				"app": "medusa",
			},
		},
		Spec: api.MedusaTaskSpec{
			CassandraDatacenter: "dc1",
			Operation:           "purge",
		},
	}

	purgeKey := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "purge-backups")

	err = f.Create(ctx, purgeKey, purgeTask)
	require.NoError(err, "failed to create purge task")

	require.Eventually(func() bool {
		updated := &api.MedusaTask{}
		err := f.Get(ctx, purgeKey, updated)
		if err != nil {
			t.Logf("failed to get purge task: %v", err)
			return false
		}

		return !updated.Status.FinishTime.IsZero() && updated.Status.Finished[0].NbBackupsPurged == 2 && len(updated.Status.Finished) == 3
	}, timeout, interval)

	// After a purge, a sync should get created
	purgeSyncKey := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "purge-backups-sync")
	require.Eventually(func() bool {
		updated := &api.MedusaTask{}
		err := f.Get(ctx, purgeSyncKey, updated)
		if err != nil {
			t.Logf("failed to get sync task: %v", err)
			return false
		}

		v, ok := updated.Labels["app"]
		if !ok || v != "medusa" {
			return false
		}

		return !updated.Status.FinishTime.IsZero()
	}, timeout, interval)

	// Ensure that 2 backups and backup jobs were deleted
	deletedBackups := []string{backup1, backup2}
	checkBackupsAndJobs(require, ctx, 4, 2, namespace, f, deletedBackups)

	medusaBackup4Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, backup4)
	medusaBackup4 := &api.MedusaBackup{}
	err = f.Get(ctx, medusaBackup4Key, medusaBackup4)
	require.NoError(err, "failed to get medusaBackup4")

	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, dc2Key, &cassdcapi.CassandraDatacenter{})
}
func checkBackupsAndJobs(require *require.Assertions, ctx context.Context, expectedJobsLen, expectedBackupsLen int, namespace string, f *framework.Framework, deleted []string) {
	var backups api.MedusaBackupList
	err := f.List(ctx, framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "list-backups"), &backups)
	require.NoError(err, "failed to list medusabackup")
	require.Len(backups.Items, expectedBackupsLen, "expected %d backups, got %d", expectedBackupsLen, len(backups.Items))

	var jobs api.MedusaBackupJobList
	err = f.List(ctx, framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "list-backup-jobs"), &jobs)
	require.NoError(err, "failed to list medusabackupjobs")
	require.Len(jobs.Items, expectedJobsLen, "expected %d jobs, got %d", expectedJobsLen, len(jobs.Items))

	for _, d := range deleted {
		require.NotContains(backups.Items, d, "MedusaBackup %s to have been deleted", d)
		require.NotContains(jobs.Items, d, "MedusaBackupJob %s to have been deleted", d)
	}
}

func checkSyncTask(require *require.Assertions, ctx context.Context, namespace, dc string, f *framework.Framework) {
	syncTaskName := "sync-backups"

	// create a sync task
	syncTask := &api.MedusaTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      syncTaskName,
			Labels: map[string]string{
				"app": "medusa",
			},
		},
		Spec: api.MedusaTaskSpec{
			CassandraDatacenter: dc,
			Operation:           "sync",
		},
	}
	syncTaskKey := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, syncTaskName)
	err := f.Create(ctx, syncTaskKey, syncTask)
	require.NoError(err, "failed to create sync task")

	// wait for sync task to finish
	require.Eventually(func() bool {
		updated := &api.MedusaTask{}
		err := f.Get(ctx, syncTaskKey, updated)
		if err != nil {
			return false
		}

		v, ok := updated.Labels["app"]
		if !ok || v != "medusa" {
			return false
		}

		return !updated.Status.FinishTime.IsZero()
	}, timeout, interval)
}
