package controllers

import (
	"context"
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"testing"
	"time"
)

func createSingleDcCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	//assert := assert.New(t)

	k8sCtx := "cluster-1"

	cluster := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test",
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.Cassandra{
				Cluster: "test",
				Datacenters: []api.CassandraDatacenterTemplateSpec{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext:    k8sCtx,
						Size:          1,
						ServerVersion: "3.11.10",
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, cluster)
	require.NoError(err, "failed to create K8ssandraCluster")

	t.Log("check that the datacenter was created")
	dcKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx}
	require.Eventually(f.DatacenterExists(ctx, dcKey), timeout, interval)

	lastTransitionTime := metav1.Now().Rfc3339Copy().Time

	t.Log("update datacenter status")
	err = f.PatchDatacenterStatus(ctx, dcKey, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(lastTransitionTime),
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	time.Sleep(16 * time.Second)

	k8ssandraKey := framework.ClusterKey{K8sContext: "cluster-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test"}}
	k8ssandra := &api.K8ssandraCluster{}
	err = f.Get(ctx, k8ssandraKey, k8ssandra)
	require.NoError(err, "failed to get k8ssandracluster")

	require.Equal(1, len(k8ssandra.Status.Datacenters))
	kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
	require.True(found, "status not found")

	cassandraStatus := kdcStatus.Cassandra

	found = false
	condition := cassdcapi.DatacenterCondition{}

	for _, c := range cassandraStatus.Conditions {
		if c.Type == cassdcapi.DatacenterScalingUp {
			found = true
			condition = c
			break
		}
	}

	require.True(found, "did not find status condition")
	assert.Equal(t, corev1.ConditionTrue, condition.Status)
	assert.True(t, lastTransitionTime.Equal(condition.LastTransitionTime.Time), fmt.Sprintf("expected %v, got %v", lastTransitionTime, condition.LastTransitionTime.Time))
}

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
			Cassandra: &api.Cassandra{
				Cluster: "test",
				Datacenters: []api.CassandraDatacenterTemplateSpec{
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext:    k8sCtx0,
						Size:          3,
						ServerVersion: "3.11.10",
					},
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						K8sContext:    k8sCtx1,
						Size:          3,
						ServerVersion: "3.11.10",
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

	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: k8sCtx0}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

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
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: k8sCtx1}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("check that remote seeds are set on dc2")
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	assert.Equal(dc1PodIps, dc2.Spec.AdditionalSeeds, "The AdditionalSeeds property for dc2 is wrong")

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

	t.Log("check that remote seeds are set on dc1")
	err = wait.Poll(interval, timeout, func() (bool, error) {
		dc := &cassdcapi.CassandraDatacenter{}
		if err = f.Get(ctx, dc1Key, dc); err != nil {
			t.Logf("failed to get dc1: %s", err)
			return false, err
		}
		t.Logf("additional seeds for dc1: %v", dc.Spec.AdditionalSeeds)
		return equalsNoOrder(allPodIps, dc.Spec.AdditionalSeeds), nil
	})
	require.NoError(err, "timed out waiting for remote seeds to be updated on dc1")
}

func equalsNoOrder(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}

	for _, s := range s1 {
		if !contains(s2, s) {
			return false
		}
	}

	return true
}
