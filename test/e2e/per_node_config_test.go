package e2e

import (
	"context"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"testing"
	"time"
)

func multiDcInitialTokens(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	t.Log("check that the K8ssandraCluster was created")
	kc := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
	dc2Key := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, "dc2")

	checkDatacenterReady(t, ctx, dc1Key, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dc1Key.Name)

	checkDatacenterReady(t, ctx, dc2Key, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dc2Key.Name)

	t.Log("check that the ConfigMaps were created")

	perNodeConfigMapKey1 := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "test1-dc1-per-node-config")
	perNodeConfigMapKey2 := framework.NewClusterKey(f.DataPlaneContexts[1], namespace, "test1-dc2-per-node-config")

	assert.Eventually(t, func() bool {
		perNodeConfigMap := &corev1.ConfigMap{}
		return f.Get(ctx, perNodeConfigMapKey1, perNodeConfigMap) == nil
	}, time.Minute, time.Second)

	assert.Eventually(t, func() bool {
		perNodeConfigMap := &corev1.ConfigMap{}
		return f.Get(ctx, perNodeConfigMapKey2, perNodeConfigMap) == nil
	}, time.Minute, time.Second)

	dc1Pod1 := DcPrefix(t, f, dc1Key) + "-rack1-sts-0"
	dc1Pod2 := DcPrefix(t, f, dc1Key) + "-rack2-sts-0"
	dc1Pod3 := DcPrefix(t, f, dc1Key) + "-rack3-sts-0"

	// dc 1 num_tokens 4

	output := selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), dc1Pod1)
	assert.Contains(t, output, "'-9223372036854775808'")
	assert.Contains(t, output, "'-4611686018427387905'")
	assert.Contains(t, output, "'-2'")
	assert.Contains(t, output, "'4611686018427387901'")

	output = selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), dc1Pod2)
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-7686143364045646507'")
	assert.Contains(t, output, "'-3074457345618258604'")
	assert.Contains(t, output, "'1537228672809129299'")
	assert.Contains(t, output, "'6148914691236517202'")

	output = selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), dc1Pod3)
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-6148914691236517206'")
	assert.Contains(t, output, "'-1537228672809129303'")
	assert.Contains(t, output, "'3074457345618258600'")
	assert.Contains(t, output, "'7686143364045646503'")

	dc2Pod1 := DcPrefix(t, f, dc2Key) + "-rack1-sts-0"
	dc2Pod2 := DcPrefix(t, f, dc2Key) + "-rack2-sts-0"
	dc2Pod3 := DcPrefix(t, f, dc2Key) + "-rack3-sts-0"

	//dc 2 num_tokens 8

	output = selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[1], namespace, kc.SanitizedName(), dc2Pod1)
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-8533254483742592229'")
	assert.Contains(t, output, "'-6227411474528898279'")
	assert.Contains(t, output, "'-3921568465315204329'")
	assert.Contains(t, output, "'-1615725456101510379'")
	assert.Contains(t, output, "'690117553112183571'")
	assert.Contains(t, output, "'2995960562325877521'")
	assert.Contains(t, output, "'5301803571539571471'")
	assert.Contains(t, output, "'7607646580753265421'")

	output = selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[1], namespace, kc.SanitizedName(), dc2Pod2)
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-7764640147338027579'")
	assert.Contains(t, output, "'-5458797138124333629'")
	assert.Contains(t, output, "'-3152954128910639679'")
	assert.Contains(t, output, "'-847111119696945729'")
	assert.Contains(t, output, "'1458731889516748221'")
	assert.Contains(t, output, "'3764574898730442171'")
	assert.Contains(t, output, "'6070417907944136121'")
	assert.Contains(t, output, "'8376260917157830071'")

	output = selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[1], namespace, kc.SanitizedName(), dc2Pod3)
	require.NoError(t, err, "failed to execute CQL query")
	assert.Contains(t, output, "'-6996025810933462929'")
	assert.Contains(t, output, "'-4690182801719768979'")
	assert.Contains(t, output, "'-2384339792506075029'")
	assert.Contains(t, output, "'-78496783292381079'")
	assert.Contains(t, output, "'2227346225921312871'")
	assert.Contains(t, output, "'4533189235135006821'")
	assert.Contains(t, output, "'6839032244348700771'")
	assert.Contains(t, output, "'9144875253562394737'")

	t.Log("check that if the K8ssandraCluster object is deleted, the ConfigMap is also deleted")
	err = f.DeleteK8ssandraCluster(ctx, kcKey, time.Minute, time.Second)
	require.NoError(t, err, "failed to delete K8ssandraCluster")

	assert.Eventually(t, func() bool {
		perNodeConfigMap := &corev1.ConfigMap{}
		err := f.Get(ctx, perNodeConfigMapKey1, perNodeConfigMap)
		return errors.IsNotFound(err)
	}, time.Minute, time.Second)

	assert.Eventually(t, func() bool {
		perNodeConfigMap := &corev1.ConfigMap{}
		err := f.Get(ctx, perNodeConfigMapKey2, perNodeConfigMap)
		return errors.IsNotFound(err)
	}, time.Minute, time.Second)

}

func userDefinedPerNodeConfig(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

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

	output := selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), pod1)
	assert.Contains(t, output, "'-9223372036854775808'")
	assert.Contains(t, output, "'-4611686018427387905'")
	assert.Contains(t, output, "'-2'")
	assert.Contains(t, output, "'4611686018427387901'")

	output = selectTokensFromSystemLocal(t, f, ctx, f.DataPlaneContexts[0], namespace, kc.SanitizedName(), pod2)
	assert.Contains(t, output, "'-7686143364045646507'")
	assert.Contains(t, output, "'-3074457345618258604'")
	assert.Contains(t, output, "'1537228672809129299'")
	assert.Contains(t, output, "'6148914691236517202'")

	t.Log("check that if K8ssandraCluster is deleted, the ConfigMap is not deleted")
	err = f.DeleteK8ssandraCluster(ctx, kcKey, time.Minute, time.Second)
	require.NoError(t, err, "failed to delete K8ssandraCluster")

	assert.Never(t, func() bool {
		perNodeConfigMapKey := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "custom-per-node-config")
		perNodeConfigMap := &corev1.ConfigMap{}
		err := f.Get(ctx, perNodeConfigMapKey, perNodeConfigMap)
		return errors.IsNotFound(err)
	}, time.Second*15, time.Second)
}

func selectTokensFromSystemLocal(t *testing.T, f *framework.E2eFramework, ctx context.Context, k8sContext, namespace, clusterName, podName string) string {
	var output string
	assert.Eventually(t, func() bool {
		var err error
		output, err = f.ExecuteCql(ctx, k8sContext, namespace, clusterName, podName, "SELECT tokens FROM system.local")
		return err == nil
	}, 1*time.Minute, 5*time.Second, "failed to query 'SELECT tokens FROM system.local' for pod %s in context %s", podName, k8sContext)
	return output
}
