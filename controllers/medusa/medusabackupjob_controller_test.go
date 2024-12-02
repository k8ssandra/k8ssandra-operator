package medusa

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	medusaImageRepo      = "test/medusa"
	cassandraUserSecret  = "medusa-secret"
	successfulBackupName = "good-backup"
	failingBackupName    = "bad-backup"
	missingBackupName    = "missing-backup"
	backupWithNoPods     = "backup-with-no-pods"
	backupWithNilSummary = "backup-with-nil-summary"
	dc1PodPrefix         = "192.168.1."
	dc2PodPrefix         = "192.168.2."
	fakeBackupFileCount  = int64(13)
	fakeBackupByteSize   = int64(42)
	fakeBackupHumanSize  = "42.00 B"
	fakeMaxBackupCount   = 1
)

var (
	alreadyReportedFailingBackup = false
	alreadyReportedMissingBackup = false
)

func testMedusaBackupDatacenter(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
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

	backupCreated := createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, successfulBackupName)
	require.True(backupCreated, "failed to create backup")

	t.Log("verify that medusa gRPC clients are invoked")
	require.Equal(map[string][]string{
		fmt.Sprintf("%s:%d", getPodIpAddress(0, dc1.DatacenterName()), shared.BackupSidecarPort): {successfulBackupName},
		fmt.Sprintf("%s:%d", getPodIpAddress(1, dc1.DatacenterName()), shared.BackupSidecarPort): {successfulBackupName},
		fmt.Sprintf("%s:%d", getPodIpAddress(2, dc1.DatacenterName()), shared.BackupSidecarPort): {successfulBackupName},
	}, medusaClientFactory.GetRequestedBackups(dc1.DatacenterName()))

	// a failing backup is one that actually starts but fails (on one pod)
	backupCreated = createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, failingBackupName)
	require.False(backupCreated, "the backup object shouldn't have been created")

	// a missing backup is one that never gets to start (on one pod)
	backupCreated = createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, missingBackupName)
	require.False(backupCreated, "the backup object shouldn't have been created")

	// in K8OP-294 we found out we can try to make backups on StatefulSets with no pods
	backupCreated = createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backupWithNoPods)
	require.False(backupCreated, "the backup object shouldn't have been created")
	// in that same effort, we also found out we can have nil instead of backup sumamry
	backupCreated = createAndVerifyMedusaBackup(dc1Key, dc1, f, ctx, require, t, namespace, backupWithNilSummary)
	require.False(backupCreated, "the backup object shouldn't have been created")

	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
}

func createAndVerifyMedusaBackup(dcKey framework.ClusterKey, dc *cassdcapi.CassandraDatacenter, f *framework.Framework, ctx context.Context, require *require.Assertions, t *testing.T, namespace, backupName string) bool {
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
			require.NoError(err)
		} else {
			t.Errorf("failed to get service %s: %v", dcServiceKey, err)
		}
	}

	createDatacenterPods(t, f, ctx, dcKey, dc)

	dcCopy := dc.DeepCopy()
	dcKeyCopy := framework.NewClusterKey(f.DataPlaneContexts[0], dcKey.Namespace+"-copy", dcKey.Name)
	dcCopy.ObjectMeta.Namespace = dc.Namespace + "-copy"

	createDatacenterPods(t, f, ctx, dcKeyCopy, dcCopy)

	// one test scenario needs to have no pods available in the STSs (see #1454)
	// we reproduce that by deleting the pods. we need this for the medusa backup controller tests to work
	// however, we need to bring them back up because medusa task controller tests interact with them later
	// both backup and task controller tests use this function to verify backups
	if backupName == backupWithNoPods {
		deleteDatacenterPods(t, f, ctx, dcKey, dc)
		deleteDatacenterPods(t, f, ctx, dcKeyCopy, dc)
		defer createDatacenterPods(t, f, ctx, dcKey, dc)
		defer createDatacenterPods(t, f, ctx, dcKeyCopy, dcCopy)
	}

	t.Log("creating MedusaBackupJob")
	backupKey := framework.NewClusterKey(dcKey.K8sContext, dcKey.Namespace, backupName)
	backup := &api.MedusaBackupJob{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      backupName,
		},
		Spec: api.MedusaBackupJobSpec{
			CassandraDatacenter: dc.Name,
		},
	}

	err := f.Create(ctx, backupKey, backup)
	require.NoError(err, "failed to create MedusaBackupJob")

	if backupName != backupWithNoPods {
		t.Log("verify that the backups start eventually")
		verifyBackupJobStarted(require.Eventually, t, dc, f, ctx, backupKey)
		verifyBackupJobFinished(t, require, dc, f, ctx, backupKey, backupName)
	} else {
		t.Log("verify that the backups never start")
		verifyBackupJobStarted(require.Never, t, dc, f, ctx, backupKey)
	}

	t.Log("check for the MedusaBackup being created")
	medusaBackupKey := framework.NewClusterKey(dcKey.K8sContext, dcKey.Namespace, backupName)
	medusaBackup := &api.MedusaBackup{}
	err = f.Get(ctx, medusaBackupKey, medusaBackup)
	if err != nil {
		if errors.IsNotFound(err) {
			// exit the test if the backup was not created. this happens for some backup names on purpose
			return false
		}
	}
	t.Log("verify the MedusaBackup is correct")
	require.Equal(medusaBackup.Status.TotalNodes, dc.Spec.Size, "backup total nodes doesn't match dc nodes")
	require.Equal(medusaBackup.Status.FinishedNodes, dc.Spec.Size, "backup finished nodes doesn't match dc nodes")
	require.Equal(len(medusaBackup.Status.Nodes), int(dc.Spec.Size), "backup topology doesn't match dc topology")
	require.Equal(medusaBackup.Status.TotalFiles, fakeBackupFileCount, "backup total files doesn't match")
	require.Equal(medusaBackup.Status.TotalSize, fakeBackupHumanSize, "backup total size doesn't match")
	require.Equal(medusa.StatusType_SUCCESS.String(), medusaBackup.Status.Status, "backup status is not success")

	require.Equal(int(dc.Spec.Size), len(medusaClientFactory.GetRequestedBackups(dc.DatacenterName())))

	return true
}

func verifyBackupJobFinished(t *testing.T, require *require.Assertions, dc *cassdcapi.CassandraDatacenter, f *framework.Framework, ctx context.Context, backupKey framework.ClusterKey, backupName string) {
	t.Log("verify the backup finished")
	require.Eventually(func() bool {
		t.Logf("Requested backups: %v", medusaClientFactory.GetRequestedBackups(dc.DatacenterName()))
		updated := &api.MedusaBackupJob{}
		err := f.Get(ctx, backupKey, updated)
		if err != nil {
			return false
		}
		t.Logf("backup %s finish time: %v", backupName, updated.Status.FinishTime)
		t.Logf("backup %s failed: %v", backupName, updated.Status.Failed)
		t.Logf("backup %s finished: %v", backupName, updated.Status.Finished)
		t.Logf("backup %s in progress: %v", backupName, updated.Status.InProgress)
		return !updated.Status.FinishTime.IsZero()
	}, timeout, interval)
}

func verifyBackupJobStarted(
	verifyFunction func(condition func() bool, waitFor time.Duration, tick time.Duration, msgAndArgs ...interface{}),
	t *testing.T,
	dc *cassdcapi.CassandraDatacenter,
	f *framework.Framework,
	ctx context.Context,
	backupKey framework.ClusterKey,
) {
	verifyFunction(func() bool {
		t.Logf("Requested backups: %v", medusaClientFactory.GetRequestedBackups(dc.DatacenterName()))
		updated := &api.MedusaBackupJob{}
		err := f.Get(ctx, backupKey, updated)
		if err != nil {
			t.Logf("failed to get MedusaBackupJob: %v", err)
			return false
		}
		return !updated.Status.StartTime.IsZero()
	}, timeout, interval)
}

func reconcileReplicatedSecret(ctx context.Context, t *testing.T, f *framework.Framework, kc *k8ss.K8ssandraCluster) {
	t.Log("check ReplicatedSecret reconciled")

	rsec := &replicationapi.ReplicatedSecret{}
	replSecretKey := types.NamespacedName{Name: kc.Name, Namespace: kc.Namespace}

	assert.Eventually(t, func() bool {
		err := f.Client.Get(ctx, replSecretKey, rsec)
		return err == nil
	}, timeout, interval, "failed to get ReplicatedSecret")

	conditions := make([]replicationapi.ReplicationCondition, 0)
	now := metav1.Now()

	for _, target := range rsec.Spec.ReplicationTargets {
		conditions = append(conditions, replicationapi.ReplicationCondition{
			Cluster:            target.K8sContextName,
			Type:               replicationapi.ReplicationDone,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
	}
	rsec.Status.Conditions = conditions
	err := f.Client.Status().Update(ctx, rsec)

	require.NoError(t, err, "Failed to update ReplicationSecret status")
}

// Creates a fake ip address with the pod's original index from the StatefulSet
func getPodIpAddress(index int, dcName string) string {
	switch dcName {
	case "dc1":
		return dc1PodPrefix + strconv.Itoa(50+index)
	case "dc2":
		return dc2PodPrefix + strconv.Itoa(50+index)
	default:
		return "192.168.3." + strconv.Itoa(50+index)
	}
}

type fakeMedusaClientFactory struct {
	clientsMutex sync.Mutex
	clients      map[string]*fakeMedusaClient
}

func NewMedusaClientFactory() *fakeMedusaClientFactory {
	return &fakeMedusaClientFactory{clients: make(map[string]*fakeMedusaClient, 0)}
}

func (f *fakeMedusaClientFactory) NewClient(ctx context.Context, address string) (medusa.Client, error) {
	f.clientsMutex.Lock()
	defer f.clientsMutex.Unlock()
	_, ok := f.clients[address]
	if !ok {
		if strings.HasPrefix(address, dc1PodPrefix) {
			f.clients[address] = newFakeMedusaClient("dc1")
		} else if strings.HasPrefix(address, dc2PodPrefix) {
			f.clients[address] = newFakeMedusaClient("dc2")
		} else {
			f.clients[address] = newFakeMedusaClient("")
		}
	}
	return f.clients[address], nil
}

func (f *fakeMedusaClientFactory) GetRequestedBackups(dc string) map[string][]string {
	f.clientsMutex.Lock()
	defer f.clientsMutex.Unlock()
	requestedBackups := make(map[string][]string)
	for k, v := range f.clients {
		if v.DcName == dc {
			requestedBackups[k] = v.RequestedBackups
		}
	}
	return requestedBackups
}

type fakeMedusaClient struct {
	RequestedBackups []string
	DcName           string
}

func newFakeMedusaClient(dcName string) *fakeMedusaClient {
	// the fake Medusa client keeps a bit of state in order to simulate different backup statuses
	// more precisely, for some backups it will return not a success for some nodes
	// we need to reset this state between tests
	// doing it here is great since we make a new fake client for each test anyway
	alreadyReportedFailingBackup = false
	alreadyReportedMissingBackup = false
	return &fakeMedusaClient{RequestedBackups: make([]string, 0), DcName: dcName}
}

func (c *fakeMedusaClient) Close() error {
	return nil
}

func (c *fakeMedusaClient) CreateBackup(ctx context.Context, name string, backupType string) (*medusa.BackupResponse, error) {
	c.RequestedBackups = append(c.RequestedBackups, name)
	return &medusa.BackupResponse{BackupName: name, Status: medusa.StatusType_IN_PROGRESS}, nil
}

func (c *fakeMedusaClient) GetBackups(ctx context.Context) ([]*medusa.BackupSummary, error) {

	backups := make([]*medusa.BackupSummary, 0)

	for _, name := range c.RequestedBackups {

		// return status based on the backup name
		// since we're implementing altogether different method of the Medusa client, we cannot reuse the BackupStatus logic
		// but we still want to "mock" failing backups
		// this does not get called per node/pod, se we don't need to track counts of what we returned
		var status medusa.StatusType
		if strings.HasPrefix(name, "good") {
			status = *medusa.StatusType_SUCCESS.Enum()
		} else if strings.HasPrefix(name, "bad") {
			status = *medusa.StatusType_FAILED.Enum()
		} else if strings.HasPrefix(name, "missing") {
			status = *medusa.StatusType_UNKNOWN.Enum()
		} else {
			status = *medusa.StatusType_IN_PROGRESS.Enum()
		}

		var backup *medusa.BackupSummary
		if strings.HasPrefix(name, backupWithNilSummary) {
			backup = nil
		} else {
			backup = &medusa.BackupSummary{
				BackupName:    name,
				StartTime:     0,
				FinishTime:    10,
				TotalNodes:    3,
				FinishedNodes: 3,
				TotalObjects:  fakeBackupFileCount,
				TotalSize:     fakeBackupByteSize,
				Status:        status,
				Nodes: []*medusa.BackupNode{
					{
						Host:       "host1",
						Tokens:     []int64{1, 2, 3},
						Datacenter: "dc1",
						Rack:       "rack1",
					},
					{
						Host:       "host2",
						Tokens:     []int64{1, 2, 3},
						Datacenter: "dc1",
						Rack:       "rack1",
					},
					{
						Host:       "host3",
						Tokens:     []int64{1, 2, 3},
						Datacenter: "dc1",
						Rack:       "rack1",
					},
				},
			}
		}
		backups = append(backups, backup)
	}
	return backups, nil
}

func (c *fakeMedusaClient) BackupStatus(ctx context.Context, name string) (*medusa.BackupStatusResponse, error) {
	// return different status for differently named backups
	// but for each not-successful backup, return not-a-success only once
	var status medusa.StatusType
	if strings.HasPrefix(name, successfulBackupName) {
		status = medusa.StatusType_SUCCESS
	} else if strings.HasPrefix(name, failingBackupName) {
		if !alreadyReportedFailingBackup {
			status = medusa.StatusType_FAILED
			alreadyReportedFailingBackup = true
		} else {
			status = medusa.StatusType_SUCCESS
		}
	} else if strings.HasPrefix(name, missingBackupName) {
		if !alreadyReportedMissingBackup {
			alreadyReportedMissingBackup = true
			// reproducing what the gRPC client would send. sadly, it's not a proper NotFound error
			return nil, fmt.Errorf("rpc error: code = NotFound desc = backup <%s> does not exist", name)
		} else {
			status = medusa.StatusType_SUCCESS
		}
	} else {
		status = medusa.StatusType_IN_PROGRESS
	}
	return &medusa.BackupStatusResponse{
		Status: status,
	}, nil
}

func (c *fakeMedusaClient) PurgeBackups(ctx context.Context) (*medusa.PurgeBackupsResponse, error) {
	size := len(c.RequestedBackups)
	if size > fakeMaxBackupCount {
		c.RequestedBackups = c.RequestedBackups[size-fakeMaxBackupCount:]
	}

	response := &medusa.PurgeBackupsResponse{
		NbBackupsPurged:           int32(size - len(c.RequestedBackups)),
		NbObjectsPurged:           10,
		TotalObjectsWithinGcGrace: 0,
		TotalPurgedSize:           1000,
	}
	return response, nil
}

func (c *fakeMedusaClient) PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*medusa.PrepareRestoreResponse, error) {
	return nil, nil
}

func findDatacenterCondition(status *cassdcapi.CassandraDatacenterStatus, condType cassdcapi.DatacenterConditionType) *cassdcapi.DatacenterCondition {
	for _, condition := range status.Conditions {
		if condition.Type == condType {
			return &condition
		}
	}
	return nil
}

func deleteDatacenterPods(t *testing.T, f *framework.Framework, ctx context.Context, dcKey framework.ClusterKey, dc *cassdcapi.CassandraDatacenter) {
	for i := 0; i < int(dc.Spec.Size); i++ {
		pod := &corev1.Pod{}
		podName := fmt.Sprintf("%s-%s-%d", dc.Spec.ClusterName, dc.DatacenterName(), i)
		podKey := framework.NewClusterKey(dcKey.K8sContext, dcKey.Namespace, podName)
		err := f.Delete(ctx, podKey, pod)
		if err != nil {
			t.Logf("failed to delete pod %s: %v", podKey, err)
		}
	}
}

func createDatacenterPods(t *testing.T, f *framework.Framework, ctx context.Context, dcKey framework.ClusterKey, dc *cassdcapi.CassandraDatacenter) {
	_ = f.CreateNamespace(dcKey.Namespace)
	for i := int32(0); i < dc.Spec.Size; i++ {
		pod := &corev1.Pod{}
		podName := fmt.Sprintf("%s-%s-%d", dc.Spec.ClusterName, dc.DatacenterName(), i)
		podKey := framework.NewClusterKey(dcKey.K8sContext, dcKey.Namespace, podName)
		err := f.Get(ctx, podKey, pod)
		if err != nil {
			if errors.IsNotFound(err) {
				t.Logf("pod %s-%s-%d not found", dc.Spec.ClusterName, dc.DatacenterName(), i)
				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: dc.Namespace,
						Name:      podName,
						Labels: map[string]string{
							cassdcapi.ClusterLabel:    cassdcapi.CleanLabelValue(dc.Spec.ClusterName),
							cassdcapi.DatacenterLabel: dc.DatacenterName(),
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "cassandra",
								Image: "cassandra",
							},
							{
								Name:  shared.BackupSidecarName,
								Image: shared.BackupSidecarName,
							},
						},
					},
				}
				err = f.Create(ctx, podKey, pod)
				require.NoError(t, err, "failed to create datacenter pod")

				patch := client.MergeFrom(pod.DeepCopy())
				pod.Status.PodIP = getPodIpAddress(int(i), dc.DatacenterName())

				err = f.PatchStatus(ctx, pod, patch, podKey)
				require.NoError(t, err, "failed to patch datacenter pod status")
			}
		}
	}
}

func verifyObjectDoesNotExist(ctx context.Context, t *testing.T, f *framework.Framework, key framework.ClusterKey, obj client.Object) {
	assert.Eventually(t, func() bool {
		err := f.Get(ctx, key, obj)
		return err != nil && errors.IsNotFound(err)
	}, timeout, interval, "failed to verify object does not exist", key)
}

func TestHumanize(t *testing.T) {
	t.Run("humanizeTrivialSizes", humanizeTrivialSizes)
	t.Run("humanizeArbitrarySizes", humanizeArbitrarySizes)
}

func humanizeTrivialSizes(t *testing.T) {
	assert.Equal(t, "1.00 B", humanize(1))
	assert.Equal(t, "1.00 KB", humanize(1024))
	assert.Equal(t, "1.00 MB", humanize(1024*1024))
	assert.Equal(t, "1.00 GB", humanize(1024*1024*1024))
}

func humanizeArbitrarySizes(t *testing.T) {
	assert.Equal(t, "127.50 KB", humanize(130557))
	assert.Equal(t, "4.03 GB", humanize(4325130557))
	assert.Equal(t, "7.67 TB", humanize(8434729356343))
	assert.Equal(t, "1096.52 PB", humanize(1234567890123456790))
}
