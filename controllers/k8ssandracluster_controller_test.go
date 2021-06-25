package controllers

import (
	"context"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func createSingleDcCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
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

	err := f.Client.Create(ctx, cluster)
	require.NoError(err, "failed to create K8ssandraCluster")

	t.Log("check that the datacenter was created")
	dcKey := types.NamespacedName{Namespace: namespace, Name: "dc1"}
	require.Eventually(f.DatacenterExists(ctx, dcKey), timeout, interval)

	dc := &cassdcapi.CassandraDatacenter{}
	err = f.Client.Get(ctx, dcKey, dc)
	require.NoError(err, "failed to get datacenter")

	t.Log("check that the owner reference is set on the datacenter")
	assert.Equal(1, len(dc.OwnerReferences), "expected to find 1 owner reference for datacenter")
	assert.Equal(cluster.UID, dc.OwnerReferences[0].UID)
}

func createMultiDcCluster(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

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
					{
						Meta: api.EmbeddedObjectMeta{
							Name: "dc2",
						},
						Size:          1,
						ServerVersion: "3.11.10",
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, cluster)
	require.NoError(err, "failed to create K8ssandraCluster")

	t.Log("check that dc1 was created")
}
