package medusa

import (
	"context"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func testInPlaceRestore(t *testing.T, namespace string, testClient client.Client) {
	require := require.New(t)

	ctx := context.Background()

	restore := &api.CassandraRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-restore",
		},
		Spec: api.CassandraRestoreSpec{
			Backup:   "test-backup",
			Shutdown: true,
			InPlace:  true,
			CassandraDatacenter: api.CassandraDatacenterConfig{
				Name:        TestCassandraDatacenterName,
				ClusterName: "test-dc",
			},
		},
	}

	restoreKey := types.NamespacedName{Namespace: restore.Namespace, Name: restore.Name}

	err := testClient.Create(ctx, restore)
	require.NoError(err, "failed to create CassandraRestore")

	dcKey := types.NamespacedName{Namespace: "default", Name: TestCassandraDatacenterName}

	withDc := newWithDatacenter(t, ctx, dcKey, testClient)

	t.Log("check that the datacenter is set to be stopped")
	require.Eventually(withDc(func(dc *cassdcapi.CassandraDatacenter) bool {
		return dc.Spec.Stopped == true
	}), timeout, interval, "timed out waiting for CassandraDatacenter stopped flag to be set")

	t.Log("delete datacenter pods to simulate shutdown")
	err = testClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("default"), client.MatchingLabels{cassdcapi.DatacenterLabel: TestCassandraDatacenterName})
	require.NoError(err, "failed to delete datacenter pods")

	restore = &api.CassandraRestore{}
	err = testClient.Get(ctx, restoreKey, restore)
	require.NoError(err, "failed to get CassandraRestore")

	dcStoppedTime := restore.Status.StartTime.Time.Add(1 * time.Second)

	t.Log("set datacenter status to stopped")
	err = patchDatacenterStatus(ctx, dcKey, testClient, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterStopped,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime),
		})
	})
	require.NoError(err, "failed to update datacenter status with stopped condition")

	t.Log("check that the datacenter podTemplateSpec is updated")
	require.Eventually(withDc(func(dc *cassdcapi.CassandraDatacenter) bool {
		restoreContainer := findContainer(dc.Spec.PodTemplateSpec.Spec.InitContainers, "medusa-restore")
		if restoreContainer == nil {
			return false
		}

		envVar := findEnvVar(restoreContainer.Env, "BACKUP_NAME")
		if envVar == nil || envVar.Value != "test-backup" {
			return false
		}

		envVar = findEnvVar(restoreContainer.Env, "RESTORE_KEY")
		return envVar != nil
	}), timeout, interval, "timed out waiting for CassandraDatacenter PodTemplateSpec update")

	restore = &api.CassandraRestore{}
	err = testClient.Get(ctx, restoreKey, restore)
	require.NoError(err, "failed to get CassandraRestore")

	// In addition to checking Updating condition, the restore controller also checks the
	// PodTemplateSpec of the StatefulSets to make sure the update has been pushed down.
	// Note that this test does **not** verify the StatefulSet check. cass-operator creates
	// the StatefulSets. While we could create the StatefulSets in this test, it will be
	// easier/better to verify the StatefulSet checks in unit and e2e tests.
	t.Log("set datacenter status to updated")
	err = patchDatacenterStatus(ctx, dcKey, testClient, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterUpdating,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(restore.Status.DatacenterStopped.Add(time.Second * 1)),
		})
	})
	require.NoError(err, "failed to update datacenter status with updating condition")

	dc := &cassdcapi.CassandraDatacenter{}
	err = testClient.Get(ctx, dcKey, dc)
	require.NoError(err)

	restore = &api.CassandraRestore{}
	err = testClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-restore"}, restore)
	require.NoError(err)

	t.Log("check datacenter restarted")
	require.Eventually(withDc(func(dc *cassdcapi.CassandraDatacenter) bool {
		return !dc.Spec.Stopped
	}), timeout, interval)

	t.Log("set datacenter status to ready")
	err = patchDatacenterStatus(ctx, dcKey, testClient, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(dcStoppedTime.Add(time.Second * 2)),
		})
	})

	require.NoError(err)

	t.Log("check restore status finish time set")
	require.Eventually(func() bool {
		restore := &api.CassandraRestore{}
		err := testClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-restore"}, restore)
		if err != nil {
			return false
		}

		return !restore.Status.FinishTime.IsZero()
	}, timeout, interval)
}

// newWithDatacenter is a function generator for withDatacenter that is bound to t, ctx, and key.
func newWithDatacenter(t *testing.T, ctx context.Context, key types.NamespacedName, testClient client.Client) func(func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func(condition func(dc *cassdcapi.CassandraDatacenter) bool) func() bool {
		return withDatacenter(t, ctx, key, testClient, condition)
	}
}

// withDatacenter Fetches the CassandraDatacenter specified by key and then calls condition.
func withDatacenter(t *testing.T, ctx context.Context, key types.NamespacedName, testClient client.Client, condition func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func() bool {
		dc := &cassdcapi.CassandraDatacenter{}
		if err := testClient.Get(ctx, key, dc); err == nil {
			return condition(dc)
		} else {
			t.Logf("failed to get CassandraDatacenter: %s", err)
			return false
		}
	}
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for _, container := range containers {
		if container.Name == name {
			return &container
		}
	}
	return nil
}

func findEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return &envVar
		}
	}
	return nil
}

func patchDatacenterStatus(ctx context.Context, key types.NamespacedName, testClient client.Client, updateFn func(dc *cassdcapi.CassandraDatacenter)) error {
	dc := &cassdcapi.CassandraDatacenter{}
	err := testClient.Get(ctx, key, dc)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(dc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(dc)

	return testClient.Status().Patch(ctx, dc, patch)
}

//func updateStatefulSet
