package k8ssandra

import (
	"context"
	"fmt"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cassandra "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	medusaImageRepo     = "test"
	storageSecret       = "storage-secret"
	cassandraUserSecret = "medusa-secret"
)

func createMultiDcClusterWithMedusa(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
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
				ReadinessProbeSettings: &medusaapi.ProbeSettings{
					InitialDelaySeconds: 1,
					TimeoutSeconds:      2,
					PeriodSeconds:       3,
					SuccessThreshold:    1,
					FailureThreshold:    5,
				},
				LivenessProbeSettings: &medusaapi.ProbeSettings{
					InitialDelaySeconds: 6,
					TimeoutSeconds:      7,
					PeriodSeconds:       8,
					SuccessThreshold:    1,
					FailureThreshold:    10,
				},
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("150m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
			},
		},
	}

	t.Log("Creating k8ssandracluster with Medusa")
	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("check that the standalone Medusa deployment was created")
	medusaDeploymentKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-medusa-standalone"}, K8sContext: f.DataPlaneContexts[0]}
	medusaDeployment := &appsv1.Deployment{}
	require.Eventually(func() bool {
		if err := f.Get(ctx, medusaDeploymentKey, medusaDeployment); err != nil {
			return false
		}
		return true
	}, timeout, interval)

	require.Equal(int32(kc.Spec.Medusa.ReadinessProbeSettings.FailureThreshold), medusaDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold)
	require.Equal(int32(kc.Spec.Medusa.ReadinessProbeSettings.InitialDelaySeconds), medusaDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds)
	require.Equal(int32(kc.Spec.Medusa.ReadinessProbeSettings.PeriodSeconds), medusaDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds)
	require.Equal(int32(kc.Spec.Medusa.ReadinessProbeSettings.SuccessThreshold), medusaDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.SuccessThreshold)
	require.Equal(int32(kc.Spec.Medusa.ReadinessProbeSettings.TimeoutSeconds), medusaDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds)

	require.Equal(int32(kc.Spec.Medusa.LivenessProbeSettings.FailureThreshold), medusaDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold)
	require.Equal(int32(kc.Spec.Medusa.LivenessProbeSettings.InitialDelaySeconds), medusaDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds)
	require.Equal(int32(kc.Spec.Medusa.LivenessProbeSettings.PeriodSeconds), medusaDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds)
	require.Equal(int32(kc.Spec.Medusa.LivenessProbeSettings.SuccessThreshold), medusaDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.SuccessThreshold)
	require.Equal(int32(kc.Spec.Medusa.LivenessProbeSettings.TimeoutSeconds), medusaDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds)

	require.Equal(kc.Spec.Medusa.Resources.Limits.Cpu().String(), medusaDeployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String())
	require.Equal(kc.Spec.Medusa.Resources.Limits.Memory().String(), medusaDeployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String())
	require.Equal(kc.Spec.Medusa.Resources.Requests.Cpu().String(), medusaDeployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
	require.Equal(kc.Spec.Medusa.Resources.Requests.Memory().String(), medusaDeployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String())
	require.True(f.ContainerHasEnvVar(medusaDeployment.Spec.Template.Spec.Containers[0], "MEDUSA_RESOLVE_IP_ADDRESSES", "False"))

	t.Log("check that the standalone Medusa service was created")
	medusaServiceKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-medusa-service"}, K8sContext: f.DataPlaneContexts[0]}
	medusaService := &corev1.Service{}
	require.Eventually(func() bool {
		if err := f.Get(ctx, medusaServiceKey, medusaService); err != nil {
			return false
		}
		return true
	}, timeout, interval)

	t.Log("update datacenter status to scaling up")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	kcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}

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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
		return !(condition == nil && condition.Status == corev1.ConditionFalse)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	checkMedusaObjectsCompliance(t, f, dc1, kc)

	t.Log("check that dc2 has not been created yet")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
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

	t.Log("check that remote seeds are set on dc2")
	dc2 = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

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
	checkMedusaObjectsCompliance(t, f, dc2, kc)

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

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			t.Logf("k8ssandracluster status check failed: cassandra in %s is not ready", dc1Key.Name)
			return false
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

		return true
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")
}

// Check that all the Medusa related objects have been created and are in the expected state.
func checkMedusaObjectsCompliance(t *testing.T, f *framework.Framework, dc *cassdcapi.CassandraDatacenter, kc *api.K8ssandraCluster) {
	require := require.New(t)

	// Check containers presence
	initContainerIndex, found := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "medusa-restore")
	require.True(found, fmt.Sprintf("%s doesn't have medusa-restore init container", dc.Name))
	_, foundConfig := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "server-config-init")
	require.True(foundConfig, fmt.Sprintf("%s doesn't have server-config-init container", dc.Name))
	initContainer := dc.Spec.PodTemplateSpec.Spec.InitContainers[initContainerIndex]
	containerIndex, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "medusa")
	require.True(found, fmt.Sprintf("%s doesn't have medusa container", dc.Name))
	mainContainer := dc.Spec.PodTemplateSpec.Spec.Containers[containerIndex]

	for _, container := range [](corev1.Container){initContainer, mainContainer} {
		// Check containers Image
		require.True(container.Image == fmt.Sprintf("docker.io/%s/medusa:latest", medusaImageRepo), fmt.Sprintf("%s %s init container doesn't have the right image %s vs docker.io/%s/medusa:latest", dc.Name, container.Name, container.Image, medusaImageRepo))

		// Check volume mounts
		assert.True(t, f.ContainerHasVolumeMount(container, "server-config", "/etc/cassandra"), "Missing Volume Mount for medusa-restore server-config")
		assert.True(t, f.ContainerHasVolumeMount(container, "server-data", "/var/lib/cassandra"), "Missing Volume Mount for medusa-restore server-data")
		assert.True(t, f.ContainerHasVolumeMount(container, "podinfo", "/etc/podinfo"), "Missing Volume Mount for medusa-restore podinfo")
		assert.True(t, f.ContainerHasVolumeMount(container, cassandraUserSecret, "/etc/medusa-secrets"), "Missing Volume Mount for medusa-restore medusa-secrets")
		assert.True(t, f.ContainerHasVolumeMount(container, fmt.Sprintf("%s-medusa", kc.Name), "/etc/medusa"), "Missing Volume Mount for medusa-restore medusa config")

		// Check env vars
		if container.Name == "medusa" {
			assert.True(t, f.ContainerHasEnvVar(container, "MEDUSA_MODE", "GRPC"), "Wrong MEDUSA_MODE env var for medusa")
		} else {
			assert.True(t, f.ContainerHasEnvVar(container, "MEDUSA_MODE", "RESTORE"), "Wrong MEDUSA_MODE env var for medusa-restore")
		}
		assert.True(t, f.ContainerHasEnvVar(container, "MEDUSA_CQL_USERNAME", ""), "Missing MEDUSA_CQL_USERNAME env var for medusa-restore")
		assert.True(t, f.ContainerHasEnvVar(container, "MEDUSA_CQL_PASSWORD", ""), "Missing MEDUSA_CQL_PASSWORD env var for medusa-restore")
	}
}
