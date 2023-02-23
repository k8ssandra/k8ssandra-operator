package e2e

import (
	"context"
	ctaskapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	ktaskapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func createMultiDatacenterTask(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	kc := &api.K8ssandraCluster{}
	kcName := "test"
	kcKey := client.ObjectKey{Namespace: namespace, Name: kcName}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dc1Key, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dc1Key.Name)

	dc2Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[1], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	checkDatacenterReady(t, ctx, dc2Key, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dc1Key.Name, dc2Key.Name)

	t.Log("create a K8ssandraTask")
	kTask := &ktaskapi.K8ssandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "restart-task",
		},
		Spec: ktaskapi.K8ssandraTaskSpec{
			Cluster: corev1.ObjectReference{Name: kcName},
			Template: ctaskapi.CassandraTaskTemplate{
				Jobs: []ctaskapi.CassandraJob{{
					Name:    "restart-job",
					Command: "restart",
				}},
			},
		},
	}
	require.NoError(f.Client.Create(ctx, kTask))

	t.Log("check that CassandraTask 1 was created")
	cTask1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "restart-task-dc1")
	cTask1 := &ctaskapi.CassandraTask{}
	require.Eventually(func() bool {
		err := f.Get(ctx, cTask1Key, cTask1)
		return err == nil
	}, polling.cassandraTaskCreated.timeout, polling.cassandraTaskCreated.interval)
	require.Equal("restart-job", cTask1.Spec.Jobs[0].Name)
	require.Equal("restart", string(cTask1.Spec.Jobs[0].Command))

	t.Log("check that DC 1 restarted")
	require.Eventually(func() bool {
		return dcRestarted(ctx, cTask1Key, kcName, f, require)
	}, polling.datacenterReady.timeout, polling.datacenterReady.interval)

	t.Log("check that CassandraTask 2 was created")
	cTask2Key := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, "restart-task-dc2")
	cTask2 := &ctaskapi.CassandraTask{}
	require.Eventually(func() bool {
		err := f.Get(ctx, cTask2Key, cTask2)
		return err == nil
	}, polling.cassandraTaskCreated.timeout, polling.cassandraTaskCreated.interval)
	require.Equal("restart-job", cTask2.Spec.Jobs[0].Name)
	require.Equal("restart", string(cTask2.Spec.Jobs[0].Command))

	t.Log("check that DC 2 restarted")
	require.Eventually(func() bool {
		return dcRestarted(ctx, cTask2Key, kcName, f, require)
	}, polling.datacenterReady.timeout, polling.datacenterReady.interval)

	t.Log("check that the K8ssandraTask completes")
	kTaskKey := framework.NewClusterKey(f.ControlPlaneContext, namespace, "restart-task")
	require.Eventually(func() bool {
		require.NoError(f.Get(ctx, kTaskKey, kTask))
		return !kTask.Status.CompletionTime.IsZero() &&
			kTask.Status.Active == 0 &&
			kTask.Status.Succeeded == 2
	}, polling.datacenterReady.timeout, polling.datacenterReady.interval)

	t.Log("delete dc1")
	require.NoError(f.Client.Get(ctx, kcKey, kc))
	kc.Spec.Cassandra.Datacenters = kc.Spec.Cassandra.Datacenters[1:]
	require.NoError(f.Client.Update(ctx, kc))

	t.Log("check that CassandraTask 1 was deleted")
	require.Eventually(func() bool {
		return !f.CassTaskExists(ctx, cTask1Key)()
	}, polling.datacenterReady.timeout, polling.datacenterReady.interval)

	t.Log("delete the K8ssandraCluster")
	require.NoError(f.Client.Delete(ctx, kc))

	t.Log("check that CassandraTask 2 was deleted")
	require.Eventually(func() bool {
		return !f.CassTaskExists(ctx, cTask2Key)()
	}, polling.datacenterReady.timeout, polling.datacenterReady.interval)

	t.Log("check that the K8ssandraTask was deleted")
	require.Eventually(func() bool {
		return !f.K8ssandraTaskExists(ctx, kTaskKey)()
	}, polling.datacenterReady.timeout, polling.datacenterReady.interval)
}

func dcRestarted(
	ctx context.Context,
	cTaskKey framework.ClusterKey,
	kcName string,
	f *framework.E2eFramework,
	require *require.Assertions,
) bool {
	statefulSets := &v1.StatefulSetList{}
	err := f.List(ctx,
		cTaskKey,
		statefulSets,
		client.InNamespace(cTaskKey.Namespace),
		client.MatchingLabels{"cassandra.datastax.com/cluster": kcName},
	)
	require.NoError(err)
	for _, statefulSet := range statefulSets.Items {
		_, hasAnnotation := statefulSet.Spec.Template.Annotations["control.k8ssandra.io/restartedAt"]
		if !hasAnnotation {
			return false
		}
	}
	return true
}
