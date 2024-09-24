package k8ssandra

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createSingleDcCluster verifies that the CassandraDatacenter is created and that the
// expected status updates happen on the K8ssandraCluster.
func createSingleDcClusterWithMetricsAgent(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
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
						K8sContext: f.DataPlaneContexts[0],
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
	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dcKey), timeout, interval)

	t.Log("update datacenter status to ready")
	kcKey := framework.NewClusterKey(f.ControlPlaneContext, namespace, kc.Name)
	err = f.PatchDatacenterStatus(ctx, dcKey, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to update datacenter status to ready")

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

		condition := findDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		return condition != nil && condition.Status == corev1.ConditionTrue
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	require.Eventually(func() bool {
		return f.UpdateDatacenterGeneration(ctx, t, dcKey)
	}, timeout, interval, "failed to update dc1 generation")

	// Check that we have the right volumes and volume mounts.
	dc := &cassdcapi.CassandraDatacenter{}
	if err := f.Get(ctx, dcKey, dc); err != nil {
		require.Fail("could not find dc")
	}

	// check that we have the right ConfigMap
	agentCmKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Name: "test-dc1-metrics-agent-config", Namespace: namespace}, K8sContext: f.DataPlaneContexts[0]}
	agentCm := corev1.ConfigMap{}
	require.Eventually(func() bool {
		if err := f.Get(ctx, agentCmKey, &agentCm); err != nil {
			t.Log("could not find expected metrics-agent-config configmap")
			return false
		}
		return f.IsOwnedByCassandraDatacenter(&agentCm)
	}, timeout, interval)

	// Verify the ConfigMap is set to be mounted
	require.True(len(dc.Spec.StorageConfig.AdditionalVolumes) > 0)

	mapMounted := false
	for _, additionalVolume := range dc.Spec.StorageConfig.AdditionalVolumes {
		if additionalVolume.Name == "metrics-agent-config" {
			require.NotNil(additionalVolume.VolumeSource.ConfigMap)
			require.Equal(agentCm.GetName(), additionalVolume.VolumeSource.ConfigMap.Name)
			mapMounted = true
		}
	}
	require.True(mapMounted)

	// Test cluster deletion, ensuring configmap deleted too.
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
}

func findDatacenterCondition(status *cassdcapi.CassandraDatacenterStatus, condType cassdcapi.DatacenterConditionType) *cassdcapi.DatacenterCondition {
	for _, condition := range status.Conditions {
		if condition.Type == condType {
			return &condition
		}
	}
	return nil
}
