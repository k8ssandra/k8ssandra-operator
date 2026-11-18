package medusa

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ss "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
				DatacenterOptions: k8ss.DatacenterOptions{
					ServerVersion: "3.11.14",
				},
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
			condition := findDatacenterScalingUpCondition(k8ssandraStatus.Cassandra)
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

type syncTestClient struct {
	medusa.Client
	backups []*medusa.BackupSummary
	err     error
	calls   int
}

func (c *syncTestClient) GetBackups(context.Context) ([]*medusa.BackupSummary, error) {
	c.calls++
	return c.backups, c.err
}
func (c *syncTestClient) Close() error { return nil }

type syncTestFactory struct {
	medusa.ClientFactory
	remote *syncTestClient
}

func (f syncTestFactory) NewClient(string) (medusa.Client, error) { return f.remote, nil }

type syncCreateFailureClient struct {
	client.Client
	fail bool
}

func (c *syncCreateFailureClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if backup, ok := obj.(*api.MedusaBackup); ok && backup.Name == "backup2" && c.fail {
		return fmt.Errorf("backup creation unavailable")
	}
	return c.Client.Create(ctx, obj, opts...)
}

func TestMedusaSyncReconcile(t *testing.T) {
	for _, failure := range []string{"none", "list", "create"} {
		t.Run(failure, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, cassdcapi.AddToScheme(scheme))
			require.NoError(t, api.AddToScheme(scheme))
			dc := &cassdcapi.CassandraDatacenter{ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "test", UID: "dc1"}}
			task := &api.MedusaTask{ObjectMeta: metav1.ObjectMeta{Name: "sync", Namespace: dc.Namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: cassdcapi.GroupVersion.String(), Kind: "CassandraDatacenter", Name: dc.Name, UID: dc.UID}}}, Spec: api.MedusaTaskSpec{Operation: api.OperationTypeSync, CassandraDatacenter: dc.Name}}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: dc.Namespace, Labels: dc.GetDatacenterLabels()}}
			stale := &api.MedusaBackup{ObjectMeta: metav1.ObjectMeta{Name: "stale", Namespace: dc.Namespace}, Spec: api.MedusaBackupSpec{CassandraDatacenter: dc.Name}}
			other := &api.MedusaBackup{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: dc.Namespace}, Spec: api.MedusaBackupSpec{CassandraDatacenter: "dc2"}}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dc, task, pod, stale, other).WithStatusSubresource(&api.MedusaTask{}, &api.MedusaBackup{}).Build()
			remote := &syncTestClient{backups: []*medusa.BackupSummary{
				{BackupName: "backup1", Status: medusa.StatusType_SUCCESS, StartTime: 100, FinishTime: 200},
				{BackupName: "backup2", Status: medusa.StatusType_SUCCESS, StartTime: 100, FinishTime: 200},
			}}
			r := &MedusaTaskReconciler{Client: c, Scheme: scheme, ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second}, ClientFactory: syncTestFactory{remote: remote}}
			req := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(task)}
			if failure == "create" {
				failingClient := &syncCreateFailureClient{Client: c, fail: true}
				r.Client = failingClient
				_, err := r.Reconcile(ctx, req)
				require.EqualError(t, err, "backup creation unavailable")
				require.NoError(t, c.Get(ctx, req.NamespacedName, task))
				require.False(t, task.Status.StartTime.IsZero())
				require.True(t, task.Status.FinishTime.IsZero())
				require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: dc.Namespace, Name: "backup1"}, &api.MedusaBackup{}))
				failingClient.fail = false
			}
			if failure == "list" {
				remote.err = fmt.Errorf("remote unavailable")
				result, err := r.Reconcile(ctx, req)
				require.NoError(t, err)
				require.NotZero(t, result.RequeueAfter)
				require.NoError(t, c.Get(ctx, req.NamespacedName, task))
				require.True(t, task.Status.FinishTime.IsZero())
				remote.err = nil
			}
			_, err := r.Reconcile(ctx, req)
			require.NoError(t, err)
			backups := &api.MedusaBackupList{}
			require.NoError(t, c.List(ctx, backups, client.InNamespace(dc.Namespace)))
			names := []string{}
			for _, backup := range backups.Items {
				names = append(names, backup.Name)
			}
			require.ElementsMatch(t, []string{"backup1", "backup2", "other"}, names)
			require.NoError(t, c.Get(ctx, req.NamespacedName, task))
			require.False(t, task.Status.FinishTime.IsZero())
			require.Len(t, task.Status.Finished, 1)
			calls := remote.calls
			_, err = r.Reconcile(ctx, req)
			require.NoError(t, err)
			require.Equal(t, calls, remote.calls)
		})
	}
}

// podOpTestFactory hands out clients whose PurgeBackups and PrepareRestore check the batches the pods run in. Each
// call blocks until every pod of its expected batch has started, so a batch only finishes if its pods run at the same
// time, and it records a violation if a pod of another batch is running at the same time.
type podOpTestFactory struct {
	medusa.ClientFactory
	mu         sync.Mutex
	batchOf    map[string]int
	batchSize  map[int]int
	started    map[int]int
	batchReady map[int]chan struct{}
	active     map[string]bool
	violations []string
	order      []string
	podOf      map[string]string
	failing    string
	hanging    string
	release    chan struct{}
}

func newPodOpTestFactory(batches [][]string, failing, hanging string) *podOpTestFactory {
	f := &podOpTestFactory{
		batchOf:    map[string]int{},
		batchSize:  map[int]int{},
		started:    map[int]int{},
		batchReady: map[int]chan struct{}{},
		active:     map[string]bool{},
		podOf:      map[string]string{},
		failing:    failing,
		hanging:    hanging,
		release:    make(chan struct{}),
	}
	for i, batch := range batches {
		f.batchSize[i] = len(batch)
		f.batchReady[i] = make(chan struct{})
		for _, pod := range batch {
			f.batchOf[pod] = i
		}
	}
	return f
}

func (f *podOpTestFactory) NewClient(address string) (medusa.Client, error) {
	return &podOpTestClient{factory: f, address: address}, nil
}

type podOpTestClient struct {
	medusa.Client
	factory *podOpTestFactory
	address string
}

func (c *podOpTestClient) run(ctx context.Context) error {
	f := c.factory
	f.mu.Lock()
	pod := f.podOf[c.address]
	batch := f.batchOf[pod]
	for other := range f.active {
		if f.batchOf[other] != batch {
			f.violations = append(f.violations, fmt.Sprintf("%s ran at the same time as %s", pod, other))
		}
	}
	f.active[pod] = true
	f.order = append(f.order, pod)
	f.started[batch]++
	if f.started[batch] == f.batchSize[batch] {
		close(f.batchReady[batch])
	}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.active, pod)
		f.mu.Unlock()
	}()

	select {
	case <-f.batchReady[batch]:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for the rest of the batch to start")
	}
	if pod == f.hanging {
		select {
		case <-f.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if pod == f.failing {
		return fmt.Errorf("pod operation failed")
	}
	return nil
}

func (c *podOpTestClient) PurgeBackups(ctx context.Context) (*medusa.PurgeBackupsResponse, error) {
	if err := c.run(ctx); err != nil {
		return nil, err
	}
	return &medusa.PurgeBackupsResponse{NbBackupsPurged: 1}, nil
}

func (c *podOpTestClient) PrepareRestore(ctx context.Context, _, _, _ string) (*medusa.PrepareRestoreResponse, error) {
	if err := c.run(ctx); err != nil {
		return nil, err
	}
	return &medusa.PrepareRestoreResponse{}, nil
}

func (c *podOpTestClient) Close() error { return nil }

type podOpTest struct {
	client  client.Client
	r       *MedusaTaskReconciler
	req     reconcile.Request
	factory *podOpTestFactory
}

// newPodOpTest creates a datacenter with racks r1, r2 and r3 and pods pod1 to pod7. pod1 to pod6 are spread over the
// racks in turn, and pod7 is in rack r4, which isn't in the datacenter spec.
func newPodOpTest(t *testing.T, operation api.OperationType, annotations map[string]string, batches [][]string, failing, hanging string) *podOpTest {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, cassdcapi.AddToScheme(scheme))
	require.NoError(t, api.AddToScheme(scheme))
	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "test", UID: "dc1"},
		Spec:       cassdcapi.CassandraDatacenterSpec{Racks: []cassdcapi.Rack{{Name: "r1"}, {Name: "r2"}, {Name: "r3"}}},
	}
	task := &api.MedusaTask{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "task",
			Namespace:       dc.Namespace,
			Annotations:     annotations,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: cassdcapi.GroupVersion.String(), Kind: "CassandraDatacenter", Name: dc.Name, UID: dc.UID}},
		},
		Spec: api.MedusaTaskSpec{Operation: operation, CassandraDatacenter: dc.Name, BackupName: "backup", RestoreKey: "key"},
	}
	factory := newPodOpTestFactory(batches, failing, hanging)
	objects := []client.Object{dc, task}
	for i := 1; i <= 7; i++ {
		rack := fmt.Sprintf("r%d", (i-1)%3+1)
		if i == 7 {
			rack = "r4"
		}
		name := fmt.Sprintf("pod%d", i)
		podLabels := dc.GetDatacenterLabels()
		podLabels[cassdcapi.RackLabel] = rack
		objects = append(objects, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: dc.Namespace, Labels: podLabels},
			Status:     corev1.PodStatus{PodIP: fmt.Sprintf("10.0.0.%d", i)},
		})
		factory.podOf[fmt.Sprintf("10.0.0.%d:%d", i, medusa.DefaultMedusaPort)] = name
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithStatusSubresource(&api.MedusaTask{}).Build()
	return &podOpTest{
		client:  c,
		r:       &MedusaTaskReconciler{Client: c, Scheme: scheme, ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second}, ClientFactory: factory},
		req:     reconcile.Request{NamespacedName: client.ObjectKeyFromObject(task)},
		factory: factory,
	}
}

func (p *podOpTest) status(t *testing.T) api.MedusaTaskStatus {
	task := &api.MedusaTask{}
	require.NoError(t, p.client.Get(context.Background(), p.req.NamespacedName, task))
	return task.Status
}

func finishedPods(status api.MedusaTaskStatus) []string {
	pods := []string{}
	for _, result := range status.Finished {
		pods = append(pods, result.PodName)
	}
	return pods
}

func TestMedusaTaskConcurrency(t *testing.T) {
	allPods := []string{"pod1", "pod2", "pod3", "pod4", "pod5", "pod6", "pod7"}
	byRack := [][]string{{"pod1", "pod4"}, {"pod2", "pod5"}, {"pod3", "pod6"}, {"pod7"}}
	byPod := [][]string{{"pod1"}, {"pod2"}, {"pod3"}, {"pod4"}, {"pod5"}, {"pod6"}, {"pod7"}}
	tests := []struct {
		name       string
		operation  api.OperationType
		annotation string
		batches    [][]string
	}{
		{name: "purge on all pods by default", operation: api.OperationTypePurge, batches: [][]string{allPods}},
		{name: "purge on all pods", operation: api.OperationTypePurge, annotation: "all", batches: [][]string{allPods}},
		{name: "purge one rack at a time", operation: api.OperationTypePurge, annotation: "rack", batches: byRack},
		{name: "purge one pod at a time", operation: api.OperationTypePurge, annotation: "pod", batches: byPod},
		{name: "purge one pod at a time for an unknown value", operation: api.OperationTypePurge, annotation: "racks", batches: byPod},
		{name: "prepare restore on all pods by default", operation: api.OperationTypePrepareRestore, batches: [][]string{allPods}},
		{name: "prepare restore one rack at a time", operation: api.OperationTypePrepareRestore, annotation: "rack", batches: byRack},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var annotations map[string]string
			if tt.annotation != "" {
				annotations = map[string]string{medusa.MedusaTaskConcurrencyAnnotation: tt.annotation}
			}
			p := newPodOpTest(t, tt.operation, annotations, tt.batches, "pod2", "")

			// The pod operations outlive Reconcile, so cancelling its context must not cancel them.
			reconcileCtx, cancel := context.WithCancel(t.Context())
			_, err := p.r.Reconcile(reconcileCtx, p.req)
			cancel()
			require.NoError(t, err)

			var status api.MedusaTaskStatus
			require.Eventually(t, func() bool {
				status = p.status(t)
				return len(status.InProgress) == 0 && len(status.Finished)+len(status.Failed) == len(allPods)
			}, 30*time.Second, 50*time.Millisecond)

			require.ElementsMatch(t, []string{"pod1", "pod3", "pod4", "pod5", "pod6", "pod7"}, finishedPods(status))
			require.Equal(t, []string{"pod2"}, status.Failed)
			if tt.operation == api.OperationTypePurge {
				for _, result := range status.Finished {
					require.Equal(t, 1, result.NbBackupsPurged)
				}
			}

			p.factory.mu.Lock()
			defer p.factory.mu.Unlock()
			require.Empty(t, p.factory.violations)
			batch := 0
			for _, pod := range p.factory.order {
				require.GreaterOrEqual(t, p.factory.batchOf[pod], batch, "pods ran out of batch order: %v", p.factory.order)
				batch = p.factory.batchOf[pod]
			}
		})
	}
}

// A pod that hangs must not hold back the status of the pods that already finished.
func TestMedusaTaskStatusWithHangingPod(t *testing.T) {
	allPods := []string{"pod1", "pod2", "pod3", "pod4", "pod5", "pod6", "pod7"}
	p := newPodOpTest(t, api.OperationTypePurge, nil, [][]string{allPods}, "pod2", "pod3")
	defer close(p.factory.release)

	_, err := p.r.Reconcile(t.Context(), p.req)
	require.NoError(t, err)

	var status api.MedusaTaskStatus
	require.Eventually(t, func() bool {
		status = p.status(t)
		return len(status.Finished)+len(status.Failed) == len(allPods)-1
	}, 15*time.Second, 50*time.Millisecond)
	require.Equal(t, []string{"pod3"}, status.InProgress)
	require.ElementsMatch(t, []string{"pod1", "pod4", "pod5", "pod6", "pod7"}, finishedPods(status))
	require.Equal(t, []string{"pod2"}, status.Failed)
}
