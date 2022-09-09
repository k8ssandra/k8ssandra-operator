package k8ssandra

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"

	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/utils/pointer"

	"strings"

	"github.com/Jeffail/gabs"

	"github.com/Masterminds/semver/v3"
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
	defaultStorageClass  = "default"
	testEnv              *testutils.MultiClusterTestEnv
	managementApiFactory = &testutils.FakeManagementApiFactory{}
)

func TestK8ssandraCluster(t *testing.T) {
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
		err := (&K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApiFactory,
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
	t.Run("AddDcToExistingCluster", testEnv.ControllerTest(ctx, addDc))
	t.Run("ApplyClusterTemplateConfigs", testEnv.ControllerTest(ctx, applyClusterTemplateConfigs))
	t.Run("ApplyDatacenterTemplateConfigs", testEnv.ControllerTest(ctx, applyDatacenterTemplateConfigs))
	t.Run("ApplyClusterTemplateAndDatacenterTemplateConfigs", testEnv.ControllerTest(ctx, applyClusterTemplateAndDatacenterTemplateConfigs))
	t.Run("CreateMultiDcClusterWithStargate", testEnv.ControllerTest(ctx, createMultiDcClusterWithStargate))
	t.Run("CreateMultiDcClusterWithReaper", testEnv.ControllerTest(ctx, createMultiDcClusterWithReaper))
	t.Run("CreateMultiDcClusterWithMedusa", testEnv.ControllerTest(ctx, createMultiDcClusterWithMedusa))
	t.Run("CreateSingleDcClusterNoAuth", testEnv.ControllerTest(ctx, createSingleDcClusterNoAuth))
	t.Run("CreateSingleDcClusterAuth", testEnv.ControllerTest(ctx, createSingleDcClusterAuth))
	// If webhooks are installed, this testcase is handled by the webhook test
	// t.Run("ChangeNumTokensValue", testEnv.ControllerTest(ctx, changeNumTokensValue))
	t.Run("ApplyClusterWithEncryptionOptions", testEnv.ControllerTest(ctx, applyClusterWithEncryptionOptions))
	t.Run("ApplyClusterWithEncryptionOptionsFail", testEnv.ControllerTest(ctx, applyClusterWithEncryptionOptionsFail))
	t.Run("StopDatacenter", testEnv.ControllerTest(ctx, stopDc))
	t.Run("ConvertSystemReplicationAnnotation", testEnv.ControllerTest(ctx, convertSystemReplicationAnnotation))
	t.Run("ChangeClusterNameFails", testEnv.ControllerTest(ctx, changeClusterNameFails))
	t.Run("InjectContainersAndVolumes", testEnv.ControllerTest(ctx, injectContainersAndVolumes))
}

// createSingleDcCluster verifies that the CassandraDatacenter is created and that the
// expected status updates happen on the K8ssandraCluster.
func createSingleDcCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				ClusterName: "Not K8s_Compliant",
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       1,
						DatacenterOptions: api.DatacenterOptions{
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
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the datacenter was created")
	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[1]}
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

	kcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}
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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
		if condition == nil || condition.Status == corev1.ConditionTrue {
			return false
		}

		condition = FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			return false
		}

		return true
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	// Test that prometheus servicemonitor comes up when it is requested in the CassandraDatacenter.
	kcPatch := client.MergeFrom(kc.DeepCopy())
	kc.Spec.Cassandra.Datacenters[0].DatacenterOptions.Telemetry = &telemetryapi.TelemetrySpec{
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
		K8sContext: f.DataPlaneContexts[1],
	}); err != nil {
		assert.Fail(t, "error setting status ready", err)
	}
	//	Check for presence of expected ServiceMonitor for Cassandra Datacenter
	sm := &promapi.ServiceMonitor{}
	smKey := framework.ClusterKey{
		NamespacedName: types.NamespacedName{
			Name:      kc.SanitizedName() + "-" + kc.Spec.Cassandra.Datacenters[0].Meta.Name + "-" + "cass-servicemonitor",
			Namespace: namespace,
		},
		K8sContext: f.DataPlaneContexts[1],
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
	kc.Spec.Cassandra.Datacenters[0].DatacenterOptions.Telemetry = nil
	if err := f.Client.Patch(ctx, kc, kcPatch); err != nil {
		assert.Fail(t, "failed to patch stargate", "error", err)
	}
	assert.Eventually(t, func() bool {
		err := f.Get(ctx, smKey, sm)
		if err != nil {
			return errors.IsNotFound(err)
		}
		return false
	}, timeout, interval)

	// Check that the datacenter has the original cluster name, without sanitization.
	dc1 := cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dcKey, &dc1)
	require.NoError(err, "failed to get CassandraDatacenter")
	require.Equal(kc.Spec.Cassandra.ClusterName, dc1.Spec.ClusterName)

	// Test cluster deletion
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

// applyClusterTemplateConfigs verifies that settings specified at the cluster-level, i.e.,
// in the CassandraClusterTemplate, are applied correctly to each CassandraDatacenter.
func applyClusterTemplateConfigs(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	clusterName := "cluster-configs"
	superUserSecretName := "test-superuser"
	serverVersion := "4.0.0"
	dc1Size := int32(6)
	dc2Size := int32(12)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				DatacenterOptions: api.DatacenterOptions{

					ServerVersion: serverVersion,
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
					CassandraConfig: &api.CassandraConfig{
						CassandraYaml: api.CassandraYaml{
							ConcurrentReads:  pointer.Int(8),
							ConcurrentWrites: pointer.Int(16),
						},
						JvmOptions: api.JvmOptions{
							MaxHeapSize: parseQuantity("1024Mi"),
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       dc1Size,
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       dc2Size,
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8sandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	klusterKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	err = f.Client.Get(ctx, klusterKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster")

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kc.Name, dc1.Spec.ClusterName)
	assert.Equal(kc.Spec.Cassandra.DatacenterOptions.ServerVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.DatacenterOptions.StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(dc1Size, dc1.Spec.Size)
	assert.Equal(dc1.Spec.SuperuserSecretName, superUserSecretName)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kc.Spec.Cassandra.DatacenterOptions.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("verify configuration of dc2")
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(kc.Name, dc2.Spec.ClusterName)
	assert.Equal(kc.Spec.Cassandra.DatacenterOptions.ServerVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.DatacenterOptions.StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(dc2Size, dc2.Spec.Size)
	assert.Equal(dc1.Spec.SuperuserSecretName, superUserSecretName)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kc.Spec.Cassandra.DatacenterOptions.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, dc2Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

// applyDatacenterTemplateConfigs verifies that settings specified at the dc-level, i.e.,
// in the DatacenterTemplate, are applied correctly to each CassandraDatacenter.
func applyDatacenterTemplateConfigs(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	clusterName := "cluster-configs"
	serverVersion := "4.0.0"
	dc1Size := int32(12)
	dc2Size := int32(30)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: serverVersion,
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       dc1Size,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: serverVersion,
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: *parseQuantity("500Gi"),
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
								CassandraYaml: api.CassandraYaml{
									ConcurrentReads:  pointer.Int(4),
									ConcurrentWrites: pointer.Int(4),
								},
								JvmOptions: api.JvmOptions{
									MaxHeapSize: parseQuantity("1024Mi"),
								},
							},
						},
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       dc2Size,
						DatacenterOptions: api.DatacenterOptions{
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: *parseQuantity("2Ti"),
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
								CassandraYaml: api.CassandraYaml{
									ConcurrentReads:  pointer.Int(4),
									ConcurrentWrites: pointer.Int(12),
								},
								JvmOptions: api.JvmOptions{
									MaxHeapSize: parseQuantity("1024Mi"),
								},
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

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kc.Name, dc1.Spec.ClusterName)
	assert.Equal(serverVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.Datacenters[0].DatacenterOptions.StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.Datacenters[0].DatacenterOptions.Networking, dc1.Spec.Networking)
	assert.Equal(dc1Size, dc1.Spec.Size)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kc.Spec.Cassandra.Datacenters[0].DatacenterOptions.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("verify configuration of dc2")
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(kc.Name, dc2.Spec.ClusterName)
	assert.Equal(serverVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.Datacenters[1].DatacenterOptions.StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.Datacenters[1].DatacenterOptions.Networking, dc2.Spec.Networking)
	assert.Equal(dc2Size, dc2.Spec.Size)

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kc.Spec.Cassandra.Datacenters[1].DatacenterOptions.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, dc2Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

// applyClusterTemplateAndDatacenterTemplateConfigs specifies settings in the cluster
// template and verifies that they are applied to dc1. It specifies dc template settings
// for dc2 and verifies that they are applied.
func applyClusterTemplateAndDatacenterTemplateConfigs(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	clusterName := "cluster-configs"
	serverVersion := "4.0.0"
	dc1Size := int32(12)
	dc2Size := int32(30)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: serverVersion,
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: *parseQuantity("500Gi"),
								},
							},
						},
					},
					Networking: &cassdcapi.NetworkingConfig{
						HostNetwork: true,
					},
					CassandraConfig: &api.CassandraConfig{
						CassandraYaml: api.CassandraYaml{
							ConcurrentReads:  pointer.Int(4),
							ConcurrentWrites: pointer.Int(4),
						},
						JvmOptions: api.JvmOptions{
							MaxHeapSize: parseQuantity("1024Mi"),
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       dc1Size,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: serverVersion,
						},
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       dc2Size,
						DatacenterOptions: api.DatacenterOptions{
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: *parseQuantity("2Ti"),
										},
									},
								},
							},
							Networking: &cassdcapi.NetworkingConfig{
								HostNetwork: false,
							},
							CassandraConfig: &api.CassandraConfig{
								CassandraYaml: api.CassandraYaml{
									ConcurrentReads:  pointer.Int(4),
									ConcurrentWrites: pointer.Int(12),
								},
								JvmOptions: api.JvmOptions{
									MaxHeapSize: parseQuantity("1024Mi"),
								},
							},
							CDC: &cassdcapi.CDCConfiguration{
								PulsarServiceUrl: pointer.String("pulsar://test-url"),
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

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")

	assert.Equal(kc.Name, dc1.Spec.ClusterName)
	assert.Equal(serverVersion, dc1.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.DatacenterOptions.StorageConfig, dc1.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.DatacenterOptions.Networking, dc1.Spec.Networking)
	assert.Equal(dc1Size, dc1.Spec.Size)

	actualConfig, err := gabs.ParseJSON(dc1.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc1 config %s", dc1.Spec.Config))

	expectedConfig, err := parseCassandraConfig(kc.Spec.Cassandra.DatacenterOptions.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("verify configuration of dc2")
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(kc.Name, dc2.Spec.ClusterName)
	assert.Equal(serverVersion, dc2.Spec.ServerVersion)
	assert.Equal(*kc.Spec.Cassandra.Datacenters[1].DatacenterOptions.StorageConfig, dc2.Spec.StorageConfig)
	assert.Equal(kc.Spec.Cassandra.Datacenters[1].DatacenterOptions.Networking, dc2.Spec.Networking)
	assert.Equal(dc2Size, dc2.Spec.Size)
	assert.Equal(*dc2.Spec.CDC.PulsarServiceUrl, "pulsar://test-url")

	actualConfig, err = gabs.ParseJSON(dc2.Spec.Config)
	require.NoError(err, fmt.Sprintf("failed to parse dc2 config %s", dc2.Spec.Config))

	expectedConfig, err = parseCassandraConfig(kc.Spec.Cassandra.Datacenters[1].DatacenterOptions.CassandraConfig, serverVersion, 3, "dc1", "dc2")
	require.NoError(err, "failed to parse CassandraConfig")
	assert.Equal(expectedConfig, actualConfig)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, dc2Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

func parseCassandraConfig(config *api.CassandraConfig, serverVersion string, systemRF int, dcNames ...string) (*gabs.Container, error) {
	sort.Strings(dcNames)
	config = config.DeepCopy()
	replicationFactors := make([]string, 0)
	for _, dc := range dcNames {
		replicationFactors = append(replicationFactors, fmt.Sprintf("%s:%d", dc, systemRF))
	}
	rfOpt := cassandra.SystemReplicationFactorStrategy + "=" + strings.Join(replicationFactors, ",")
	*config = cassandra.ApplyAuthSettings(*config, true)
	config.JvmOptions.AdditionalOptions = append(
		[]string{rfOpt, "-Dcom.sun.management.jmxremote.authenticate=true"},
		config.JvmOptions.AdditionalOptions...,
	)
	template := cassandra.DatacenterConfig{
		ServerVersion:   semver.MustParse(serverVersion),
		ServerType:      "cassandra",
		CassandraConfig: *config,
	}
	json, err := cassandra.CreateJsonConfig(&template)
	if err != nil {
		return nil, err
	}

	return gabs.ParseJSON(json)
}

// createMultiDcCluster verifies that the CassandraDatacenters are created in the expected
// k8s clusters. It also verifies that status updates are made to the K8ssandraCluster.
func createMultiDcCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	clusterName := "cluster-multi"
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.10",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
							},
						},
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
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
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
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

	kcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}

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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
		return !(condition == nil && condition.Status == corev1.ConditionFalse)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("check that dc2 has not been created yet")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.True(err != nil && errors.IsNotFound(err), "dc2 should not be created until dc1 is ready")

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			return false
		}

		k8ssandraStatus, found = kc.Status.Datacenters[dc2Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc2Key)
			return false
		}

		condition = FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		return condition != nil && condition.Status == corev1.ConditionTrue
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, dc2Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

func createSuperuserSecret(ctx context.Context, t *testing.T, f *framework.Framework, kcKey client.ObjectKey, secretName string) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kcKey.Namespace,
			Name:      secretName,
		},
		Data: map[string][]byte{},
	}
	labels.SetManagedBy(secret, kcKey)

	err := f.Client.Create(ctx, secret)
	require.NoError(t, err, "failed to create superuser secret")
}

func createReplicatedSecret(ctx context.Context, t *testing.T, f *framework.Framework, kcKey client.ObjectKey, replicationTargets ...string) {
	rsec := &replicationapi.ReplicatedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kcKey.Namespace,
			Name:      kcKey.Name,
		},
		Spec: replicationapi.ReplicatedSecretSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels.ManagedByLabels(kcKey),
			},
			ReplicationTargets: []replicationapi.ReplicationTarget{},
		},
		Status: replicationapi.ReplicatedSecretStatus{
			Conditions: []replicationapi.ReplicationCondition{
				{
					Type:   replicationapi.ReplicationDone,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	labels.SetManagedBy(rsec, kcKey)

	for _, val := range replicationTargets {
		rsec.Spec.ReplicationTargets = append(rsec.Spec.ReplicationTargets, replicationapi.ReplicationTarget{
			Namespace:      kcKey.Namespace,
			K8sContextName: val,
		})
	}

	err := f.Client.Create(ctx, rsec)
	require.NoError(t, err, "failed to create replicated secret")
}

func setReplicationStatusDone(ctx context.Context, t *testing.T, f *framework.Framework, key client.ObjectKey) {
	rsec := &replicationapi.ReplicatedSecret{}
	err := f.Client.Get(ctx, key, rsec)
	require.NoError(t, err, "failed to get ReplicatedSecret", "key", key)

	now := metav1.Now()
	conditions := make([]replicationapi.ReplicationCondition, 0)

	for _, target := range rsec.Spec.ReplicationTargets {
		conditions = append(conditions, replicationapi.ReplicationCondition{
			Cluster:            target.K8sContextName,
			Type:               replicationapi.ReplicationDone,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
	}
	rsec.Status.Conditions = conditions
	err = f.Client.Status().Update(ctx, rsec)

	require.NoError(t, err, "Failed to update ReplicationSecret status", "key", key)
}

func createMultiDcClusterWithStargate(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	clusterName := "cluster-multi-stargate"
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.10",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
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
						K8sContext: f.DataPlaneContexts[1],
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.10",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
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

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
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

	kcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}

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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
		return !(condition == nil && condition.Status == corev1.ConditionFalse)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	sg1Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[0],
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      kc.Name + "-" + dc1Key.Name + "-stargate"},
	}

	t.Logf("check that stargate %s has not been created", sg1Key)
	sg1 := &stargateapi.Stargate{}
	err = f.Get(ctx, sg1Key, sg1)
	require.True(err != nil && errors.IsNotFound(err), fmt.Sprintf("stargate %s should not be created until dc1 is ready", sg1Key))

	t.Log("check that dc2 has not been created yet")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.True(err != nil && errors.IsNotFound(err), "dc2 should not be created until dc1 is ready")

	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to update dc1 status to ready")

	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	sg2Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      kc.Name + "-" + dc2Key.Name + "-stargate"},
	}

	t.Log("update dc2 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to update dc2 status to ready")

	t.Log("check that stargate sg1 is created")
	require.Eventually(f.StargateExists(ctx, sg1Key), timeout, interval)

	t.Logf("update stargate sg1 status to ready")
	err = f.SetStargateStatusReady(ctx, sg1Key)
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
	err = f.SetStargateStatusReady(ctx, sg2Key)
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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
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

		condition = FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
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
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, dc2Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, sg1Key, &stargateapi.Stargate{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, sg2Key, &stargateapi.Stargate{}, timeout, interval)
}

// changeNumTokensValue creates a Datacenter and then changes the numTokens value
// Such change is prohibited and should fail
func changeNumTokensValue(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					CassandraConfig: &api.CassandraConfig{
						CassandraYaml: api.CassandraYaml{
							NumTokens: pointer.Int(16),
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       1,
						DatacenterOptions: api.DatacenterOptions{
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
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the datacenter was created")

	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
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

	// Update the datacenter with a different num_tokens value and check that it failed
	initialNumTokens := kc.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml.NumTokens
	kcKey := framework.ClusterKey{NamespacedName: utils.GetKey(kc), K8sContext: f.ControlPlaneContext}
	kcPatch := client.MergeFrom(kc.DeepCopy())
	kc.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml.NumTokens = pointer.Int(256)
	err = f.Patch(ctx, kc, kcPatch, kcKey)
	require.NoError(err, "got error patching num_tokens")

	err = f.Client.Get(ctx, kcKey.NamespacedName, kc)
	require.NoError(err, "failed to get K8ssandraCluster")
	dc := cassdcapi.CassandraDatacenter{}
	err = f.Client.Get(ctx, dcKey.NamespacedName, &dc)
	require.NoError(err, "failed to get CassandraDatacenter")
	dcConfig, err := utils.UnmarshalToMap(dc.Spec.Config)
	require.NoError(err, "failed to unmarshall CassandraDatacenter config")
	dcConfigYaml, _ := dcConfig["cassandra-yaml"].(map[string]interface{})
	t.Logf("Initial num_tokens value: %d", initialNumTokens)
	t.Logf("Spec num_tokens value: %d", kc.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml.NumTokens)
	t.Logf("dcConfigYaml num tokens: %v", dcConfigYaml["num_tokens"].(float64))
	require.NotEqual(fmt.Sprintf("%v", dcConfigYaml["num_tokens"]), fmt.Sprintf("%d", kc.Spec.Cassandra.DatacenterOptions.CassandraConfig.CassandraYaml.NumTokens), "num_tokens should not be updated")
	require.Equal(fmt.Sprintf("%v", dcConfigYaml["num_tokens"]), fmt.Sprintf("%d", initialNumTokens), "num_tokens should not be updated")

	// Test cluster deletion
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

// Create a cluster with encryption options and Stargate.
// Verify that volumes, mounts and config maps are correctly created.
func applyClusterWithEncryptionOptions(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

	clusterName := "cluster-with-encryption"
	serverVersion := "4.0.0"
	dc1Size := int32(3)

	// Create the client keystore and truststore secrets
	clientKeystore := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-keystore-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"keystore":          []byte("keystore content"),
			"keystore-password": []byte("keystore password"),
		},
	}

	clientTruststore := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-truststore-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"truststore":          []byte("truststore content"),
			"truststore-password": []byte("truststore password"),
		},
	}

	// Create the server keystore and truststore configmaps
	serverKeystore := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "server-keystore-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"keystore":          []byte("keystore content"),
			"keystore-password": []byte("keystore password"),
		},
	}

	serverTruststore := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "server-truststore-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"truststore":          []byte("truststore content"),
			"truststore-password": []byte("truststore password"),
		},
	}

	// Create the client keystore and truststore secrets
	clientCertificates := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-certificates",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"rootca.crt": []byte("Root CA content"),
			"client.crt": []byte("Client certificate content"),
			"client.key": []byte("Client key content"),
		},
	}

	// Loop over the secrets and create them
	for _, secret := range []*corev1.Secret{clientKeystore, clientTruststore, serverKeystore, serverTruststore, clientCertificates} {
		secretKey := utils.GetKey(secret)
		secretClusterKey0 := framework.ClusterKey{NamespacedName: secretKey, K8sContext: f.DataPlaneContexts[0]}
		f.Create(ctx, secretClusterKey0, secret)
	}

	// Create the cluster template with encryption enabled
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: serverVersion,
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       dc1Size,
						DatacenterOptions: api.DatacenterOptions{
							CassandraConfig: &api.CassandraConfig{
								CassandraYaml: api.CassandraYaml{
									ClientEncryptionOptions: &encryption.ClientEncryptionOptions{
										Enabled: true,
									},
									ServerEncryptionOptions: &encryption.ServerEncryptionOptions{
										InternodeEncryption: "all",
									},
								},
							},
						},
						Stargate: &stargateapi.StargateDatacenterTemplate{
							StargateClusterTemplate: stargateapi.StargateClusterTemplate{
								Size: 1,
							},
						},
					},
				},
				ServerEncryptionStores: &encryption.Stores{
					KeystoreSecretRef: corev1.LocalObjectReference{
						Name: "server-keystore-secret",
					},
					TruststoreSecretRef: corev1.LocalObjectReference{
						Name: "server-truststore-secret",
					},
				},
				ClientEncryptionStores: &encryption.Stores{
					KeystoreSecretRef: corev1.LocalObjectReference{
						Name: "client-keystore-secret",
					},
					TruststoreSecretRef: corev1.LocalObjectReference{
						Name: "client-truststore-secret",
					},
				},
			},
			Medusa: &medusaapi.MedusaClusterTemplate{
				ContainerImage: &images.Image{
					Repository: medusaImageRepo,
				},
				StorageProperties: medusaapi.Storage{
					StorageSecretRef: corev1.LocalObjectReference{
						Name: cassandraUserSecret,
					},
				},
				CassandraUserSecretRef: corev1.LocalObjectReference{
					Name: cassandraUserSecret,
				},
				CertificatesSecretRef: corev1.LocalObjectReference{
					Name: "client-certificates",
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify configuration of dc1")
	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	require.NoError(err, "failed to get dc1")
	cassContainerIdx, foundCassandra := cassandra.FindContainer(dc1.Spec.PodTemplateSpec, "cassandra")
	require.True(foundCassandra, "failed to find cassandra container in dc1")
	cassContainer := dc1.Spec.PodTemplateSpec.Spec.Containers[cassContainerIdx]
	// Get the cassandra container's volume mounts
	var clientKeystoreMount *corev1.VolumeMount
	var clientTruststoreMount *corev1.VolumeMount
	var serverKeystoreMount *corev1.VolumeMount
	var serverTruststoreMount *corev1.VolumeMount
	for _, mount := range cassContainer.VolumeMounts {
		if mount.Name == "client-keystore" {
			clientKeystoreMount = &mount
			assert.Equal("/mnt/client-keystore", clientKeystoreMount.MountPath, "client-keystore isn't mounted correctly")
		} else if mount.Name == "client-truststore" {
			clientTruststoreMount = &mount
			assert.Equal("/mnt/client-truststore", clientTruststoreMount.MountPath, "client-truststore isn't mounted correctly")
		} else if mount.Name == "server-keystore" {
			serverKeystoreMount = &mount
			assert.Equal("/mnt/server-keystore", serverKeystoreMount.MountPath, "server-keystore isn't mounted correctly")
		} else if mount.Name == "server-truststore" {
			serverTruststoreMount = &mount
			assert.Equal("/mnt/server-truststore", serverTruststoreMount.MountPath, "server-truststore isn't mounted correctly")
		}
	}

	assert.NotNil(clientKeystoreMount, "client-keystore volume mount not found")
	assert.NotNil(clientTruststoreMount, "client-truststore volume mount not found")
	assert.NotNil(serverKeystoreMount, "server-keystore volume mount not found")
	assert.NotNil(serverTruststoreMount, "server-truststore volume mount not found")

	_, foundClientKeystore := cassandra.FindVolume(dc1.Spec.PodTemplateSpec, "client-keystore")
	assert.True(foundClientKeystore, "failed to find client-keystore volume in dc1")
	_, foundClientTruststore := cassandra.FindVolume(dc1.Spec.PodTemplateSpec, "client-truststore")
	assert.True(foundClientTruststore, "failed to find client-truststore volume in dc1")
	_, foundServerKeystore := cassandra.FindVolume(dc1.Spec.PodTemplateSpec, "server-keystore")
	assert.True(foundServerKeystore, "failed to find server-keystore volume in dc1")
	_, foundServerTruststore := cassandra.FindVolume(dc1.Spec.PodTemplateSpec, "server-truststore")
	assert.True(foundServerTruststore, "failed to find server-truststore volume in dc1")

	dcConfig, err := utils.UnmarshalToMap(dc1.Spec.Config)
	require.NoError(err, "failed to unmarshal dc1 config")

	cassYaml, foundYaml := dcConfig["cassandra-yaml"].(map[string]interface{})

	assert.True(foundYaml, "failed to find cassandra-yaml in dcConfig")

	serverEncryptionOptions := cassYaml["server_encryption_options"].(map[string]interface{})

	assert.NotEqual("none", serverEncryptionOptions["internode_encryption"].(string), "server_encryption_options is not enabled")

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

	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to set dc1 status ready")

	sg1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: fmt.Sprintf("%s-dc1-stargate", dc1.Spec.ClusterName)}, K8sContext: f.DataPlaneContexts[0]}
	t.Log("check that stargate sg1 is created")
	require.Eventually(f.StargateExists(ctx, sg1Key), timeout, interval)

	t.Logf("update stargate sg1 status to ready")
	err = f.SetStargateStatusReady(ctx, sg1Key)
	require.NoError(err, "failed to patch stargate status")

	t.Log("verify configuration of stargate in dc1")
	sg1 := &stargateapi.Stargate{}
	err = f.Get(ctx, sg1Key, sg1)
	require.NoError(err, "failed to get stargate in dc1")

	stargateEncryptionSettings := sg1.Spec.CassandraEncryption
	require.NotNil(stargateEncryptionSettings, "stargate encryption settings are not set")
	t.Logf("stargate encryption settings: %+v", stargateEncryptionSettings)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

// Create a cluster with server and client encryption but client encryption stores missing.
// Verify that dc1 never gets created.
func applyClusterWithEncryptionOptionsFail(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	clusterName := "cluster-with-encryption"
	serverVersion := "4.0.0"
	dc1Size := int32(3)
	dc2Size := int32(3)

	// Create the client keystore and truststore secrets
	clientKeystore := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-keystore-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"keystore": []byte("keystore content"),
		},
	}

	clientTruststore := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-truststore-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"truststore": []byte("truststore content"),
		},
	}

	// Create the client keystore and truststore secrets
	clientKeystoreSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-keystore-password-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"keystore-password": []byte("keystore password"),
		},
	}

	clientTruststoreSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "client-truststore-password-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"truststore-password": []byte("truststore password"),
		},
	}

	// Loop over the created configmaps and create them
	for _, secret := range []*corev1.Secret{clientKeystore, clientTruststore, clientKeystoreSecret, clientTruststoreSecret} {
		secretKey := utils.GetKey(secret)
		secretClusterKey0 := framework.ClusterKey{NamespacedName: secretKey, K8sContext: f.DataPlaneContexts[0]}
		secretClusterKey1 := framework.ClusterKey{NamespacedName: secretKey, K8sContext: f.DataPlaneContexts[1]}
		f.Create(ctx, secretClusterKey0, secret)
		f.Create(ctx, secretClusterKey1, secret)
	}

	// Create the cluster template with encryption enabled for both server and client, but missing client encryption stores
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: serverVersion,
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
					CassandraConfig: &api.CassandraConfig{
						CassandraYaml: api.CassandraYaml{
							ClientEncryptionOptions: &encryption.ClientEncryptionOptions{
								Enabled: true,
							},
							ServerEncryptionOptions: &encryption.ServerEncryptionOptions{
								InternodeEncryption: "all",
							},
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       dc1Size,
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       dc2Size,
					},
				},
				ServerEncryptionStores: &encryption.Stores{
					KeystoreSecretRef: corev1.LocalObjectReference{
						Name: "server-keystore-secret",
					},
					TruststoreSecretRef: corev1.LocalObjectReference{
						Name: "server-truststore-secret",
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was never created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Never(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

func verifySuperuserSecretCreated(ctx context.Context, t *testing.T, f *framework.Framework, kluster *api.K8ssandraCluster) {
	t.Logf("check that the default superuser secret is created")
	assert.Eventually(t, superuserSecretExists(f, ctx, kluster), timeout, interval, "failed to verify that the default superuser secret was created")
}

func superuserSecretExists(f *framework.Framework, ctx context.Context, kluster *api.K8ssandraCluster) func() bool {
	secretName := kluster.Spec.Cassandra.SuperuserSecretRef.Name
	if secretName == "" {
		secretName = secret.DefaultSuperuserSecretName(kluster.SanitizedName())
	}
	return secretExists(f, ctx, kluster.Namespace, secretName)
}

func verifySecretCreated(ctx context.Context, t *testing.T, f *framework.Framework, namespace, secretName string) {
	t.Logf("check that the default superuser secret is created")
	assert.Eventually(t, secretExists(f, ctx, namespace, secretName), timeout, interval, "failed to verify that the secret %s was created", secretName)
}

func verifySecretNotCreated(ctx context.Context, t *testing.T, f *framework.Framework, namespace, secretName string) {
	t.Logf("check that the default superuser secret is created")
	assert.Never(t, secretExists(f, ctx, namespace, secretName), timeout, interval, "failed to verify that the secret %s was not created", secretName)
}

func secretExists(f *framework.Framework, ctx context.Context, namespace, secretName string) func() bool {
	return func() bool {
		s := &corev1.Secret{}
		if err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, s); err != nil {
			return false
		}
		return true
	}
}

func verifySystemReplicationAnnotationSet(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster) {
	t.Logf("check that the %s annotation is set", api.InitialSystemReplicationAnnotation)
	assert.Eventually(t, systemReplicationAnnotationIsSet(t, f, ctx, kc), timeout, interval, "Failed to verify that the system replication annotation was set correctly")
}

func systemReplicationAnnotationIsSet(t *testing.T, f *framework.Framework, ctx context.Context, kc *api.K8ssandraCluster) func() bool {
	return func() bool {
		key := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
		expectedReplication := cassandra.ComputeReplicationFromDatacenters(3, kc.Spec.ExternalDatacenters, kc.Spec.Cassandra.Datacenters...)
		kc = &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, key, kc); err != nil {
			t.Logf("Failed to check system replication annotation. Could not retrieve the K8ssandraCluster: %v", err)
			return false
		}

		val, found := kc.Annotations[api.InitialSystemReplicationAnnotation]
		if !found {
			return false
		}

		actualReplication := make(map[string]int)
		if err := json.Unmarshal([]byte(val), &actualReplication); err != nil {
			t.Logf("Failed to unmarshal system replication annotation: %v", err)
			return false
		}

		return reflect.DeepEqual(expectedReplication, actualReplication)
	}
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

func FindDatacenterCondition(status *cassdcapi.CassandraDatacenterStatus, condType cassdcapi.DatacenterConditionType) *cassdcapi.DatacenterCondition {
	for _, condition := range status.Conditions {
		if condition.Type == condType {
			return &condition
		}
	}
	return nil
}

func parseQuantity(quantity string) *resource.Quantity {
	parsed := resource.MustParse(quantity)
	return &parsed
}

// convertSystemReplicationAnnotation creates a K8ssandraCluster object with the system replication annotation set in the old format
func convertSystemReplicationAnnotation(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
			Annotations: map[string]string{
				api.InitialSystemReplicationAnnotation: `{"datacenters":["dc1"], "replicationFactor":3}`,
			},
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       1,
						DatacenterOptions: api.DatacenterOptions{
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
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that the datacenter was created")
	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[1]}
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

	kcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}
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

		initialSystemRfAnnotation, found := kc.Annotations[api.InitialSystemReplicationAnnotation]
		if !found {
			t.Logf("annotation %s not found", api.InitialSystemReplicationAnnotation)
			return false
		}

		if initialSystemRfAnnotation != "{\"dc1\":3}" {
			t.Logf("annotation %s has wrong value: %s", api.InitialSystemReplicationAnnotation, initialSystemRfAnnotation)
			return false
		}

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
		return condition != nil
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	// Test cluster deletion
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

// Create a cluster with server and client encryption but client encryption stores missing.
// Verify that dc1 never gets created.
func changeClusterNameFails(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	clusterName := "cluster-with-encryption"
	newClusterName := "cluster-with-encryption-new"
	serverVersion := "4.0.0"
	dc1Size := int32(3)

	// Create the cluster template with encryption enabled for both server and client, but missing client encryption stores
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: serverVersion,
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       dc1Size,
					},
				},
				ServerEncryptionStores: &encryption.Stores{
					KeystoreSecretRef: corev1.LocalObjectReference{
						Name: "server-keystore-secret",
					},
					TruststoreSecretRef: corev1.LocalObjectReference{
						Name: "server-truststore-secret",
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was never created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	k8c := &api.K8ssandraCluster{}
	err = f.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, k8c)
	require.NoError(err, "failed to get K8ssandraCluster")

	// Change the cluster name
	k8c.Spec.Cassandra.ClusterName = newClusterName
	err = f.Client.Update(ctx, k8c)
	require.Error(err, "failed to update K8ssandraCluster")

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

// Create a cluster with both volumes and additional volumes being injected and check that the volumes are created in the podTemplateSpec.
func injectContainersAndVolumes(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	clusterName := "cluster-with-injection"
	serverVersion := "4.0.0"
	dc1Size := int32(3)

	// Create the cluster template with encryption enabled for both server and client, but missing client encryption stores
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Medusa: &medusaapi.MedusaClusterTemplate{
				StorageProperties: medusaapi.Storage{
					StorageProvider: "local",
				},
			},
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: serverVersion,
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "injected-container",
							Image: "busybox",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "injected-volume",
									MountPath: "/injected-volume",
								},
							},
						},
					},
					ExtraVolumes: &api.K8ssandraVolumes{
						PVCs: []cassdcapi.AdditionalVolumes{
							{
								Name:      "injected-volume",
								MountPath: "/etc/injected",
							},
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[0],
						Size:       dc1Size,
						DatacenterOptions: api.DatacenterOptions{
							InitContainers: []corev1.Container{
								{
									Name: "server-config-init",
								},
								{
									Name: "medusa-restore",
								},
								{
									Name:  "injected-init-container",
									Image: "busybox",
								},
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

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that dc1 was never created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	dc := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc)
	require.NoError(err, "failed to get CassandraDatacenter dc1")

	posCfgInit, foundCfgInit := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "server-config-init")
	require.True(foundCfgInit, "failed to find server-config-init init container")
	require.Equal(0, posCfgInit, "server-config-init should be the first init container")

	posMedusaInit, foundMedusaInit := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "medusa-restore")
	require.True(foundMedusaInit, "failed to find medusa-restore init container")
	require.Equal(1, posMedusaInit, "medusa-restore should be the second init container")

	posInit, foundInit := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "injected-init-container")
	require.True(foundInit, "failed to find injected-init-container")
	require.Equal(2, posInit, "injected-init-container should be the third init container")

	posJmxInit, foundJmxInit := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "jmx-credentials")
	require.True(foundJmxInit, "failed to find jmx-credentials init container")
	require.Equal(3, posJmxInit, "jmx-credentials should be the fourth init container")

	_, foundMain := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "injected-container")
	require.True(foundMain, "failed to find injected-container")

	require.Equal(2, len(dc.Spec.StorageConfig.AdditionalVolumes), "expected 1 additional volume")
	require.Equal("/etc/injected", dc.Spec.StorageConfig.AdditionalVolumes[0].MountPath, "expected injected-volume mount path")

	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}
