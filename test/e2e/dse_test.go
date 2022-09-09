package e2e

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/driver"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	require.Eventually(t, func() bool {
		return testSolrEndpoint(t, solrHostAndPort, username, password)
	}, 2*time.Minute, 10*time.Second, "failed to reach solr endpoint")
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// createSingleDseSearchDatacenterCluster creates a K8ssandraCluster with one CassandraDatacenter running with search enabled
func createSingleDseGraphDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dcKey.Name)
	dcPrefix := DcPrefix(t, f, dcKey)

	t.Log("deploying graph ingress routes in", f.DataPlaneContexts[0])
	graphHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].Graph
	f.DeployGraphIngresses(t, f.DataPlaneContexts[0], namespace, dcPrefix+"-service", graphHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)

	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], namespace, k8ssandra.SanitizedName())
	require.NoError(t, err, "failed to retrieve database credentials")

	t.Log("Check that we can reach the graph endpoint of DSE")

	var remote *gremlingo.DriverRemoteConnection

	require.Eventually(t, func() bool {
		remote, err = gremlingo.NewDriverRemoteConnection(fmt.Sprintf("ws://%s:%s/gremlin", graphHostAndPort.Host(), graphHostAndPort.Port()),
			func(settings *gremlingo.DriverRemoteConnectionSettings) {
				settings.TlsConfig = &tls.Config{InsecureSkipVerify: true}
				settings.AuthInfo = gremlingo.BasicAuthInfo(username, password)
			})
		return err == nil
	}, 2*time.Minute, 10*time.Second, "failed to create remote graph connection")
	defer remote.Close()

	_, err = remote.Submit("system.graph('test').create()")
	require.NoError(t, err, "failed to create graph")

	_, err = remote.Submit("g.V().count()")
	require.NoError(t, err, "failed to execute gremlin query against DSE")
}

// changeDseWorkload creates a K8ssandraCluster with one CassandraDatacenter using no specific workload, and then changes it to enable search
func changeDseWorkload(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)
	dcKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dcKey.Name)
	dcPrefix := DcPrefix(t, f, dcKey)

	t.Log("Check that we can communicate through CQL with DSE")
	_, err = f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, k8ssandra.SanitizedName(), DcPrefix(t, f, dcKey)+"-default-sts-0",
		"SELECT * FROM system.local")
	require.NoError(t, err, "failed to execute CQL query against DSE")

	t.Log("deploying Solr ingress routes in", f.DataPlaneContexts[0])
	solrHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].Solr
	f.DeploySolrIngresses(t, f.DataPlaneContexts[0], namespace, dcPrefix+"-service", solrHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)

	username, password, err := f.RetrieveDatabaseCredentials(ctx, f.DataPlaneContexts[0], namespace, k8ssandra.SanitizedName())
	require.NoError(t, err, "failed to retrieve database credentials")

	t.Log("Check that we cannot reach the search endpoint of DSE")

	require.Never(t, func() bool {
		return testSolrEndpoint(t, solrHostAndPort, username, password)
	}, 30*time.Second, 10*time.Second, "failed to reach solr endpoint")

	t.Log("change the workload to search")
	err = f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(t, err, "failed to get K8ssandraCluster in namespace %s", namespace)
	patch := client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	k8ssandra.Spec.Cassandra.DseWorkloads.SearchEnabled = true
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster in namespace %s", namespace)
	require.Eventually(t, func() bool {
		dc := &cassdcapi.CassandraDatacenter{}
		err := f.Get(ctx, dcKey, dc)
		if err != nil {
			t.Logf("failed to get CassandraDatacenter in namespace %s: %s", namespace, err)
			return false
		}
		t.Logf("CassDC search enabled %t", dc.Spec.DseWorkloads.SearchEnabled)
		return dc.Spec.DseWorkloads.SearchEnabled
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval)
	checkDatacenterReady(t, ctx, dcKey, f)
	assertCassandraDatacenterK8cStatusReady(ctx, t, f, kcKey, dcKey.Name)

	require.Eventually(t, func() bool {
		return testSolrEndpoint(t, solrHostAndPort, username, password)
	}, 30*time.Second, 10*time.Second, "failed to reach solr endpoint")
}

func testSolrEndpoint(t *testing.T, solrHostAndPort framework.HostAndPort, username, password string) bool {
	client := &http.Client{}
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
}
