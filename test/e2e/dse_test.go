package e2e

import (
	"context"
	"fmt"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

// createSingleDseDatacenterCluster creates a K8ssandraCluster with one CassandraDatacenter running
func createSingleDseDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dcKey.Name)

	t.Log("Check that we can communicate through CQL with DSE")
	_, err = f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, k8ssandra.SanitizedName(), DcPrefix(t, f, dcKey)+"-default-sts-0",
		fmt.Sprintf("CREATE KEYSPACE ks1 WITH REPLICATION = {'class' : 'NetworkTopologyStrategy', 'dc1' : %d}", 3))
	require.NoError(t, err, "failed to create keyspace")
}
