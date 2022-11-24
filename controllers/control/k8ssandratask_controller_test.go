package control

import (
	"context"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	k8capi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"testing"
	"time"
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
		NumDataPlanes: 3,
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
		}).SetupWithManager(mgr, clusters)
		return err
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	defer testEnv.Stop(t)
	defer cancel()

	t.Run("CreateK8ssandraTask", testEnv.ControllerTest(ctx, createK8ssandraTask))
}

// createK8ssandraTask verifies that CassandraTasks are created for each datacenter.
func createK8ssandraTask(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := newCluster(namespace, "kc",
		newDc("dc1", f.DataPlaneContexts[0]),
		newDc("dc2", f.DataPlaneContexts[1]))
	require.NoError(f.Client.Create(ctx, kc), "failed to create K8ssandraCluster")

	k8Task := &api.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "upgradesstables",
		},
		Spec: api.K8ssandraTaskSpec{
			Cluster: corev1.ObjectReference{
				Name: "kc",
			},
			Template: cassapi.CassandraTaskSpec{
				Jobs: []cassapi.CassandraJob{{
					Name:    "job1",
					Command: "upgradesstables",
				}},
			},
		},
	}
	require.NoError(f.Client.Create(ctx, k8Task), "failed to create K8ssandraTask")

	cassTask1Key := newClusterKey(f.DataPlaneContexts[0], namespace, "upgradesstables-dc1")
	require.Eventually(f.CassTaskExists(ctx, cassTask1Key), timeout, interval)
	cassTask1 := &cassapi.CassandraTask{}
	require.NoError(f.Get(ctx, cassTask1Key, cassTask1), "failed to get CassandraTask in dc1")
	require.Equal("job1", cassTask1.Spec.Jobs[0].Name)
	require.Equal("upgradesstables", string(cassTask1.Spec.Jobs[0].Command))

	cassTask2Key := newClusterKey(f.DataPlaneContexts[1], namespace, "upgradesstables-dc2")
	require.Eventually(f.CassTaskExists(ctx, cassTask2Key), timeout, interval)
	cassTask2 := &cassapi.CassandraTask{}
	require.NoError(f.Get(ctx, cassTask2Key, cassTask2), "failed to get CassandraTask in dc2")
	require.Equal("job1", cassTask2.Spec.Jobs[0].Name)
	require.Equal("upgradesstables", string(cassTask2.Spec.Jobs[0].Command))
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
			ServerVersion: "3.11.10",
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
