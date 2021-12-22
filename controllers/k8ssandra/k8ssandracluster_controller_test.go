package k8ssandra

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/mock"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"reflect"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

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
		err := (&K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
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
	t.Run("CreateMultiDcClusterWithReaper", testEnv.ControllerTest(ctx, createMultiDcClusterWithReaper))
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

	verifySuperUserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

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

		if (&kc.Status).GetConditionStatus(api.CassandraInitialized) != corev1.ConditionTrue {
			t.Logf("Expected status condition %s to be true", api.CassandraInitialized)
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

	//Test that prometheus servicemonitor comes up when it is requested in the CassandraDatacenter.
	kcPatch := client.MergeFrom(kc.DeepCopy())
	kc.Spec.Cassandra.Datacenters[0].CassandraTelemetry = &telemetryapi.TelemetrySpec{
		Prometheus: &telemetryapi.PrometheusTelemetrySpec{
			Enabled: true,
		},
	}
	if err := f.Patch(ctx, kc, kcPatch, kcKey); err != nil {
		assert.Fail(t, "got error patching for telemetry", "error", err)
	}
	if err = f.SetDatacenterStatusReady(ctx, framework.ClusterKey{
		NamespacedName: types.NamespacedName{
			Name:      "dc1",
			Namespace: namespace,
		},
		K8sContext: "cluster-1",
	}); err != nil {
		assert.Fail(t, "error setting status ready", err)
	}
	//	Check for presence of expected ServiceMonitor for Cassandra Datacenter
	sm := &promapi.ServiceMonitor{}
	smKey := framework.ClusterKey{
		NamespacedName: types.NamespacedName{
			Name:      kc.Name + "-" + kc.Spec.Cassandra.Datacenters[0].Meta.Name + "-" + "cass-servicemonitor",
			Namespace: namespace,
		},
		K8sContext: k8sCtx,
	}
	assert.Eventually(t, func() bool {
		if err := f.Get(ctx, smKey, sm); err != nil {
			return false
		}
		return true
	}, timeout, interval)
	assert.NotNil(t, sm.Spec.Endpoints)
	// Ensure that removing the telemetry spec does delete the ServiceMonitor
	kcPatch = client.MergeFrom(kc.DeepCopy())
	kc.Spec.Cassandra.Datacenters[0].CassandraTelemetry = nil
	if err := f.Client.Patch(ctx, kc, kcPatch); err != nil {
		assert.Fail(t, "failed to patch stargate", "error", err)
	}
	assert.Eventually(t, func() bool {
		err := f.Get(ctx, smKey, sm)
		if err != nil {
			return k8serrors.IsNotFound(err)
		}
		return false
	}, timeout, interval)
	// Test cluster deletion
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name})
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

	kc := &api.K8ssandraCluster{
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

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8sandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperUserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	klusterKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	err = f.Client.Get(ctx, klusterKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster")

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kc.Spec.Cassandra.Cluster, dc1.Spec.ClusterName)
	assert.Equal(kc.Spec.Cassandra.ServerVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(dc1Size, dc1.Spec.Size)
	assert.Equal(dc1.Spec.SuperuserSecretName, superUserSecretName)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kc.Spec.Cassandra.CassandraConfig, serverVersion, 3, "dc1", "dc2")
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

	assert.Equal(kc.Spec.Cassandra.Cluster, dc2.Spec.ClusterName)
	assert.Equal(kc.Spec.Cassandra.ServerVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(dc2Size, dc2.Spec.Size)
	assert.Equal(dc1.Spec.SuperuserSecretName, superUserSecretName)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kc.Spec.Cassandra.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
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

	kc := &api.K8ssandraCluster{
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

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8sandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperUserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kc.Spec.Cassandra.Cluster, dc1.Spec.ClusterName)
	assert.Equal(serverVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.Datacenters[0].StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.Datacenters[0].Networking, dc1.Spec.Networking)
	assert.Equal(dc1Size, dc1.Spec.Size)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kc.Spec.Cassandra.Datacenters[0].CassandraConfig, serverVersion, 3, "dc1", "dc2")
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

	assert.Equal(kc.Spec.Cassandra.Cluster, dc2.Spec.ClusterName)
	assert.Equal(serverVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.Datacenters[1].StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.Datacenters[1].Networking, dc2.Spec.Networking)
	assert.Equal(dc2Size, dc2.Spec.Size)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kc.Spec.Cassandra.Datacenters[1].CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
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

	kc := &api.K8ssandraCluster{
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

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8sandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperUserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kc.Spec.Cassandra.Cluster, dc1.Spec.ClusterName)
	assert.Equal(serverVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.Networking, dc1.Spec.Networking)
	assert.Equal(dc1Size, dc1.Spec.Size)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kc.Spec.Cassandra.CassandraConfig, serverVersion, 3, "dc1", "dc2")
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

	assert.Equal(kc.Spec.Cassandra.Cluster, dc2.Spec.ClusterName)
	assert.Equal(serverVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.Datacenters[1].StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.Datacenters[1].Networking, dc2.Spec.Networking)
	assert.Equal(dc2Size, dc2.Spec.Size)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kc.Spec.Cassandra.Datacenters[1].CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
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

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperUserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

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

		if (&kc.Status).GetConditionStatus(api.CassandraInitialized) == corev1.ConditionTrue {
			t.Logf("Did not expect status condition %s to be true", api.CassandraInitialized)
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

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("check that remote seeds are set on dc2")
	dc2 = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	t.Log("update dc2 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to set dc2 status ready")

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
		err = f.Get(ctx, kcKey, kc)
		if err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}

		if (&kc.Status).GetConditionStatus(api.CassandraInitialized) != corev1.ConditionTrue {
			t.Logf("Expected status condition %s to be true", api.CassandraInitialized)
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
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	require.NoError(err, "failed to delete K8ssandraCluster")
	verifyObjectDoesNotExist(ctx, t, f, dc1Key, &cassdcapi.CassandraDatacenter{})
	verifyObjectDoesNotExist(ctx, t, f, dc2Key, &cassdcapi.CassandraDatacenter{})
}

func createMultiDcClusterWithStargate(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

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

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperUserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

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

		if (&kc.Status).GetConditionStatus(api.CassandraInitialized) == corev1.ConditionTrue {
			t.Logf("Did not expect status condition %s to be true", api.CassandraInitialized)
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
	err = f.PatchStargateStatus(ctx, sg1Key, func(sg *stargateapi.Stargate) {
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

	k := &api.K8ssandraCluster{}
	err = f.Get(ctx, kcKey, k)
	require.NoError(err)
	require.NotNil(k.Spec.Cassandra.Datacenters[1].Stargate)

	t.Logf("check that stargate %s has not been created", sg2Key)
	sg2 := &stargateapi.Stargate{}
	err = f.Get(ctx, sg2Key, sg2)
	require.True(err != nil && errors.IsNotFound(err), fmt.Sprintf("stargate %s should not be created until dc2 is ready", sg2Key))

	t.Log("check that stargate sg2 is created")
	require.Eventually(f.StargateExists(ctx, sg2Key), timeout, interval)

	t.Logf("update stargate sg2 status to ready")
	err = f.PatchStargateStatus(ctx, sg2Key, func(sg *stargateapi.Stargate) {
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

		if (&kc.Status).GetConditionStatus(api.CassandraInitialized) != corev1.ConditionTrue {
			t.Logf("Expected status condition %s to be true", api.CassandraInitialized)
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

func verifySuperUserSecretCreated(ctx context.Context, t *testing.T, f *framework.Framework, kluster *api.K8ssandraCluster) {
	t.Logf("check that the default superuser secret is created")

	secretName := kluster.Spec.Cassandra.SuperuserSecretName
	if secretName == "" {
		secretName = secret.DefaultSuperuserSecretName(kluster.Spec.Cassandra.Cluster)
	}

	assert.Eventually(t, func() bool {
		defaultSecret := &corev1.Secret{}
		if err := f.Client.Get(ctx, types.NamespacedName{Namespace: kluster.Namespace, Name: secretName}, defaultSecret); err != nil {
			t.Logf("failed to get superuser secret: %v", err)
			return false
		}
		return true
	}, timeout, interval, "failed to verify that the default superuser secret was created")
}

func verifySystemReplicationAnnotationSet(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	t.Logf("check that the %s annotation is set", api.SystemReplicationAnnotation)

	key := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	expectedReplication := cassandra.ComputeSystemReplication(kc)

	assert.Eventually(t, func() bool {
		kc = &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, key, kc); err != nil {
			t.Logf("Failed to check system replication annotation. Could not retrieve the K8ssandraCluster: %v", err)
			return false
		}

		val, found := kc.Annotations[api.SystemReplicationAnnotation]
		if !found {
			return false
		}

		actualReplication := &cassandra.SystemReplication{}
		if err := json.Unmarshal([]byte(val), actualReplication); err != nil {
			t.Logf("Failed to unmarshal system replication annotation: %v", err)
			return false
		}

		return reflect.DeepEqual(expectedReplication, *actualReplication)
	}, timeout, interval, "Failed to verify that the system replication annotation was set correctly")

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

	rsec := &replicationapi.ReplicatedSecret{}
	replSecretKey := types.NamespacedName{Name: kc.Name, Namespace: kc.Namespace}

	assert.Eventually(t, func() bool {
		err := f.Client.Get(ctx, replSecretKey, rsec)
		return err == nil
	}, timeout, interval, "failed to get ReplicatedSecret")

	val, exists := rsec.Labels[api.ManagedByLabel]
	assert.True(t, exists)
	assert.Equal(t, api.NameLabelValue, val)
	val, exists = rsec.Labels[api.K8ssandraClusterNameLabel]
	assert.True(t, exists)
	assert.Equal(t, kc.Name, val)
	val, exists = rsec.Labels[api.K8ssandraClusterNamespaceLabel]
	assert.True(t, exists)
	assert.Equal(t, kc.Namespace, val)

	assert.Equal(t, len(kc.Spec.Cassandra.Datacenters), len(rsec.Spec.ReplicationTargets))

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
	m.On("EnsureKeyspaceReplication", mock.Anything, mock.Anything).Return(nil)
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
