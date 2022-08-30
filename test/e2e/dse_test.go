package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"
	"time"

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
		"SELECT * FROM system.local")
	require.NoError(t, err, "failed to execute CQL query against DSE")
}

// createSingleDseSearchDatacenterCluster creates a K8ssandraCluster with one CassandraDatacenter running with search enabled
func createSingleDseSearchDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dcKey.Name)
	dcPrefix := DcPrefix(t, f, dcKey)

	t.Log("deploying Solr ingress routes in", f.DataPlaneContexts[0])
	solrHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].Solr
	f.DeploySolrIngresses(t, f.DataPlaneContexts[0], namespace, dcPrefix+"-service", solrHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)

	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], namespace, k8ssandra.SanitizedName())
	require.NoError(t, err, "failed to retrieve database credentials")

	t.Log("Check that we can reach the search endpoint of DSE")
	client := &http.Client{}

	require.Eventually(t, func() bool {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%s/solr", solrHostAndPort.Host(), solrHostAndPort.Port()), nil)
		if err != nil {
			t.Logf("failed to create request: %v", err)
			return false
		}

		req.Header.Add("Authorization", "Basic "+basicAuth(username, password))
		if resp, err := client.Do(req); err != nil {
			t.Logf("failed to reach search endpoint: %s", err)
			return false
		} else {
			t.Logf("search endpoint returned %d", resp.StatusCode)
			return http.StatusOK == resp.StatusCode
		}
	}, 2*time.Minute, 10*time.Second, "failed to reach solr endpoint")
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
