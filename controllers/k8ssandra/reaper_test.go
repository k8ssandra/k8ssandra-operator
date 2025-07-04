package k8ssandra

import (
	"context"
	"fmt"
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/resource"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createMultiDcClusterWithReaper(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: newTwoDcCassandraClusterTemplate(f),
			Reaper: &reaperapi.ReaperClusterTemplate{
				ReaperTemplate: reaperapi.ReaperTemplate{
					AutoScheduling: reaperapi.AutoScheduling{Enabled: true},
				},
			},
		},
	}
	kc.Spec.Cassandra.Meta.CommonLabels = map[string]string{"testLabel": "testValue"}
	kc.Spec.Cassandra.Meta.CommonAnnotations = map[string]string{"testAnnotation": "testValue"}

	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifySuperuserSecretCreated(ctx, t, f, kc)

	verifyReplicatedSecretReconciled(ctx, t, f, kc)

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
		return condition != nil && condition.Status == corev1.ConditionTrue
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	reaper1Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[0],
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      kc.Name + "-" + dc1Key.Name + "-reaper"},
	}

	t.Logf("check that reaper %s has not been created", reaper1Key)
	reaper1 := &reaperapi.Reaper{}
	err = f.Get(ctx, reaper1Key, reaper1)
	require.True(err != nil && errors.IsNotFound(err), fmt.Sprintf("reaper %s should not be created until dc1 is ready", reaper1Key))

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

	t.Log("check that remote seeds are set on dc2")
	dc2 = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	reaper2Key := framework.ClusterKey{
		K8sContext: f.DataPlaneContexts[1],
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      kc.Name + "-" + dc2Key.Name + "-reaper"},
	}

	t.Log("update dc2 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to update dc2 status to ready")

	t.Log("check that dc2 was rebuilt")
	verifyRebuildTaskCreated(ctx, t, f, dc2Key, dc1Key)
	rebuildTaskKey := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, "dc2-rebuild")
	setRebuildTaskFinished(ctx, t, f, rebuildTaskKey, dc2Key)

	t.Log("check that reaper reaper1 is created")
	require.Eventually(f.ReaperExists(ctx, reaper1Key), timeout, interval)
	verifyReaperSecretAnnotationAdded(t, f, ctx, reaper1Key, reaper.DefaultUserSecretName(kc.SanitizedName()))

	t.Logf("update reaper reaper1 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper1Key)
	require.NoError(err, "failed to patch reaper status")

	t.Log("check that reaper reaper2 is created")
	require.Eventually(f.ReaperExists(ctx, reaper2Key), timeout, interval)
	verifyReaperSecretAnnotationAdded(t, f, ctx, reaper2Key, reaper.DefaultUserSecretName(kc.SanitizedName()))

	t.Logf("update reaper reaper2 status to ready")
	err = f.SetReaperStatusReady(ctx, reaper2Key)
	require.NoError(err, "failed to patch reaper status")

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

		if k8ssandraStatus.Reaper == nil || !k8ssandraStatus.Reaper.IsReady() {
			t.Logf("k8ssandracluster status check failed: reaper in %s is not ready", dc1Key.Name)
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

		if k8ssandraStatus.Reaper == nil || !k8ssandraStatus.Reaper.IsReady() {
			t.Logf("k8ssandracluster status check failed: reaper in %s is not ready", dc2Key.Name)
			return false
		}

		return true
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	t.Log("remove both reapers from kc spec")
	err = f.Get(ctx, kcKey, kc)
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Reaper = nil
	err = f.Client.Patch(ctx, kc, patch)
	require.NoError(err, "failed to update K8ssandraCluster")

	require.Eventually(func() bool {
		return f.UpdateDatacenterGeneration(ctx, t, dc1Key)
	}, timeout, interval, "failed to update dc1 generation")

	require.Eventually(func() bool {
		return f.UpdateDatacenterGeneration(ctx, t, dc2Key)
	}, timeout, interval, "failed to update dc2 generation")

	t.Log("check that reaper reaper1 is deleted")
	require.Eventually(func() bool {
		err = f.Get(ctx, reaper1Key, &reaperapi.Reaper{})
		return errors.IsNotFound(err)
	}, timeout, interval)

	t.Log("check that reaper reaper2 is deleted")
	require.Eventually(func() bool {
		err = f.Get(ctx, reaper2Key, &reaperapi.Reaper{})
		return errors.IsNotFound(err)
	}, timeout, interval)

	t.Log("check that kc status is updated")
	require.Eventually(func() bool {
		err = f.Get(ctx, kcKey, kc)
		require.NoError(err, "failed to get K8ssandraCluster")
		return kc.Status.Datacenters[dc1Key.Name].Reaper == nil &&
			kc.Status.Datacenters[dc2Key.Name].Reaper == nil
	}, timeout, interval)

}

func createMultiDcClusterWithControlPlaneReaper(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	cpr := &reaperapi.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "reaper",
		},
		Spec: newControlPlaneReaper(),
	}

	err := f.Client.Create(ctx, cpr)
	require.NoError(err, "failed to create control plane reaper")

	cpReaperKey := framework.ClusterKey{
		K8sContext: f.ControlPlaneContext,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      "reaper"},
	}
	t.Log("check that control plane reaper is created")
	require.Eventually(f.ReaperExists(ctx, cpReaperKey), timeout, interval)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: newTwoDcCassandraClusterTemplate(f),
			Reaper: &reaperapi.ReaperClusterTemplate{
				ReaperRef: corev1.ObjectReference{
					Name:      cpr.Name,
					Namespace: cpr.Namespace,
				},
			},
		},
	}

	err = f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifySuperuserSecretCreated(ctx, t, f, kc)
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

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

	t.Log("check that remote seeds are set on dc2")
	dc2 = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	t.Log("update dc2 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(err, "failed to update dc2 status to ready")

	// check that reapers were not created together with cass DCs
	verifyReaperAbsent(t, f, ctx, kc, f.DataPlaneContexts[0], dc1Key, namespace)
	verifyReaperAbsent(t, f, ctx, kc, f.DataPlaneContexts[1], dc2Key, namespace)

	// check the kc is added to reaper
	verifyClusterRegistered(t, f, ctx, kc, namespace)

	err = f.DeleteK8ssandraCluster(ctx, utils.GetKey(kc), timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
}

func getReaperAnnotations(t *testing.T, f *framework.Framework, ctx context.Context, key framework.ClusterKey) map[string]string {
	reaper := &reaperapi.Reaper{}
	err := f.Get(ctx, key, reaper)
	if err != nil {
		t.Logf("Failed to get Reaper: %v", err)
	}

	return reaper.Spec.ResourceMeta.Pods.Annotations
}

func verifyReaperSecretAnnotationAdded(t *testing.T, f *framework.Framework, ctx context.Context, dcKey framework.ClusterKey, secretName string) {
	t.Logf("check that the superuser secret annotation is added")
	assert.Eventually(t, secretAnnotationAdded(t, f, ctx, dcKey, getReaperAnnotations, secretName), timeout, interval, " failed to verify reaper secret annotation added")
}

func newTwoDcCassandraClusterTemplate(f *framework.Framework) *api.CassandraClusterTemplate {
	return &api.CassandraClusterTemplate{
		Datacenters: []api.CassandraDatacenterTemplate{
			{
				Meta: api.EmbeddedObjectMeta{
					Name: "dc1",
				},
				K8sContext: f.DataPlaneContexts[0],
				Size:       3,
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "3.11.14",
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
					ServerVersion: "3.11.14",
					StorageConfig: &cassdcapi.StorageConfig{
						CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
							StorageClassName: &defaultStorageClass,
						},
					},
				},
			},
		},
	}
}

func newControlPlaneReaper() reaperapi.ReaperSpec {
	return reaperapi.ReaperSpec{
		ReaperTemplate: reaperapi.ReaperTemplate{
			StorageType: reaperapi.StorageTypeLocal,
			StorageConfig: &corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
			AutoScheduling: reaperapi.AutoScheduling{
				Enabled: true,
				// we need to be explicit to prevent checking the cass-dc spec, which is not available if reaper's in control plane mode
				RepairType: "ADAPTIVE",
			},
		},
	}
}

func verifyReaperAbsent(t *testing.T, f *framework.Framework, ctx context.Context, kc *api.K8ssandraCluster, dataPlaneContext string, dcKey framework.ClusterKey, namespace string) {
	reaperKey := framework.ClusterKey{
		K8sContext: dataPlaneContext,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      kc.Name + "-" + dcKey.Name + "-reaper"},
	}

	t.Logf("check that reaper %s has not been created", reaperKey)
	reaper := &reaperapi.Reaper{}
	err := f.Get(ctx, reaperKey, reaper)
	require.True(t, err != nil && errors.IsNotFound(err), fmt.Sprintf("reaper %s should not be created in dc %s", reaperKey, dcKey))
}

func verifyClusterRegistered(t *testing.T, f *framework.Framework, ctx context.Context, kc *api.K8ssandraCluster, namespace string) {

}
