package k8ssandra

import (
	"context"
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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
							Enabled: pointer.Bool(true),
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
							PodSecurityContext: &corev1.PodSecurityContext{
								RunAsUser: pointer.Int64(999),
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

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the datacenter was created")
	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dcKey), timeout, interval)
	// Check that we have the right volumes and volume mounts.
	dc := &cassdcapi.CassandraDatacenter{}
	f.Get(ctx, dcKey, dc)
	if err := f.Get(ctx, dcKey, dc); err != nil {
		require.Fail("could not find dc")
	}
	_, found := cassandra.FindVolume(dc.Spec.PodTemplateSpec, "metrics-agent-config")
	if !found {
		require.Fail("could not find expected metrics-agent-config volume")
	}
	cassContainerIdx, _ := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "cassandra")
	volMount := cassandra.FindVolumeMount(&dc.Spec.PodTemplateSpec.Spec.Containers[cassContainerIdx], "metrics-agent-config")
	if volMount == nil {
		require.Fail("could not find expected metrics-agent-config volumeMount")
	}

	// check that we have the right ConfigMap
	agentCmKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Name: "test-dc1" + "-metrics-agent-config", Namespace: namespace}, K8sContext: f.DataPlaneContexts[0]}
	agentCm := corev1.ConfigMap{}
	if err := f.Get(ctx, agentCmKey, &agentCm); err != nil {
		assert.Fail(t, "could not find expected metrics-agent-config configmap")
	}

	// Test cluster deletion, ensuring configmap deleted too.
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dcKey, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t,
		agentCmKey,
		&corev1.ConfigMap{},
		timeout,
		interval)
}
