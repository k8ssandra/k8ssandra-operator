package controllers

import (
	"context"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func createDatacenter(t *testing.T, ctx context.Context, namespace string) {
	require := require.New(t)
	assert := assert.New(t)

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
						Size:          1,
						ServerVersion: "3.11.10",
					},
				},
			},
		},
	}

	err := testClient.Create(ctx, cluster)
	require.NoError(err, "failed to create K8ssandraCluster")

	t.Log("check that the datacenter was created")
	dcKey := types.NamespacedName{Namespace: namespace, Name: "dc1"}
	require.Eventually(func() bool {
		dc := &cassdcapi.CassandraDatacenter{}
		if err := testClient.Get(ctx, dcKey, dc); err != nil {
			t.Logf("failed to get datacenter: %v", err)
			return false
		}
		return true
	}, timeout, interval)

	dc := &cassdcapi.CassandraDatacenter{}
	err = testClient.Get(ctx, dcKey, dc)
	require.NoError(err, "failed to get datacenter")

	t.Log("check that the owner reference is set on the datacenter")
	assert.Equal(1, len(dc.OwnerReferences), "expected to find 1 owner reference for datacenter")
	assert.Equal(cluster.UID, dc.OwnerReferences[0].UID)
}
