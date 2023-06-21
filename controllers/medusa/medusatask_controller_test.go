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
	backup1 = "backup1"
	backup2 = "backup2"
	backup3 = "backup3"
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
				},
			},
			Medusa: &api.MedusaClusterTemplate{
				ContainerImage: &images.Image{
					Repository: medusaImageRepo,
				},
				StorageProperties: api.Storage{
					StorageSecretRef: corev1.LocalObjectReference{
						Name: cassandraUserSecret,
					},
					MaxBackupCount: 1,
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: cassandraUserSecret,
				},
			},
		},
	}

	t.Log("Creating k8ssandracluster with Medusa")
	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	reconcileReplicatedSecret(ctx, t, f, kc)
	reconcileMedusaStandaloneDeployment(ctx, t, f, kc, "dc1", f.DataPlaneContexts[0])
	t.Log("check that dc1 was created")
	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("update datacenter status to scaling up")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	kcKey := framework.NewClusterKey(f.ControlPlaneContext, namespace, "test")

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

		k8ssandraStatus, found := kc.Status.Datacenters[dc1Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc1Key)
			return false
		}

		condition := findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
		return condition != nil && condition.Status == corev1.ConditionTrue
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)

	t.Log("update dc1 status to ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to update dc1 status to ready")

	backup1Created := createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backup1)
	require.True(backup1Created, "failed to create backup1")
	backup2Created := createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backup2)
	require.True(backup2Created, "failed to create backup2")
	backup3Created := createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backup3)
	require.True(backup3Created, "failed to create backup3")

	// Purge backups and verify that only one out of three remains
	t.Log("purge backups")

	purgeTask := &api.MedusaTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "purge-backups",
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

		return !updated.Status.FinishTime.IsZero()
	}, timeout, interval)

	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
}
