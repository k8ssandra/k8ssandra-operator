package medusa

import (
	"context"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	restoredBackupName = "backup2"
)

func testMedusaRestoreDatacenter(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	err := f.Client.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(namespace))
	require.NoError(err)
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
							DatacenterName: "real-dc1",
							ServerVersion:  "3.11.14",
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
				},
				ServiceProperties: api.Service{
					GrpcPort: 4567,
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: cassandraUserSecret,
				},
			},
		},
	}

	t.Log("Creating k8ssandracluster with Medusa")
	require.NoError(f.Client.Create(ctx, kc), "failed to create K8ssandraCluster")

	reconcileReplicatedSecret(ctx, t, f, kc)
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

	kcKey := framework.ClusterKey{K8sContext: k8sCtx0, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &k8ss.K8ssandraCluster{}
		err = f.Client.Get(ctx, kcKey.NamespacedName, kc)

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

	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)

	err = createCassandraPods(t, f, ctx, dc1Key, dc1)
	require.NoError(err)

	t.Log("creating MedusaBackup")
	backup := &api.MedusaBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      restoredBackupName,
		},
		Spec: api.MedusaBackupSpec{
			CassandraDatacenter: dc1.Name,
		},
	}

	backupKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restoredBackupName)
	err = f.Create(ctx, backupKey, backup)
	require.NoError(err, "failed to create CassandraBackup")

	patch := client.MergeFrom(backup.DeepCopy())
	backup.Status.FinishTime = metav1.Now()
	backup.Status.FinishedNodes = dc1.Spec.Size
	backup.Status.TotalNodes = dc1.Spec.Size
	backup.Status.Nodes = []*api.MedusaBackupNode{
		{
			Datacenter: "real-dc1",
			Rack:       "default",
		},
		{
			Datacenter: "real-dc1",
			Rack:       "default",
		},
		{
			Datacenter: "real-dc1",
			Rack:       "default",
		},
	}

	err = f.PatchStatus(ctx, backup, patch, backupKey)
	require.NoError(err, "failed to patch MedusaBackup")

	restore := &api.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-restore",
		},
		Spec: api.MedusaRestoreJobSpec{
			Backup:              restoredBackupName,
			CassandraDatacenter: dc1.Name,
		},
	}

	restoreKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restore.ObjectMeta.Name)
	err = f.Create(ctx, restoreKey, restore)
	require.NoError(err, "failed to create MedusaRestoreJob")

	withDc1 := f.NewWithDatacenter(ctx, dc1Key)

	t.Log("check that the datacenter is set to be stopped")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		return dc.Spec.Stopped == true
	}), timeout*5, interval, "timed out waiting for CassandraDatacenter stopped flag to be set")

	t.Log("delete datacenter pods to simulate shutdown")
	err = f.DeleteAllOf(ctx, dc1Key.K8sContext, &corev1.Pod{}, client.InNamespace(namespace), client.MatchingLabels{cassdcapi.DatacenterLabel: "dc1"})
	require.NoError(err, "failed to delete datacenter pods")

	restore = &api.MedusaRestoreJob{}
	err = f.Get(ctx, restoreKey, restore)
	require.NoError(err, "failed to get MedusaRestoreJob")

	dcStoppedTime := restore.Status.StartTime.Time.Add(1 * time.Second)

	t.Log("set datacenter status to stopped")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterStopped,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime),
		})
	})
	require.NoError(err, "failed to update datacenter status with stopped condition")

	t.Log("check that the datacenter podTemplateSpec is updated")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		restoreContainer := findContainer(dc.Spec.PodTemplateSpec.Spec.InitContainers, "medusa-restore")
		if restoreContainer == nil {
			t.Log("restore container not found")
			return false
		}

		envVar := utils.FindEnvVar(restoreContainer.Env, "BACKUP_NAME")
		if envVar == nil || envVar.Value != restoredBackupName {
			t.Logf("backup name not found in restore container: %v", restoreContainer.Env)
			return false
		}

		envVar = utils.FindEnvVar(restoreContainer.Env, "RESTORE_MAPPING")
		t.Logf("restore mapping: %v", envVar)
		if envVar == nil || envVar.Value == "" {
			t.Logf("restore mapping not foun d in restore container: %v", restoreContainer.Env)
			return false
		}

		envVar = utils.FindEnvVar(restoreContainer.Env, "RESTORE_KEY")
		t.Logf("restore key: %v", envVar)
		return envVar != nil
	}), timeout, interval, "timed out waiting for CassandraDatacenter PodTemplateSpec update")

	restore = &api.MedusaRestoreJob{}
	err = f.Get(ctx, restoreKey, restore)
	require.NoError(err, "failed to get MedusaRestoreJob")

	// In addition to checking Updating condition, the restore controller also checks the
	// PodTemplateSpec of the StatefulSets to make sure the update has been pushed down.
	// Note that this test does **not** verify the StatefulSet check. cass-operator creates
	// the StatefulSets. While we could create the StatefulSets in this test, it will be
	// easier/better to verify the StatefulSet checks in unit and e2e tests.
	t.Log("set datacenter status to updated")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterUpdating,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(restore.Status.DatacenterStopped.Add(time.Second * 1)),
		})
	})
	require.NoError(err, "failed to update datacenter status with updating condition")

	dc := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc)
	require.NoError(err)

	restore = &api.MedusaRestoreJob{}
	err = f.Get(ctx, restoreKey, restore)
	require.NoError(err)

	t.Log("check datacenter restarted")
	require.Eventually(withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		return !dc.Spec.Stopped
	}), timeout, interval)

	t.Log("set datacenter status to ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime.Add(time.Second * 2)),
		})
	})

	require.NoError(err)

	t.Log("check restore status finish time set")
	require.Eventually(func() bool {
		restore := &api.MedusaRestoreJob{}
		err := f.Get(ctx, restoreKey, restore)
		if err != nil {
			return false
		}

		return !restore.Status.FinishTime.IsZero()
	}, timeout, interval)

	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: dc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")

}

func createCassandraPods(t *testing.T, f *framework.Framework, ctx context.Context, dcKey framework.ClusterKey, dc *cassdcapi.CassandraDatacenter) error {
	dcServiceKey := framework.NewClusterKey(dcKey.K8sContext, dcKey.Namespace, dc.GetAllPodsServiceName())
	dcService := &corev1.Service{}
	if err := f.Get(ctx, dcServiceKey, dcService); err != nil {
		if errors.IsNotFound(err) {
			dcService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: dcServiceKey.Namespace,
					Name:      dcServiceKey.Name,
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						cassdcapi.ClusterLabel: cassdcapi.CleanLabelValue(dc.Spec.ClusterName),
					},
					Ports: []corev1.ServicePort{
						{
							Name: "cql",
							Port: 9042,
						},
					},
				},
			}

			err := f.Create(ctx, dcServiceKey, dcService)
			if err != nil {
				return err
			}
		} else {
			t.Errorf("failed to get service %s: %v", dcServiceKey, err)
			return err
		}
	}
	createDatacenterPods(t, f, ctx, dcKey, dc)
	return nil
}

func testValidationErrorStopsRestore(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	require.NoError(f.Client.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(namespace)))
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
							DatacenterName: "real-dc1",
							ServerVersion:  "3.11.14",
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

	kcKey := framework.ClusterKey{K8sContext: k8sCtx0, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &k8ss.K8ssandraCluster{}
		err = f.Client.Get(ctx, kcKey.NamespacedName, kc)

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

	t.Log("creating MedusaBackup")
	backup := &api.MedusaBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      restoredBackupName,
		},
		Spec: api.MedusaBackupSpec{
			CassandraDatacenter: dc1.Name,
		},
	}

	backupKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restoredBackupName)
	err = f.Create(ctx, backupKey, backup)
	require.NoError(err, "failed to create CassandraBackup")

	patch := client.MergeFrom(backup.DeepCopy())
	backup.Status.FinishTime = metav1.Now()
	backup.Status.FinishedNodes = dc1.Spec.Size - 1
	backup.Status.TotalNodes = dc1.Spec.Size
	backup.Status.Nodes = []*api.MedusaBackupNode{
		{
			Datacenter: "dc1",
			Rack:       "default",
		},
		{
			Datacenter: "dc1",
			Rack:       "default",
		},
		{
			Datacenter: "dc1",
			Rack:       "default",
		},
	}

	err = f.PatchStatus(ctx, backup, patch, backupKey)
	require.NoError(err, "failed to patch MedusaBackup")

	restore := &api.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-restore",
		},
		Spec: api.MedusaRestoreJobSpec{
			Backup:              restoredBackupName,
			CassandraDatacenter: dc1.Name,
		},
	}

	restoreKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, restore.ObjectMeta.Name)
	err = f.Create(ctx, restoreKey, restore)
	require.NoError(err, "failed to create MedusaRestoreJob")

	t.Log("check restore status set to failed")
	require.Eventually(func() bool {
		restore := &api.MedusaRestoreJob{}
		err := f.Get(ctx, restoreKey, restore)
		if err != nil {
			return false
		}

		return restore.Status.Message != ""
	}, timeout, interval)

	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for _, container := range containers {
		if container.Name == name {
			return &container
		}
	}
	return nil
}

func TestValidateBackupForRestore(t *testing.T) {
	assert := assert.New(t)

	createBackup := func() *api.MedusaBackup {
		return &api.MedusaBackup{
			Spec: api.MedusaBackupSpec{},
			Status: api.MedusaBackupStatus{
				FinishTime:    metav1.Now(),
				TotalNodes:    3,
				FinishedNodes: 3,
				Nodes: []*api.MedusaBackupNode{
					{
						Datacenter: "dc1",
						Rack:       "r1",
					},
					{
						Datacenter: "dc1",
						Rack:       "r2",
					},
					{
						Datacenter: "dc1",
						Rack:       "r3",
					},
				},
			},
		}
	}

	createCassDc := func() *cassdcapi.CassandraDatacenter {
		return &cassdcapi.CassandraDatacenter{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dc1",
			},
			Spec: cassdcapi.CassandraDatacenterSpec{
				Size: 3,
				Racks: []cassdcapi.Rack{
					{
						Name: "r1",
					},
					{
						Name: "r2",
					},
					{
						Name: "r3",
					},
				},
			},
		}
	}

	assert.NoError(validateBackupForRestore(createBackup(), createCassDc()))

	backup := createBackup()
	backup.Status.FinishedNodes = 2
	assert.Error(validateBackupForRestore(backup, createCassDc()))

	backup = createBackup()
	backup.Status.Nodes[0].Datacenter = "dc2"
	assert.Error(validateBackupForRestore(backup, createCassDc()))

	cassdc := createCassDc()
	cassdc.Spec.Racks = nil
	assert.Error(validateBackupForRestore(createBackup(), cassdc))

	backup = createBackup()
	backup.Status.Nodes[0].Rack = "default"
	backup.Status.Nodes[1].Rack = "default"
	backup.Status.Nodes[2].Rack = "default"
	assert.NoError(validateBackupForRestore(backup, cassdc))

	cassdc = createCassDc()
	cassdc.Spec.Size = 6
	assert.Error(validateBackupForRestore(createBackup(), cassdc))
}

type fakeMedusaRestoreClientFactory struct {
	clientsMutex sync.Mutex
	clients      map[string]*fakeMedusaRestoreClient
}

func NewMedusaClientRestoreFactory() *fakeMedusaRestoreClientFactory {
	return &fakeMedusaRestoreClientFactory{clients: make(map[string]*fakeMedusaRestoreClient, 0)}
}

func (f *fakeMedusaRestoreClientFactory) NewClient(address string) (medusa.Client, error) {
	f.clientsMutex.Lock()
	defer f.clientsMutex.Unlock()
	_, ok := f.clients[address]
	if !ok {
		f.clients[address] = newFakeMedusaRestoreClient()
	}
	return f.clients[address], nil
}

type fakeMedusaRestoreClient struct {
}

func newFakeMedusaRestoreClient() *fakeMedusaRestoreClient {
	return &fakeMedusaRestoreClient{}
}

func (c *fakeMedusaRestoreClient) Close() error {
	return nil
}

func (c *fakeMedusaRestoreClient) CreateBackup(ctx context.Context, name string, backupType string) (*medusa.BackupResponse, error) {
	return nil, nil
}

func (c *fakeMedusaRestoreClient) GetBackups(ctx context.Context) ([]*medusa.BackupSummary, error) {
	return []*medusa.BackupSummary{
		{
			BackupName: restoredBackupName,
			StartTime:  0,
			FinishTime: 10,
			Status:     *medusa.StatusType_SUCCESS.Enum(),
			Nodes: []*medusa.BackupNode{
				{
					Host: "node1", Datacenter: "real-dc1", Rack: "rack1",
				},
				{
					Host: "node2", Datacenter: "real-dc1", Rack: "rack1",
				},
				{
					Host: "node3", Datacenter: "real-dc1", Rack: "rack1",
				},
			},
		},
	}, nil
}

func (c *fakeMedusaRestoreClient) PurgeBackups(ctx context.Context) (*medusa.PurgeBackupsResponse, error) {
	response := &medusa.PurgeBackupsResponse{
		NbBackupsPurged:           2,
		NbObjectsPurged:           10,
		TotalObjectsWithinGcGrace: 0,
		TotalPurgedSize:           1000,
	}
	return response, nil
}

func (c *fakeMedusaRestoreClient) BackupStatus(ctx context.Context, name string) (*medusa.BackupStatusResponse, error) {
	return nil, nil
}

func (c *fakeMedusaRestoreClient) PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*medusa.PrepareRestoreResponse, error) {
	return nil, nil
}
