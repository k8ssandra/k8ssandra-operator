package e2e

import (
	"context"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func perNodeConfigTest(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	t.Log("check that the K8ssandraCluster was created")
	kc := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
	checkDatacenterReady(t, ctx, dcKey, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dcKey.Name)

	pod1 := DcPrefix(t, f, dcKey) + "-default-sts-0"
	pod2 := DcPrefix(t, f, dcKey) + "-default-sts-1"
	pod3 := DcPrefix(t, f, dcKey) + "-default-sts-2"

	output, err := f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), pod1, "SELECT tokens FROM system.local")
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-9223372036854775808'")
	assert.Contains(t, output, "'-4611686018427387905'")
	assert.Contains(t, output, "'-2'")
	assert.Contains(t, output, "'4611686018427387901'")

	output, err = f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), pod2, "SELECT tokens FROM system.local")
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-7686143364045646507'")
	assert.Contains(t, output, "'-3074457345618258604'")
	assert.Contains(t, output, "'1537228672809129299'")
	assert.Contains(t, output, "'6148914691236517202'")

	output, err = f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), pod3, "SELECT tokens FROM system.local")
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-6148914691236517206'")
	assert.Contains(t, output, "'-1537228672809129303'")
	assert.Contains(t, output, "'3074457345618258600'")
	assert.Contains(t, output, "'7686143364045646503'")
}
