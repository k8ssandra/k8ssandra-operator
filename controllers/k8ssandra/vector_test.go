package k8ssandra

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createSingleDcCluster verifies that the CassandraDatacenter is created and that the
// expected status updates happen on the K8ssandraCluster.
func createSingleDcClusterWithVector(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Telemetry: &telemetryapi.TelemetrySpec{
						Vector: &telemetryapi.VectorSpec{
							Enabled: ptr.To(true),
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: f.DataPlaneContexts[1],
						Size:       1,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "3.11.14",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
									StorageClassName: &defaultStorageClass,
								},
							},
							PodSecurityContext: &corev1.PodSecurityContext{
								RunAsUser: ptr.To[int64](999),
							},
							ManagementApiAuth: &cassdcapi.ManagementApiAuthConfig{
								Insecure: &cassdcapi.ManagementApiAuthInsecureConfig{},
							},
						},
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifyFinalizerAdded(ctx, t, f, kc)

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the datacenter was created")
	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[1]}
	require.Eventually(f.DatacenterExists(ctx, dcKey), timeout, interval)

	t.Log("check the pod SecurityContext was set in the CassandraDatacenter")
	dc := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dcKey, dc)
	require.NoError(err, "failed to get CassandraDatacenter")

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

	// Test that prometheus servicemonitor comes up when it is requested in the CassandraDatacenter.
	kcPatch := client.MergeFrom(kc.DeepCopy())
	kc.Spec.Cassandra.Datacenters[0].DatacenterOptions.Telemetry = &telemetryapi.TelemetrySpec{
		Prometheus: &telemetryapi.PrometheusTelemetrySpec{
			Enabled: ptr.To(true),
		},
	}
	if err := f.Patch(ctx, kc, kcPatch, kcKey); err != nil {
		assert.Fail(t, "got error patching for telemetry", "error", err)
	}

	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, "dc1")

	if err = f.SetDatacenterStatusReady(ctx, dc1Key); err != nil {
		assert.Fail(t, "error setting status ready", err)
	}

	// Check that the Vector config map was created
	vectorConfigMapKey := types.NamespacedName{Namespace: namespace, Name: telemetry.VectorAgentConfigMapName(kc.Name, dc1Key.Name)}
	vectorConfigMap := &corev1.ConfigMap{}
	require.Eventually(func() bool {
		err = f.Get(ctx, framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: vectorConfigMapKey}, vectorConfigMap)
		if err != nil {
			t.Logf("failed to get Vector config map: %v", err)
			return false
		}
		return true
	}, timeout, interval, "timed out waiting for Vector config map")

	// Check that Vector configuration was set to the SystemLoggerResources
	loggerResources := dc.Spec.SystemLoggerResources
	_, foundCPURequest := loggerResources.Requests[corev1.ResourceCPU]
	require.True(foundCPURequest)

	// Check that the AdditionalVolumes has VolumeSource
	foundVectorConfig := false
	for _, vol := range dc.Spec.StorageConfig.AdditionalVolumes {
		if vol.Name == "vector-config" {
			foundVectorConfig = true
			require.Equal("/etc/vector", vol.MountPath)
			require.NotNil(vol.VolumeSource)
		}
	}
	require.True(foundVectorConfig)

	// Test cluster deletion
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: vectorConfigMapKey}, &corev1.ConfigMap{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}
