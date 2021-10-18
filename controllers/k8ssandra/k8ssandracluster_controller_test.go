package k8ssandra

import (
	"context"
	"fmt"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/go-logr/logr"

	"strconv"
	"strings"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	replicationctrl "github.com/k8ssandra/k8ssandra-operator/controllers/replication"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
        "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
)

const (
	timeout  = time.Second * 5
	interval = time.Millisecond * 500
)

var (
	defaultStorageClass = "default"
	testEnv             *testutils.MultiClusterTestEnv
	seedsResolver       = &fakeSeedsResolver{}
	managementApi       = &fakeManagementApiFactory{}
)

func TestK8ssandraCluster(t *testing.T) {
	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)
	testEnv = &testutils.MultiClusterTestEnv{}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&replicationctrl.SecretSyncController{
			ReconcilerConfig: reconcilerConfig,
			ClientCache:      clientCache,
		}).SetupWithManager(mgr, clusters)
		if err != nil {
			return err
		}

		err = (&K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			SeedsResolver:    seedsResolver,
			ManagementApi:    managementApi,
		}).SetupWithManager(mgr, clusters)
		return err
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	defer testEnv.Stop(t)
	defer cancel()

	t.Run("CreateSingleDcCluster", testEnv.ControllerTest(ctx, createSingleDcCluster))
	t.Run("CreateMultiDcCluster", testEnv.ControllerTest(ctx, createMultiDcCluster))
	t.Run("ApplyClusterTemplateConfigs", testEnv.ControllerTest(ctx, applyClusterTemplateConfigs))
	t.Run("ApplyDatacenterTemplateConfigs", testEnv.ControllerTest(ctx, applyDatacenterTemplateConfigs))
	t.Run("ApplyClusterTemplateAndDatacenterTemplateConfigs", testEnv.ControllerTest(ctx, applyClusterTemplateAndDatacenterTemplateConfigs))
	t.Run("CreateMultiDcClusterWithStargate", testEnv.ControllerTest(ctx, createMultiDcClusterWithStargate))
}

// createSingleDcCluster verifies that the CassandraDatacenter is created and that the
// expected status updates happen on the K8ssandraCluster.
func createSingleDcCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	k8sCtx := "cluster-1"

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Cluster: "test",
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext:    k8sCtx,
						Size:          1,
						ServerVersion: "3.11.10",
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
							},
						},
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	verifyDefaultSuperUserSecretCreated(ctx, t, f, kc)
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that the datacenter was created")
	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx}
	require.Eventually(f.DatacenterExists(ctx, dcKey), timeout, interval)

	lastTransitionTime := metav1.Now()

	t.Log("update datacenter status to scaling up")
	err = f.PatchDatacenterStatus(ctx, dcKey, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: lastTransitionTime,
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	kcKey := framework.ClusterKey{K8sContext: "cluster-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
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
		return condition != nil
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("update datacenter status to ready")
	err = f.PatchDatacenterStatus(ctx, dcKey, func(dc *cassdcapi.CassandraDatacenter) {
		lastTransitionTime = metav1.Now()
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: lastTransitionTime,
		})
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: lastTransitionTime,
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
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
		if condition == nil || condition.Status == corev1.ConditionTrue {
			return false
		}

		condition = findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			return false
		}

		return true
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dcKey, &cassdcapi.CassandraDatacenter{})
}

// applyClusterTemplateConfigs verifies that settings specified at the cluster-level, i.e.,
// in the CassandraClusterTemplate, are applied correctly to each CassandraDatacenter.
func applyClusterTemplateConfigs(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	k8sCtx0 := "cluster-0"
	k8sCtx1 := "cluster-1"

	clusterName := "cluster-configs"
	superUserSecretName := "test-superuser"
	serverVersion := "4.0.0"
	dc1Size := int32(6)
	dc2Size := int32(12)

	kluster := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Cluster:             clusterName,
				SuperuserSecretName: superUserSecretName,
				ServerVersion:       serverVersion,
				StorageConfig: &cassdcapi.StorageConfig{
					CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
						StorageClassName: &defaultStorageClass,
					},
				},
				CassandraConfig: &api.CassandraConfig{
					CassandraYaml: &api.CassandraYaml{
						ConcurrentReads:  intPtr(8),
						ConcurrentWrites: intPtr(16),
					},
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: k8sCtx0,
						Size:       dc1Size,
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: k8sCtx1,
						Size:       dc2Size,
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kluster)
	require.NoError(err, "failed to create K8sandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kluster.Namespace, Name: kluster.Name})
	verifyReplicatedSecretReconciled(ctx, t, f, kluster)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	klusterKey := client.ObjectKey{Namespace: kluster.Namespace, Name: kluster.Name}
	err = f.Client.Get(ctx, klusterKey, kluster)
	require.NoError(err, "failed to get K8ssandraCluster")

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kluster.Spec.Cassandra.Cluster, dc1.Spec.ClusterName)
	assert.Equal(kluster.Spec.Cassandra.ServerVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kluster.Spec.Cassandra.StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(dc1Size, dc1.Spec.Size)
	assert.Equal(dc1.Spec.SuperuserSecretName, superUserSecretName)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kluster.Spec.Cassandra.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: k8sCtx1}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("verify configuration of dc2")
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(kluster.Spec.Cassandra.Cluster, dc2.Spec.ClusterName)
	assert.Equal(kluster.Spec.Cassandra.ServerVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kluster.Spec.Cassandra.StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(dc2Size, dc2.Spec.Size)
	assert.Equal(dc1.Spec.SuperuserSecretName, superUserSecretName)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kluster.Spec.Cassandra.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kluster.Namespace, Name: kluster.Name})
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, dc2Key, &cassdcapi.CassandraDatacenter{})
}

// applyDatacenterTemplateConfigs verifies that settings specified at the dc-level, i.e.,
// in the DatacenterTemplate, are applied correctly to each CassandraDatacenter.
func applyDatacenterTemplateConfigs(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	k8sCtx0 := "cluster-0"
	k8sCtx1 := "cluster-1"

	clusterName := "cluster-configs"
	serverVersion := "4.0.0"
	dc1Size := int32(12)
	dc2Size := int32(30)

	kluster := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Cluster:       clusterName,
				ServerVersion: serverVersion,
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext:    k8sCtx0,
						Size:          dc1Size,
						ServerVersion: serverVersion,
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: *parseResource("500Gi"),
									},
								},
							},
						},
						Networking: &cassdcapi.NetworkingConfig{
							NodePort: &cassdcapi.NodePortConfig{
								Native: 9142,
							},
						},
						CassandraConfig: &api.CassandraConfig{
							CassandraYaml: &api.CassandraYaml{
								ConcurrentReads:  intPtr(4),
								ConcurrentWrites: intPtr(4),
							},
							JvmOptions: &api.JvmOptions{
								HeapSize: parseResource("1024Mi"),
							},
						},
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: k8sCtx1,
						Size:       dc2Size,
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: *parseResource("2Ti"),
									},
								},
							},
						},
						Networking: &cassdcapi.NetworkingConfig{
							NodePort: &cassdcapi.NodePortConfig{
								Native: 9242,
							},
						},
						CassandraConfig: &api.CassandraConfig{
							CassandraYaml: &api.CassandraYaml{
								ConcurrentReads:  intPtr(4),
								ConcurrentWrites: intPtr(12),
							},
							JvmOptions: &api.JvmOptions{
								HeapSize: parseResource("2048Mi"),
							},
						},
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kluster)
	require.NoError(err, "failed to create K8sandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kluster.Namespace, Name: kluster.Name})
	verifyDefaultSuperUserSecretCreated(ctx, t, f, kluster)
	verifyReplicatedSecretReconciled(ctx, t, f, kluster)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kluster.Spec.Cassandra.Cluster, dc1.Spec.ClusterName)
	assert.Equal(serverVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kluster.Spec.Cassandra.Datacenters[0].StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(kluster.Spec.Cassandra.Datacenters[0].Networking, dc1.Spec.Networking)
	assert.Equal(dc1Size, dc1.Spec.Size)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kluster.Spec.Cassandra.Datacenters[0].CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: k8sCtx1}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("verify configuration of dc2")
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(kluster.Spec.Cassandra.Cluster, dc2.Spec.ClusterName)
	assert.Equal(serverVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kluster.Spec.Cassandra.Datacenters[1].StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(kluster.Spec.Cassandra.Datacenters[1].Networking, dc2.Spec.Networking)
	assert.Equal(dc2Size, dc2.Spec.Size)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kluster.Spec.Cassandra.Datacenters[1].CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kluster.Namespace, Name: kluster.Name})
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, dc2Key, &cassdcapi.CassandraDatacenter{})
}

// applyClusterTemplateAndDatacenterTemplateConfigs specifies settings in the cluster
// template and verifies that they are applied to dc1. It specifies dc template settings
// for dc2 and verifies that they are applied.
func applyClusterTemplateAndDatacenterTemplateConfigs(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	k8sCtx0 := "cluster-0"
	k8sCtx1 := "cluster-1"

	clusterName := "cluster-configs"
	serverVersion := "4.0.0"
	dc1Size := int32(12)
	dc2Size := int32(30)

	kluster := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Cluster:       clusterName,
				ServerVersion: serverVersion,
				StorageConfig: &cassdcapi.StorageConfig{
					CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
						StorageClassName: &defaultStorageClass,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: *parseResource("500Gi"),
							},
						},
					},
				},
				Networking: &cassdcapi.NetworkingConfig{
					HostNetwork: true,
				},
				CassandraConfig: &api.CassandraConfig{
					CassandraYaml: &api.CassandraYaml{
						ConcurrentReads:  intPtr(4),
						ConcurrentWrites: intPtr(4),
					},
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext:    k8sCtx0,
						Size:          dc1Size,
						ServerVersion: serverVersion,
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: k8sCtx1,
						Size:       dc2Size,
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: *parseResource("2Ti"),
									},
								},
							},
						},
						Networking: &cassdcapi.NetworkingConfig{
							HostNetwork: false,
						},
						CassandraConfig: &api.CassandraConfig{
							CassandraYaml: &api.CassandraYaml{
								ConcurrentReads:  intPtr(4),
								ConcurrentWrites: intPtr(12),
							},
							JvmOptions: &api.JvmOptions{
								HeapSize: parseResource("2048Mi"),
							},
						},
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kluster)
	require.NoError(err, "failed to create K8sandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kluster.Namespace, Name: kluster.Name})
	verifyDefaultSuperUserSecretCreated(ctx, t, f, kluster)
	verifyReplicatedSecretReconciled(ctx, t, f, kluster)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kluster.Spec.Cassandra.Cluster, dc1.Spec.ClusterName)
	assert.Equal(serverVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kluster.Spec.Cassandra.StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(kluster.Spec.Cassandra.Networking, dc1.Spec.Networking)
	assert.Equal(dc1Size, dc1.Spec.Size)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kluster.Spec.Cassandra.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: k8sCtx1}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("verify configuration of dc2")
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(kluster.Spec.Cassandra.Cluster, dc2.Spec.ClusterName)
	assert.Equal(serverVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kluster.Spec.Cassandra.Datacenters[1].StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(kluster.Spec.Cassandra.Datacenters[1].Networking, dc2.Spec.Networking)
	assert.Equal(dc2Size, dc2.Spec.Size)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kluster.Spec.Cassandra.Datacenters[1].CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kluster.Namespace, Name: kluster.Name})
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, dc2Key, &cassdcapi.CassandraDatacenter{})
}

func parseCassandraConfig(config *api.CassandraConfig, serverVersion string, systemRF int, dcNames ...string) (*gabs.Container, error) {
	config = config.DeepCopy()
	if config.JvmOptions == nil {
		config.JvmOptions = &api.JvmOptions{}
	}
	if config.JvmOptions.AdditionalOptions == nil {
		config.JvmOptions.AdditionalOptions = make([]string, 0)
	}
	additionalOpts := config.JvmOptions.AdditionalOptions

	dcNamesOpt := "-Dcassandra.system_distributed_replication_dc_names=" + strings.Join(dcNames, ",")
	rfOpt := "-Dcassandra.system_distributed_replication_per_dc=" + strconv.Itoa(systemRF)
	additionalOpts = append(additionalOpts, dcNamesOpt, rfOpt)

	config.JvmOptions.AdditionalOptions = additionalOpts

	json, err := cassandra.CreateJsonConfig(config, serverVersion)
	if err != nil {
		return nil, err
	}

	return gabs.ParseJSON(json)
}

// createMultiDcCluster verifies that the CassandraDatacenters are created in the expected
// k8s clusters. It also verifies that status updates are made to the K8ssandraCluster.
func createMultiDcCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	k8sCtx0 := "cluster-0"
	k8sCtx1 := "cluster-1"

	cluster := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Cluster: "test",
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext:    k8sCtx0,
						Size:          3,
						ServerVersion: "3.11.10",
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
							},
						},
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext:    k8sCtx1,
						Size:          3,
						ServerVersion: "3.11.10",
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
							},
						},
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, cluster)
	require.NoError(err, "failed to create K8ssandraCluster")

	dc1PodIps := []string{"10.10.100.1", "10.10.100.2", "10.10.100.3"}
	dc2PodIps := []string{"10.11.100.1", "10.11.100.2", "10.11.100.3"}

	allPodIps := make([]string, 0, 6)
	allPodIps = append(allPodIps, dc1PodIps...)
	allPodIps = append(allPodIps, dc2PodIps...)

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name})
	verifyDefaultSuperUserSecretCreated(ctx, t, f, cluster)
	verifyReplicatedSecretReconciled(ctx, t, f, cluster)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("check that replicatedSecret was created")
	replicatedSecretKey := types.NamespacedName{Name: cluster.Spec.Cassandra.Cluster, Namespace: namespace}
	require.Eventually(func() bool {
		sec := &replicationapi.ReplicatedSecret{}
		err = f.Client.Get(ctx, replicatedSecretKey, sec)
		if err != nil {
			return false
		}

		return len(sec.Labels) > 0
	}, timeout, interval, "Failed to find replicatedSecret")

	t.Log("check that superuserSecret was created")
	superuserSecretKey := types.NamespacedName{Name: secret.DefaultSuperuserSecretName(cluster.Spec.Cassandra.Cluster), Namespace: namespace}
	require.Eventually(func() bool {
		sec := &corev1.Secret{}
		err = f.Client.Get(ctx, superuserSecretKey, sec)
		if err != nil {
			return false
		}

		return sec.Data != nil && len(sec.Data) == 2
	}, timeout, interval, "Failed to find superuserSecret")

	t.Log("check that superUserSecret was replicated to both clusters")
	var empty struct{}
	require.Eventually(func() bool {
		return verifySecretsMatch(t, ctx, f.Client, []string{k8sCtx0, k8sCtx1}, map[string]struct{}{
			superuserSecretKey.Name: empty,
		}, namespace)
	}, timeout, interval, "Failed to find matching superuserSecret from all clusters")

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
		kc := &api.K8ssandraCluster{}
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
		return !(condition == nil && condition.Status == corev1.ConditionFalse)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("check that dc2 has not been created yet")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: k8sCtx1}
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.True(err != nil && errors.IsNotFound(err), "dc2 should not be created until dc1 is ready")

	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		if dc.Name == "dc1" {
			return dc1PodIps, nil
		}
		if dc.Name == "dc2" {
			return dc2PodIps, nil
		}
		return nil, fmt.Errorf("unknown datacenter: %s", dc.Name)
	}

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("check that remote seeds are set on dc2")
	dc2 = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(dc1PodIps, dc2.Spec.AdditionalSeeds, "The AdditionalSeeds property for dc2 is wrong")

	t.Log("update dc2 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	// Commenting out the following check for now to due to
	// https://github.com/k8ssandra/k8ssandra-operator/issues/67
	//
	// t.Log("check that remote seeds are set on dc1")
	// err = wait.Poll(interval, timeout, func() (bool, error) {
	//	dc := &cassdcapi.CassandraDatacenter{}
	//	if err = f.Get(ctx, dc1Key, dc); err != nil {
	//		t.Logf("failed to get dc1: %s", err)
	//		return false, err
	//	}
	//	t.Logf("additional seeds for dc1: %v", dc.Spec.AdditionalSeeds)
	//	return equalsNoOrder(allPodIps, dc.Spec.AdditionalSeeds), nil
	// })
	// require.NoError(err, "timed out waiting for remote seeds to be updated on dc1")

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
		err = f.Get(ctx, kcKey, kc)
		if err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}

		if len(kc.Status.Datacenters) != 2 {
			return false
		}

		k8ssandraStatus, found := kc.Status.Datacenters[dc1Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc1Key)
			return false
		}

		condition := findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			return false
		}

		k8ssandraStatus, found = kc.Status.Datacenters[dc2Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc2Key)
			return false
		}

		condition = findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		return condition != nil && condition.Status == corev1.ConditionTrue
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name})
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, dc2Key, &cassdcapi.CassandraDatacenter{})
}

func createMultiDcClusterWithStargate(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	k8sCtx0 := "cluster-0"
	k8sCtx1 := "cluster-1"

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Cluster: "test",
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext:    k8sCtx0,
						Size:          3,
						ServerVersion: "3.11.10",
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
							},
						},
						Stargate: &stargateapi.StargateDatacenterTemplate{
							StargateClusterTemplate: stargateapi.StargateClusterTemplate{
								Size: 1,
							},
						},
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext:    k8sCtx1,
						Size:          3,
						ServerVersion: "3.11.10",
						StorageConfig: &cassdcapi.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
								StorageClassName: &defaultStorageClass,
							},
						},
						Stargate: &stargateapi.StargateDatacenterTemplate{
							StargateClusterTemplate: stargateapi.StargateClusterTemplate{
								Size: 1,
							},
						},
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	dc1PodIps := []string{"10.10.100.1", "10.10.100.2", "10.10.100.3"}
	dc2PodIps := []string{"10.11.100.1", "10.11.100.2", "10.11.100.3"}

	allPodIps := make([]string, 0, 6)
	allPodIps = append(allPodIps, dc1PodIps...)
	allPodIps = append(allPodIps, dc2PodIps...)

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	verifyDefaultSuperUserSecretCreated(ctx, t, f, kc)
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
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
		kc := &api.K8ssandraCluster{}
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
		return !(condition == nil && condition.Status == corev1.ConditionFalse)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	sg1Key := framework.ClusterKey{
		K8sContext: k8sCtx0,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      kc.Name + "-" + dc1Key.Name + "-stargate"},
	}

	t.Logf("check that stargate %s has not been created", sg1Key)
	sg1 := &stargateapi.Stargate{}
	err = f.Get(ctx, sg1Key, sg1)
	require.True(err != nil && errors.IsNotFound(err), fmt.Sprintf("stargate %s should not be created until dc1 is ready", sg1Key))

	t.Log("check that dc2 has not been created yet")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: k8sCtx1}
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.True(err != nil && errors.IsNotFound(err), "dc2 should not be created until dc1 is ready")

	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		if dc.Name == "dc1" {
			return dc1PodIps, nil
		}
		if dc.Name == "dc2" {
			return dc2PodIps, nil
		}
		return nil, fmt.Errorf("unknown datacenter: %s", dc.Name)
	}

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

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("check that remote seeds are set on dc2")
	dc2 = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(dc1PodIps, dc2.Spec.AdditionalSeeds, "The AdditionalSeeds property for dc2 is wrong")

	sg2Key := framework.ClusterKey{
		K8sContext: k8sCtx1,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      kc.Name + "-" + dc2Key.Name + "-stargate"},
	}

	t.Log("update dc2 status to ready")
	err = f.PatchDatacenterStatus(ctx, dc2Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to update dc2 status to ready")

	t.Log("check that stargate sg1 is created")
	require.Eventually(f.StargateExists(ctx, sg1Key), timeout, interval)

	t.Logf("update stargate sg1 status to ready")
	err = f.PatchStagateStatus(ctx, sg1Key, func(sg *stargateapi.Stargate) {
		now := metav1.Now()
		sg.Status.Progress = stargateapi.StargateProgressRunning
		sg.Status.AvailableReplicas = 1
		sg.Status.Replicas = 1
		sg.Status.ReadyReplicas = 1
		sg.Status.UpdatedReplicas = 1
		sg.Status.SetCondition(stargateapi.StargateCondition{
			Type:               stargateapi.StargateReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
	})
	require.NoError(err, "failed to patch stargate status")

	t.Logf("check that stargate %s has not been created", sg2Key)
	sg2 := &stargateapi.Stargate{}
	err = f.Get(ctx, sg2Key, sg2)
	require.True(err != nil && errors.IsNotFound(err), fmt.Sprintf("stargate %s should not be created until dc2 is ready", sg2Key))

	t.Log("check that stargate sg2 is created")
	require.Eventually(f.StargateExists(ctx, sg2Key), timeout, interval)

	// Commenting out the following check for now to due to
	// https://github.com/k8ssandra/k8ssandra-operator/issues/67
	//
	// t.Log("check that remote seeds are set on dc1")
	// err = wait.Poll(interval, timeout, func() (bool, error) {
	//	dc := &cassdcapi.CassandraDatacenter{}
	//	if err = f.Get(ctx, dc1Key, dc); err != nil {
	//		t.Logf("failed to get dc1: %s", err)
	//		return false, err
	//	}
	//	t.Logf("additional seeds for dc1: %v", dc.Spec.AdditionalSeeds)
	//	return equalsNoOrder(allPodIps, dc.Spec.AdditionalSeeds), nil
	// })
	// require.NoError(err, "timed out waiting for remote seeds to be updated on dc1")

	t.Logf("update stargate sg2 status to ready")
	err = f.PatchStagateStatus(ctx, sg2Key, func(sg *stargateapi.Stargate) {
		now := metav1.Now()
		sg.Status.Progress = stargateapi.StargateProgressRunning
		sg.Status.AvailableReplicas = 1
		sg.Status.Replicas = 1
		sg.Status.ReadyReplicas = 1
		sg.Status.UpdatedReplicas = 1
		sg.Status.SetCondition(stargateapi.StargateCondition{
			Type:               stargateapi.StargateReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
	})
	require.NoError(err, "failed to patch stargate status")

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
		err = f.Get(ctx, kcKey, kc)
		if err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}

		if len(kc.Status.Datacenters) != 2 {
			return false
		}

		k8ssandraStatus, found := kc.Status.Datacenters[dc1Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc1Key)
			return false
		}

		condition := findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			t.Logf("k8ssandracluster status check failed: cassandra in %s is not ready", dc1Key.Name)
			return false
		}

		if k8ssandraStatus.Stargate == nil || !k8ssandraStatus.Stargate.IsReady() {
			t.Logf("k8ssandracluster status check failed: stargate in %s is not ready", dc1Key.Name)
		}

		k8ssandraStatus, found = kc.Status.Datacenters[dc2Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc2Key)
			return false
		}

		condition = findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			t.Logf("k8ssandracluster status check failed: cassandra in %s is not ready", dc2Key.Name)
			return false
		}

		if k8ssandraStatus.Stargate == nil || !k8ssandraStatus.Stargate.IsReady() {
			t.Logf("k8ssandracluster status check failed: stargate in %s is not ready", dc2Key.Name)
			return false
		}

		return true
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("remove both stargates from kc spec")
	err = f.Get(ctx, kcKey, kc)
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Cassandra.Datacenters[0].Stargate = nil
	kc.Spec.Cassandra.Datacenters[1].Stargate = nil
	err = f.Client.Patch(ctx, kc, patch)
	require.NoError(err, "failed to update K8ssandraCluster")

	t.Log("check that stargate sg1 is deleted")
	require.Eventually(func() bool {
		err = f.Get(ctx, sg1Key, &stargateapi.Stargate{})
		return errors.IsNotFound(err)
	}, timeout, interval)

	t.Log("check that stargate sg2 is deleted")
	require.Eventually(func() bool {
		err = f.Get(ctx, sg2Key, &stargateapi.Stargate{})
		return errors.IsNotFound(err)
	}, timeout, interval)

	t.Log("check that kc status is updated")
	require.Eventually(func() bool {
		err = f.Get(ctx, kcKey, kc)
		require.NoError(err, "failed to get K8ssandraCluster")
		return kc.Status.Datacenters[dc1Key.Name].Stargate == nil &&
			kc.Status.Datacenters[dc2Key.Name].Stargate == nil
	}, timeout, interval)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, dc2Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, sg1Key, &stargateapi.Stargate{})
	verifyObjectDoesNotExist(ctx, t, f, sg2Key, &stargateapi.Stargate{})
}

func verifyDefaultSuperUserSecretCreated(ctx context.Context, t *testing.T, f *framework.Framework, kluster *api.K8ssandraCluster) {
	t.Logf("check that the default superuser secret is created")
	assert.Eventually(t, func() bool {
		secretName := secret.DefaultSuperuserSecretName(kluster.Spec.Cassandra.Cluster)
		defaultSecret := &corev1.Secret{}
		if err := f.Client.Get(ctx, types.NamespacedName{Namespace: kluster.Namespace, Name: secretName}, defaultSecret); err != nil {
			t.Logf("failed to get superuser secret: %v", err)
			return false
		}
		return true
	}, timeout, interval, "failed to verify that the default superuser secret was created")
}

func verifyFinalizerAdded(ctx context.Context, t *testing.T, f *framework.Framework, key client.ObjectKey) {
	t.Log("check finalizer added to K8ssandraCluster")

	assert.Eventually(t, func() bool {
		kc := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, key, kc); err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}
		return controllerutil.ContainsFinalizer(kc, k8ssandraClusterFinalizer)
	}, timeout, interval, "failed to verify that finalizer was added")
}

func verifyObjectDoesNotExist(ctx context.Context, t *testing.T, f *framework.Framework, key framework.ClusterKey, obj client.Object) {
	assert.Eventually(t, func() bool {
		err := f.Get(ctx, key, obj)
		return err != nil && errors.IsNotFound(err)
	}, timeout, interval, "failed to verify object does not exist", key)
}

func verifyReplicatedSecretReconciled(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	t.Log("check ReplicatedSecret reconciled")

	replSecret := &replicationapi.ReplicatedSecret{}
	replSecretKey := types.NamespacedName{Name: kc.Spec.Cassandra.Cluster, Namespace: kc.Namespace}

	assert.Eventually(t, func() bool {
		err := f.Client.Get(ctx, replSecretKey, replSecret)
		return err == nil
	}, timeout, interval, "failed to get ReplicatedSecret")

	if replSecret == nil {
		return
	}

	val, exists := replSecret.Labels[api.ManagedByLabel]
	assert.True(t, exists)
	assert.Equal(t, api.NameLabelValue, val)
	val, exists = replSecret.Labels[api.K8ssandraClusterLabel]
	assert.True(t, exists)
	assert.Equal(t, kc.Spec.Cassandra.Cluster, val)

	assert.Equal(t, len(kc.Spec.Cassandra.Datacenters), len(replSecret.Spec.ReplicationTargets))
}

func findDatacenterCondition(status *cassdcapi.CassandraDatacenterStatus, condType cassdcapi.DatacenterConditionType) *cassdcapi.DatacenterCondition {
	for _, condition := range status.Conditions {
		if condition.Type == condType {
			return &condition
		}
	}
	return nil
}

func intPtr(n int) *int {
	return &n
}

func parseResource(quantity string) *resource.Quantity {
	parsed := resource.MustParse(quantity)
	return &parsed
}

type fakeSeedsResolver struct {
	callback func(dc *cassdcapi.CassandraDatacenter) ([]string, error)
}

func (r *fakeSeedsResolver) ResolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error) {
	return r.callback(dc)
}

type fakeManagementApiFactory struct {
}

func (f fakeManagementApiFactory) NewManagementApiFacade(context.Context, *cassdcapi.CassandraDatacenter, client.Client, logr.Logger) (cassandra.ManagementApiFacade, error) {
	m := new(mocks.ManagementApiFacade)
	m.On("CreateKeyspaceIfNotExists", stargate.AuthKeyspace, mock.Anything).Return(nil)
	m.On("ListKeyspaces", stargate.AuthKeyspace).Return([]string{stargate.AuthKeyspace}, nil)
	m.On("AlterKeyspace", stargate.AuthKeyspace, mock.Anything).Return(nil)
	m.On("GetKeyspaceReplication", stargate.AuthKeyspace).Return(
		map[string]string{
			"class": "org.apache.cassandra.locator.NetworkTopologyStrategy",
			"dc1":   "1",
		},
		nil)
	m.On("ListTables", stargate.AuthKeyspace).Return([]string{"token"}, nil)
	m.On("CreateTable", mock.MatchedBy(func(def *httphelper.TableDefinition) bool {
		return def.KeyspaceName == stargate.AuthKeyspace && def.TableName == stargate.AuthTable
	})).Return(nil)
	return m, nil
}

// verifySecretsMatch checks that the same secret is copied to other clusters
func verifySecretsMatch(t *testing.T, ctx context.Context, localClient client.Client, remoteClusters []string, secrets map[string]struct{}, namespace string) bool {
	secretList := &corev1.SecretList{}
	err := localClient.List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return false
	}

	for _, remoteCluster := range remoteClusters {
		testClient := testEnv.Clients[remoteCluster]

		targetSecretList := &corev1.SecretList{}
		err := testClient.List(ctx, targetSecretList, client.InNamespace(namespace))
		if err != nil {
			return false
		}

		for _, s := range secretList.Items {
			if _, exists := secrets[s.Name]; exists {
				// Find the corresponding item from targetSecretList - or fail if it's not there
				found := false
				for _, ts := range targetSecretList.Items {
					if s.Name == ts.Name {
						found = true
						if s.GetAnnotations()[api.ResourceHashAnnotation] != ts.GetAnnotations()[api.ResourceHashAnnotation] {
							return false
						}
						break
					}
				}
				if !found {
					return false
				}
			}
		}
	}

	return true
}
