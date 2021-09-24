package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"net/http"
	neturl "net/url"
	"os/exec"
	"strconv"
	"testing"
)

func testStargateApis(
	t *testing.T,
	ctx context.Context,
	k8sContextIdx int,
	username string,
	password string,
	replication map[string]int,
	// for debugging purposes
	k8sContextName string,
	namespace string,
	dcName string,
	stargateRacks ...string,
) {
	t.Run(fmt.Sprintf("TestStargateApis[%d]", k8sContextIdx), func(t *testing.T) {
		defer func() {
			if t.Failed() {
				for _, rack := range stargateRacks {
					dumpStargateLogs(k8sContextName, namespace, dcName, rack)
				}
			}
		}()
		t.Run("TestStargateNativeApi", func(t *testing.T) {
			t.Log("test Stargate native API in context " + k8sContextName)
			testStargateNativeApi(t, ctx, k8sContextIdx, username, password, replication)
		})
		t.Run("TestStargateRestApi", func(t *testing.T) {
			t.Log("test Stargate REST API in context " + k8sContextName)
			testStargateRestApis(t, k8sContextIdx, username, password, replication)
		})
	})
}

func dumpStargateLogs(k8sContextName string, namespace string, dc string, rack string) {
	deploymentName := fmt.Sprintf("deployment.apps/test-%s-%s-stargate-deployment", dc, rack)
	cmd := exec.Command("kubectl", "logs", deploymentName, "-n", namespace, "--context", k8sContextName)
	output, _ := cmd.CombinedOutput()
	println()
	fmt.Println("=============", "BEGIN STARGATE LOGS", dc, rack, "context", k8sContextName, "namespace", namespace, "=============")
	fmt.Println(string(output))
	fmt.Println("=============", "END STARGATE LOGS", dc, rack, "context", k8sContextName, "namespace", namespace, "=============")
	println()
}

func testStargateRestApis(t *testing.T, k8sContextIdx int, username string, password string, replication map[string]int) {
	restClient := resty.New()
	token := authenticate(t, restClient, k8sContextIdx, username, password)
	testSchemaApi(t, restClient, k8sContextIdx, token, replication)
	testDocumentApi(t, restClient, k8sContextIdx, token, replication)
}

func testStargateNativeApi(t *testing.T, ctx context.Context, k8sContextIdx int, username string, password string, replication map[string]int) {
	connection := openCqlClientConnection(t, ctx, k8sContextIdx, username, password)
	defer connection.Close()
	tableName := fmt.Sprintf("table_%s", rand.String(6))
	keyspaceName := fmt.Sprintf("ks_%s", rand.String(6))
	createKeyspaceAndTableNative(t, connection, tableName, keyspaceName, replication)
	insertRowsNative(t, connection, 10, tableName, keyspaceName)
	checkRowCountNative(t, connection, 10, tableName, keyspaceName)
	dropKeyspaceNative(t, connection, keyspaceName)
}

func testSchemaApi(t *testing.T, restClient *resty.Client, k8sContextIdx int, token string, replication map[string]int) {
	tableName := fmt.Sprintf("table_%s", rand.String(6))
	keyspaceName := fmt.Sprintf("ks_%s", rand.String(6))
	createKeyspaceAndTableRest(t, restClient, k8sContextIdx, token, tableName, keyspaceName, replication)
	insertRowsRest(t, restClient, k8sContextIdx, token, 10, tableName, keyspaceName)
	checkRowCountRest(t, restClient, k8sContextIdx, token, 10, tableName, keyspaceName)
	dropKeyspaceRest(t, restClient, k8sContextIdx, token, keyspaceName)
}

func testDocumentApi(t *testing.T, restClient *resty.Client, k8sContextIdx int, token string, replication map[string]int) {
	documentNamespace := fmt.Sprintf("ns_%s", rand.String(6))
	documentId := fmt.Sprintf("watchmen_%s", rand.String(6))
	createDocumentNamespace(t, restClient, k8sContextIdx, token, documentNamespace, replication)
	writeDocument(t, restClient, k8sContextIdx, token, documentNamespace, documentId)
	readDocument(t, restClient, k8sContextIdx, token, documentNamespace, documentId)
	dropDocumentNamespace(t, restClient, k8sContextIdx, token, documentNamespace)
}

func createKeyspaceAndTableRest(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, tableName, keyspaceName string, replication map[string]int) {
	keyspaceUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/keyspaces", k8sContextIdx)
	keyspaceJson := fmt.Sprintf(`{"name":"%v","datacenters":%v}`, keyspaceName, formatReplicationForRestApi(replication))
	request := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token).
		SetBody(keyspaceJson)
	response, err := request.Post(keyspaceUrl)
	require.NoError(t, err, "Create keyspace with Schema API failed")
	assert.Equal(t, http.StatusCreated, response.StatusCode(), "Expected create keyspace request to return 201")
	tableUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/keyspaces/%s/tables", k8sContextIdx, keyspaceName)
	tableJson := fmt.Sprintf(`{ "name": "%v",
  "columnDefinitions": [
    { "name": "pk", "typeDefinition": "int", "static": false },
    { "name": "cc", "typeDefinition": "int", "static": false },
    { "name": "v" , "typeDefinition": "int", "static": false }
  ],
  "primaryKey": { "partitionKey": [ "pk" ], "clusteringKey": [ "cc" ] }
}`, tableName)
	request = restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token).
		SetBody(tableJson)
	response, err = request.Post(tableUrl)
	require.NoError(t, err, "Create table with Schema API failed")
	assert.Equal(t, http.StatusCreated, response.StatusCode(), "Expected create table request to return 201")
}

func dropKeyspaceRest(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, keyspaceName string) {
	keyspaceUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/keyspaces/%s", k8sContextIdx, keyspaceName)
	request := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token)
	response, err := request.Delete(keyspaceUrl)
	assert.NoError(t, err, "Delete keyspace with Schema API failed")
	assert.Equal(t, http.StatusNoContent, response.StatusCode(), "Expected drop keyspace request to return 204")
}

func insertRowsRest(t *testing.T, restClient *resty.Client, k8sContextIdx int, token string, nbRows int, tableName, keyspaceName string) {
	tableUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/keyspaces/%s/%s", k8sContextIdx, keyspaceName, tableName)
	for i := 0; i < nbRows; i++ {
		rowJson := fmt.Sprintf(`{"pk":"0","cc":"%v","v":"%v"}`, i, i)
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetHeader("X-Cassandra-Token", token).
			SetBody(rowJson)
		response, err := request.Post(tableUrl)
		assert.NoError(t, err, "Insert row with Schema API failed")
		assert.Equal(t, http.StatusCreated, response.StatusCode(), "Expected insert row request to return 201")
	}
}

func checkRowCountRest(t *testing.T, restClient *resty.Client, k8sContextIdx int, token string, nbRows int, tableName, keyspaceName string) {
	tableUrl := fmt.Sprintf(
		"http://stargate.127.0.0.1.nip.io:3%v080/v2/keyspaces/%s/%s?",
		k8sContextIdx,
		keyspaceName,
		tableName,
	)
	params := neturl.Values{}
	params.Add("where", `{"pk":{"$eq":"0"}}`)
	request := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token)
	response, err := request.Get(tableUrl + params.Encode())
	assert.NoError(t, err, "Retrieve rows with Schema API failed")
	assert.Equal(t, http.StatusOK, response.StatusCode(), "Expected retrieve data request to return 200")
	data := string(response.Body())
	expected := fmt.Sprintf(`"count":%v`, nbRows)
	assert.Contains(t, data, expected, "Expected response body to contain count:%d", nbRows)
}

func createDocumentNamespace(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, documentNamespace string, replication map[string]int) string {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/namespaces", k8sContextIdx)
	documentNamespaceJson := fmt.Sprintf(`{"name":"%s","datacenters":%v}`, documentNamespace, formatReplicationForRestApi(replication))
	request := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token).
		SetBody(documentNamespaceJson)
	response, err := request.Post(url)
	assert.NoError(t, err, "Failed creating Stargate document namespace")
	assert.Equal(t, http.StatusCreated, response.StatusCode(), "Expected create namespace request to return 201")
	return documentNamespace
}

func dropDocumentNamespace(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, documentNamespace string) {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/namespaces/%s", k8sContextIdx, documentNamespace)
	request := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token)
	response, err := request.Delete(url)
	assert.NoError(t, err, "Failed deleting Stargate document namespace")
	assert.Equal(t, http.StatusNoContent, response.StatusCode(), "Expected delete namespace request to return 201")
}

const (
	awesomeMovieDirector = "Zack Snyder"
	awesomeMovieName     = "Watchmen"
)

func writeDocument(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, documentNamespace, documentId string) string {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/namespaces/%s/collections/movies/%s", k8sContextIdx, documentNamespace, documentId)
	awesomeMovieDocument := fmt.Sprintf("{\"Director\":\"%s\",\"Name\":\"%s\"}", awesomeMovieDirector, awesomeMovieName)
	response, err := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token).
		SetBody(awesomeMovieDocument).
		Put(url)
	assert.NoError(t, err, "Failed writing Stargate document")
	stargateResponse := string(response.Body())
	expectedResponse := fmt.Sprintf("{\"documentId\":\"%s\"}", documentId)
	assert.Equal(t, expectedResponse, stargateResponse, "Unexpected response from Stargate: '%s'", stargateResponse)
	return documentId
}

func readDocument(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, documentNamespace, documentId string) {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/namespaces/%s/collections/movies/%s", k8sContextIdx, documentNamespace, documentId)
	response, err := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token).
		Get(url)
	assert.NoError(t, err, "Failed to retrieve Stargate document")
	var genericJson map[string]interface{}
	err = json.Unmarshal(response.Body(), &genericJson)
	assert.NoError(t, err, "Failed to decode Stargate response")
	assert.Equal(t, documentId, genericJson["documentId"], "Expected JSON payload to contain a field documentId")
	assert.Equal(t, awesomeMovieDirector, genericJson["data"].(map[string]interface{})["Director"], "Expected JSON payload to contain a field data.Director")
	assert.Equal(t, awesomeMovieName, genericJson["data"].(map[string]interface{})["Name"], "Expected JSON payload to contain a field data.Name")
}

func authenticate(t *testing.T, restClient *resty.Client, k8sContextIdx int, username, password string) string {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v1/auth", k8sContextIdx)
	body := fmt.Sprintf("{\"username\": \"%s\", \"password\": \"%s\"}", username, password)
	request := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetBody(body)
	response, err := request.Post(url)
	require.NoError(t, err, "Authentication via REST API failed")
	require.Equal(t, http.StatusCreated, response.StatusCode(), "Expected auth request to return 201")
	var result map[string]interface{}
	err = json.Unmarshal(response.Body(), &result)
	require.NoError(t, err, "Failed to decode Stargate auth response")
	token, found := result["authToken"]
	assert.True(t, found, "REST authentication response did not have expected authToken field")
	tokenStr := token.(string)
	assert.NotEmpty(t, tokenStr, "REST authentication response did not have expected authToken field")
	return tokenStr
}

func retrieveDatabaseCredentials(t *testing.T, f *framework.E2eFramework, ctx context.Context, k8sContext, namespace, clusterName string) (string, string) {
	superUserSecret := retrieveSuperuserSecret(t, f, ctx, k8sContext, namespace, clusterName)
	username := string(superUserSecret.Data["username"])
	password := string(superUserSecret.Data["password"])
	return username, password
}

func retrieveSuperuserSecret(t *testing.T, f *framework.E2eFramework, ctx context.Context, k8sContext, namespace, clusterName string) *corev1.Secret {
	superUserSecret := &corev1.Secret{}
	superUserSecretKey := framework.ClusterKey{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      clusterName + "-superuser",
		},
		K8sContext: k8sContext,
	}
	err := f.Get(ctx, superUserSecretKey, superUserSecret)
	require.NoError(t, err, "Failed to get superuser secret")
	return superUserSecret
}

func openCqlClientConnection(t *testing.T, ctx context.Context, k8sContextIdx int, username, password string) *client.CqlClientConnection {
	contactPoint := fmt.Sprintf("stargate.127.0.0.1.nip.io:3%v942", k8sContextIdx)
	cqlClient := client.NewCqlClient(contactPoint, &client.AuthCredentials{
		Username: username,
		Password: password,
	})
	connection, err := cqlClient.ConnectAndInit(ctx, primitive.ProtocolVersion4, client.ManagedStreamId)
	require.NoError(t, err, "Failed to connect via CQL native port to %s", contactPoint)
	return connection
}

func createKeyspaceAndTableNative(t *testing.T, connection *client.CqlClientConnection, tableName, keyspaceName string, replication map[string]int) {
	response := sendQuery(t, connection, fmt.Sprintf(
		"CREATE KEYSPACE IF NOT EXISTS %s with replication = {'class':'NetworkTopologyStrategy', %s}",
		keyspaceName,
		formatReplicationForCql(replication),
	))
	require.IsType(t, &message.SchemaChangeResult{}, response.Body.Message, "Expected CREATE KEYSPACE response to be of type SchemaChangeResult")
	response = sendQuery(t, connection, fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s.%s (id timeuuid PRIMARY KEY, val text)",
		keyspaceName,
		tableName,
	))
	require.IsType(t, &message.SchemaChangeResult{}, response.Body.Message, "Expected CREATE TABLE response to be of type SchemaChangeResult")
	response = sendQuery(t, connection, fmt.Sprintf("TRUNCATE %s.%s", keyspaceName, tableName))
	assert.IsType(t, &message.VoidResult{}, response.Body.Message, "Expected TRUNCATE response to be of type VoidResult")
}

func dropKeyspaceNative(t *testing.T, connection *client.CqlClientConnection, keyspaceName string) {
	response := sendQuery(t, connection, fmt.Sprintf("DROP KEYSPACE IF EXISTS %s", keyspaceName))
	assert.IsType(t, &message.SchemaChangeResult{}, response.Body.Message, "Expected DROP KEYSPACE response to be of type SchemaChangeResult")
}

func insertRowsNative(t *testing.T, connection *client.CqlClientConnection, nbRows int, tableName, keyspaceName string) {
	for i := 0; i < nbRows; i++ {
		response := sendQuery(t, connection, fmt.Sprintf(
			"INSERT INTO %s.%s (id, val) VALUES (now(), '%d')",
			keyspaceName,
			tableName,
			i,
		))
		assert.IsType(t, &message.VoidResult{}, response.Body.Message, "Expected INSERT INTO response to be of type VoidResult")
	}
}

func checkRowCountNative(t *testing.T, connection *client.CqlClientConnection, nbRows int, tableName, keyspaceName string) {
	response := sendQuery(t, connection, fmt.Sprintf("SELECT id FROM %s.%s", keyspaceName, tableName))
	assert.IsType(t, &message.RowsResult{}, response.Body.Message, "Expected SELECT response to be of type RowsResult")
	result := response.Body.Message.(*message.RowsResult)
	assert.Len(t, result.Data, nbRows, "Expected SELECT query to return %d rows", nbRows)
}

func sendQuery(t *testing.T, connection *client.CqlClientConnection, query string) *frame.Frame {
	request := frame.NewFrame(
		primitive.ProtocolVersion4,
		client.ManagedStreamId,
		&message.Query{
			Query: query,
			Options: &message.QueryOptions{
				Consistency: primitive.ConsistencyLevelQuorum,
			},
		},
	)
	response, err := connection.SendAndReceive(request)
	require.NoError(t, err, "Failed to send QUERY request: %s", query)
	require.NotNil(t, response.Body, "Expected non nil response body")
	return response
}

func formatReplicationForRestApi(replication map[string]int) string {
	s := "["
	for dcName, dcRf := range replication {
		if s != "[" {
			s += ","
		}
		s += fmt.Sprintf(`{"name":"%v","replicas":%v}`, dcName, strconv.Itoa(dcRf))
	}
	return s + "]"
}

func formatReplicationForCql(replication map[string]int) string {
	var s string
	for dcName, dcRf := range replication {
		if s != "" {
			s += ","
		}
		s += fmt.Sprintf(`'%v':%v`, dcName, strconv.Itoa(dcRf))
	}
	return s
}
