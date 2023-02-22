package control

import (
	"context"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	casscontrol "github.com/k8ssandra/cass-operator/controllers/control"
	api "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	k8capi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	timeout  = time.Second * 5
	interval = time.Millisecond * 500
)

var (
	defaultStorageClass  = "default"
	testEnv              *testutils.MultiClusterTestEnv
	managementApiFactory = &testutils.FakeManagementApiFactory{}
)

func TestK8ssandraTask(t *testing.T) {
	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)
	testEnv = &testutils.MultiClusterTestEnv{
		NumDataPlanes: 2,
		BeforeTest: func(t *testing.T) {
			managementApiFactory.SetT(t)
			managementApiFactory.UseDefaultAdapter()
		},
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&K8ssandraTaskReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			Recorder:         mgr.GetEventRecorderFor("k8ssandratask-controller"),
		}).SetupWithManager(mgr, clusters)
		return err
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	defer testEnv.Stop(t)
	defer cancel()

	t.Run("ExecuteParallelK8ssandraTask", testEnv.ControllerTest(ctx, executeParallelK8ssandraTask))
	t.Run("ExecuteSequentialK8ssandraTask", testEnv.ControllerTest(ctx, executeSequentialK8ssandraTask))
	t.Run("DeleteK8ssandraTask", testEnv.ControllerTest(ctx, deleteK8ssandraTask))
	t.Run("ExpireK8ssandraTask", testEnv.ControllerTest(ctx, expireK8ssandraTask))
}

// executeParallelK8ssandraTask creates and runs a K8ssandraTask with parallel DC processing.
func executeParallelK8ssandraTask(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := newCluster(namespace, "kc",
		newDc("dc1", f.DataPlaneContexts[0]),
		newDc("dc2", f.DataPlaneContexts[1]))
	require.NoError(f.Client.Create(ctx, kc), "failed to create K8ssandraCluster")

	t.Log("Create a K8ssandraTask")
	k8Task := &api.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "upgradesstables",
		},
		Spec: api.K8ssandraTaskSpec{
			Cluster: corev1.ObjectReference{
				Name: "kc",
			},
			Template: cassapi.CassandraTaskTemplate{
				Jobs: []cassapi.CassandraJob{{
					Name:    "job1",
					Command: "upgradesstables",
				}},
			},
			DcConcurrencyPolicy: batchv1.AllowConcurrent,
		},
	}
	require.NoError(f.Client.Create(ctx, k8Task), "failed to create K8ssandraTask")

	t.Log("Check that all the CassandraTasks have been created")
	cassTask1 := loadCassandraTask(f.DataPlaneContexts[0], namespace, "upgradesstables-dc1", ctx, f, require)
	require.Equal("job1", cassTask1.Spec.Jobs[0].Name)
	require.Equal("upgradesstables", string(cassTask1.Spec.Jobs[0].Command))

	cassTask2 := loadCassandraTask(f.DataPlaneContexts[1], namespace, "upgradesstables-dc2", ctx, f, require)
	require.Equal("job1", cassTask2.Spec.Jobs[0].Name)
	require.Equal("upgradesstables", string(cassTask2.Spec.Jobs[0].Command))

	t.Log("Mark the CassandraTasks as Running")
	startTime1 := metav1.Now().Rfc3339Copy()
	startTime2 := metav1.NewTime(startTime1.Add(time.Second))
	cassTask1Key := newClusterKey(f.DataPlaneContexts[0], namespace, "upgradesstables-dc1")
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask1Key, func(cassTask1 *cassapi.CassandraTask) {
		cassTask1.Status.Active = 1
		cassTask1.Status.StartTime = &startTime1
		casscontrol.SetCondition(cassTask1, cassapi.JobRunning, corev1.ConditionTrue)
	}))

	cassTask2Key := newClusterKey(f.DataPlaneContexts[1], namespace, "upgradesstables-dc2")
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask2Key, func(cassTask2 *cassapi.CassandraTask) {
		cassTask2.Status.Active = 1
		cassTask2.Status.StartTime = &startTime2
		casscontrol.SetCondition(cassTask2, cassapi.JobRunning, corev1.ConditionTrue)
	}))

	t.Log("Check that the K8ssandraTask is marked as Running")
	require.Eventually(func() bool {
		k8Task = &api.K8ssandraTask{}
		require.NoError(f.Get(ctx, newClusterKey(f.ControlPlaneContext, namespace, "upgradesstables"), k8Task))
		return k8Task.Status.Active == 2 &&
			k8Task.Status.StartTime.Equal(&startTime1) &&
			k8Task.GetConditionStatus(cassapi.JobRunning) == corev1.ConditionTrue
	}, timeout, interval)

	t.Log("Mark the CassandraTasks as Complete")
	completionTime1 := metav1.NewTime(startTime1.Add(10 * time.Second))
	completionTime2 := metav1.NewTime(completionTime1.Add(time.Second))
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask1Key, func(cassTask1 *cassapi.CassandraTask) {
		cassTask1.Status.Active = 0
		cassTask1.Status.Succeeded = 1
		cassTask1.Status.CompletionTime = &completionTime1
		casscontrol.SetCondition(cassTask1, cassapi.JobRunning, corev1.ConditionFalse)
		casscontrol.SetCondition(cassTask1, cassapi.JobComplete, corev1.ConditionTrue)
	}))
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask2Key, func(cassTask2 *cassapi.CassandraTask) {
		cassTask2.Status.Active = 0
		cassTask2.Status.Succeeded = 1
		cassTask2.Status.CompletionTime = &completionTime2
		casscontrol.SetCondition(cassTask2, cassapi.JobRunning, corev1.ConditionFalse)
		casscontrol.SetCondition(cassTask2, cassapi.JobComplete, corev1.ConditionTrue)
	}))

	t.Log("Check that the K8ssandraTask is marked as Complete")
	require.Eventually(func() bool {
		k8Task = &api.K8ssandraTask{}
		require.NoError(f.Get(ctx, newClusterKey(f.ControlPlaneContext, namespace, "upgradesstables"), k8Task))
		return k8Task.Status.Active == 0 &&
			k8Task.Status.Succeeded == 2 &&
			k8Task.Status.CompletionTime.Equal(&completionTime2) &&
			k8Task.GetConditionStatus(cassapi.JobRunning) == corev1.ConditionFalse &&
			k8Task.GetConditionStatus(cassapi.JobComplete) == corev1.ConditionTrue
	}, timeout, interval)
}

// executeSequentialK8ssandraTask creates and runs a K8ssandraTask with sequential DC processing.
func executeSequentialK8ssandraTask(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := newCluster(namespace, "kc",
		newDc("dc1", f.DataPlaneContexts[0]),
		newDc("dc2", f.DataPlaneContexts[1]))
	require.NoError(f.Client.Create(ctx, kc), "failed to create K8ssandraCluster")

	t.Log("Create a K8ssandraTask")
	k8Task := &api.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "upgradesstables",
		},
		Spec: api.K8ssandraTaskSpec{
			Cluster: corev1.ObjectReference{
				Name: "kc",
			},
			Template: cassapi.CassandraTaskTemplate{
				Jobs: []cassapi.CassandraJob{{
					Name:    "job1",
					Command: "upgradesstables",
				}},
			},
			// Note: DcConcurrencyPolicy defaults to batchv1.ForbidConcurrent
		},
	}
	require.NoError(f.Client.Create(ctx, k8Task), "failed to create K8ssandraTask")

	cassTask1Key := newClusterKey(f.DataPlaneContexts[0], namespace, "upgradesstables-dc1")
	cassTask2Key := newClusterKey(f.DataPlaneContexts[1], namespace, "upgradesstables-dc2")

	t.Log("Check that CassandraTask 1 has been created")
	cassTask1 := loadCassandraTask(f.DataPlaneContexts[0], namespace, "upgradesstables-dc1", ctx, f, require)
	require.Equal("job1", cassTask1.Spec.Jobs[0].Name)
	require.Equal("upgradesstables", string(cassTask1.Spec.Jobs[0].Command))

	t.Log("Check that CassandraTask 2 hasn't been created yet")
	require.False(f.CassTaskExists(ctx, cassTask2Key)())

	t.Log("Mark CassandraTask 1 as Running")
	startTime1 := metav1.Now().Rfc3339Copy()
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask1Key, func(cassTask1 *cassapi.CassandraTask) {
		cassTask1.Status.Active = 1
		cassTask1.Status.StartTime = &startTime1
		casscontrol.SetCondition(cassTask1, cassapi.JobRunning, corev1.ConditionTrue)
	}))

	t.Log("Check that the K8ssandraTask is marked as Running")
	require.Eventually(func() bool {
		k8Task = &api.K8ssandraTask{}
		require.NoError(f.Get(ctx, newClusterKey(f.ControlPlaneContext, namespace, "upgradesstables"), k8Task))
		return k8Task.Status.Active == 1 &&
			k8Task.Status.StartTime.Equal(&startTime1) &&
			k8Task.GetConditionStatus(cassapi.JobRunning) == corev1.ConditionTrue
	}, timeout, interval)

	t.Log("Check that CassandraTask 2 still hasn't been created")
	require.False(f.CassTaskExists(ctx, cassTask2Key)())

	t.Log("Mark CassandraTask 1 as Complete")
	completionTime1 := metav1.NewTime(startTime1.Add(10 * time.Second))
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask1Key, func(cassTask1 *cassapi.CassandraTask) {
		cassTask1.Status.Active = 0
		cassTask1.Status.Succeeded = 1
		cassTask1.Status.CompletionTime = &completionTime1
		casscontrol.SetCondition(cassTask1, cassapi.JobRunning, corev1.ConditionFalse)
		casscontrol.SetCondition(cassTask1, cassapi.JobComplete, corev1.ConditionTrue)
	}))

	t.Log("Check that CassandraTask 2 has been created")
	cassTask2 := loadCassandraTask(f.DataPlaneContexts[1], namespace, "upgradesstables-dc2", ctx, f, require)
	require.Equal("job1", cassTask2.Spec.Jobs[0].Name)
	require.Equal("upgradesstables", string(cassTask2.Spec.Jobs[0].Command))

	t.Log("Mark CassandraTask 2 as Running")
	startTime2 := metav1.NewTime(startTime1.Add(11 * time.Second))
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask2Key, func(cassTask2 *cassapi.CassandraTask) {
		cassTask2.Status.Active = 1
		cassTask2.Status.StartTime = &startTime2
		casscontrol.SetCondition(cassTask2, cassapi.JobRunning, corev1.ConditionTrue)
	}))

	t.Log("Mark CassandraTask 2 as Complete")
	completionTime2 := metav1.NewTime(startTime1.Add(20 * time.Second))
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask2Key, func(cassTask2 *cassapi.CassandraTask) {
		cassTask2.Status.Active = 0
		cassTask2.Status.Succeeded = 1
		cassTask2.Status.CompletionTime = &completionTime2
		casscontrol.SetCondition(cassTask2, cassapi.JobRunning, corev1.ConditionFalse)
		casscontrol.SetCondition(cassTask2, cassapi.JobComplete, corev1.ConditionTrue)
	}))

	t.Log("Check that the K8ssandraTask is marked as Complete")
	require.Eventually(func() bool {
		k8Task = &api.K8ssandraTask{}
		require.NoError(f.Get(ctx, newClusterKey(f.ControlPlaneContext, namespace, "upgradesstables"), k8Task))
		for _, cond := range k8Task.Status.Conditions {
			t.Log("Condition", "type", cond.Type, "status", cond.Status)
		}
		for dcName, dc := range k8Task.Status.Datacenters {
			t.Log("Datacenter", "dc name", dcName, "active", dc.Active, "Failed", dc.Failed, "Succeeded", dc.Succeeded, "CompletionTime", dc.CompletionTime)
		}
		t.Log("Checking completion", "k8Task", k8Task, "completionTime1", completionTime1, "completionTime2", completionTime2, "k8Task.Status.CompletionTime", k8Task.Status.CompletionTime, "JobRunning", k8Task.GetConditionStatus(cassapi.JobRunning) == corev1.ConditionFalse, "JobComplete", k8Task.GetConditionStatus(cassapi.JobComplete) == corev1.ConditionTrue)
		return k8Task.Status.Active == 0 &&
			k8Task.Status.Succeeded == 2 &&
			k8Task.Status.CompletionTime.Equal(&completionTime2) &&
			k8Task.GetConditionStatus(cassapi.JobRunning) == corev1.ConditionFalse &&
			k8Task.GetConditionStatus(cassapi.JobComplete) == corev1.ConditionTrue
	}, timeout, interval)
}

// deleteK8ssandraTask verifies that when the K8ssandraTask gets deleted, its dependent CassandraTasks get deleted.
func deleteK8ssandraTask(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := newCluster(namespace, "kc",
		newDc("dc1", f.DataPlaneContexts[0]),
		newDc("dc2", f.DataPlaneContexts[1]))
	require.NoError(f.Client.Create(ctx, kc), "failed to create K8ssandraCluster")

	t.Log("Create a K8ssandraTask")
	k8Task := &api.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "upgradesstables",
		},
		Spec: api.K8ssandraTaskSpec{
			Cluster: corev1.ObjectReference{
				Name: "kc",
			},
			Template: cassapi.CassandraTaskTemplate{
				Jobs: []cassapi.CassandraJob{{
					Name:    "job1",
					Command: "upgradesstables",
				}},
			},
			DcConcurrencyPolicy: batchv1.AllowConcurrent,
		},
	}
	require.NoError(f.Client.Create(ctx, k8Task), "failed to create K8ssandraTask")

	t.Log("Check that the CassandraTasks exist")
	cassTask1Key := newClusterKey(f.DataPlaneContexts[0], namespace, "upgradesstables-dc1")
	require.Eventually(f.CassTaskExists(ctx, cassTask1Key), timeout, interval)
	cassTask2Key := newClusterKey(f.DataPlaneContexts[1], namespace, "upgradesstables-dc2")
	require.Eventually(f.CassTaskExists(ctx, cassTask2Key), timeout, interval)

	t.Log("Delete the K8ssandraTask")
	require.NoError(f.Client.Delete(ctx, k8Task), "failed to delete K8ssandraTask")

	t.Log("Check that the CassandraTasks do not exist anymore")
	require.Eventually(func() bool { return !f.CassTaskExists(ctx, cassTask1Key)() }, timeout, interval)
	require.Eventually(func() bool { return !f.CassTaskExists(ctx, cassTask2Key)() }, timeout, interval)
}

// expireK8ssandraTask verifies that a completed K8ssandraTask gets deleted (as well as the corresponding
// CassandraTasks) after its TTL expires.
func expireK8ssandraTask(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := newCluster(namespace, "kc",
		newDc("dc1", f.DataPlaneContexts[0]),
		newDc("dc2", f.DataPlaneContexts[1]))
	require.NoError(f.Client.Create(ctx, kc), "failed to create K8ssandraCluster")

	t.Log("Create a K8ssandraTask with TTL")
	ttl := new(int32)
	*ttl = 1
	k8Task := &api.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "upgradesstables",
		},
		Spec: api.K8ssandraTaskSpec{
			Cluster: corev1.ObjectReference{
				Name: "kc",
			},
			Template: cassapi.CassandraTaskTemplate{
				TTLSecondsAfterFinished: ttl,
				Jobs: []cassapi.CassandraJob{{
					Name:    "job1",
					Command: "upgradesstables",
				}},
			},
			DcConcurrencyPolicy: batchv1.AllowConcurrent,
		},
	}
	require.NoError(f.Client.Create(ctx, k8Task), "failed to create K8ssandraTask")

	t.Log("Mark the CassandraTasks as Complete")
	cassTask1Key := newClusterKey(f.DataPlaneContexts[0], namespace, "upgradesstables-dc1")
	require.Eventually(f.CassTaskExists(ctx, cassTask1Key), timeout, interval)
	cassTask2Key := newClusterKey(f.DataPlaneContexts[1], namespace, "upgradesstables-dc2")
	require.Eventually(f.CassTaskExists(ctx, cassTask2Key), timeout, interval)

	completionTime1 := metav1.Now().Rfc3339Copy()
	completionTime2 := metav1.NewTime(completionTime1.Add(time.Second))
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask1Key, func(cassTask1 *cassapi.CassandraTask) {
		cassTask1.Status.Succeeded = 1
		cassTask1.Status.CompletionTime = &completionTime1
		casscontrol.SetCondition(cassTask1, cassapi.JobComplete, corev1.ConditionTrue)
	}))
	require.NoError(f.PatchCassandraTaskStatus(ctx, cassTask2Key, func(cassTask2 *cassapi.CassandraTask) {
		cassTask2.Status.Succeeded = 1
		cassTask2.Status.CompletionTime = &completionTime2
		casscontrol.SetCondition(cassTask2, cassapi.JobComplete, corev1.ConditionTrue)
	}))

	t.Log("Check that everything gets deleted")
	require.Eventually(func() bool { return !f.CassTaskExists(ctx, cassTask1Key)() }, timeout, interval)
	require.Eventually(func() bool { return !f.CassTaskExists(ctx, cassTask2Key)() }, timeout, interval)
	k8TaskKey := newClusterKey(f.ControlPlaneContext, namespace, "upgradesstables")
	require.Eventually(func() bool { return !f.K8ssandraTaskExists(ctx, k8TaskKey)() }, timeout, interval)
}

func newCluster(namespace, name string, dcs ...k8capi.CassandraDatacenterTemplate) *k8capi.K8ssandraCluster {
	return &k8capi.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: k8capi.K8ssandraClusterSpec{
			Cassandra: &k8capi.CassandraClusterTemplate{
				Datacenters: dcs,
			},
		},
	}
}

func newDc(name string, k8sContext string) k8capi.CassandraDatacenterTemplate {
	return k8capi.CassandraDatacenterTemplate{
		Meta: k8capi.EmbeddedObjectMeta{
			Name: name,
		},
		K8sContext: k8sContext,
		Size:       1,
		DatacenterOptions: k8capi.DatacenterOptions{
			ServerVersion: "3.11.14",
			StorageConfig: &cassdcapi.StorageConfig{
				CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &defaultStorageClass,
				},
			},
		},
	}
}

func newClusterKey(k8sContext, namespace, name string) framework.ClusterKey {
	return framework.ClusterKey{
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: name},
		K8sContext:     k8sContext,
	}
}

func loadCassandraTask(k8sContext, namespace, cassTaskName string, ctx context.Context, f *framework.Framework, require *require.Assertions) *cassapi.CassandraTask {
	cassTaskKey := newClusterKey(k8sContext, namespace, cassTaskName)
	require.Eventually(f.CassTaskExists(ctx, cassTaskKey), timeout, interval)
	cassTask := &cassapi.CassandraTask{}
	require.NoError(f.Get(ctx, cassTaskKey, cassTask), "failed to get CassandraTask")
	return cassTask
}
