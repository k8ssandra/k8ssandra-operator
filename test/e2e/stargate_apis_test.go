package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"strconv"
	"testing"
	"time"

	retry "github.com/avast/retry-go/v4"
	"github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

func testStargateApis(t *testing.T, ctx context.Context, k8sContextName string, k8sContextIdx int, username string, password string, replication map[string]int) {
	t.Run(fmt.Sprintf("TestStargateApis[%s]", k8sContextName), func(t *testing.T) {
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

func testStargateRestApis(t *testing.T, k8sContextIdx int, username string, password string, replication map[string]int) {
	restClient := resty.New()
	token := authenticate(t, restClient, k8sContextIdx, username, password)
	t.Run("TestSchemaApi", func(t *testing.T) {
		testSchemaApi(t, restClient, k8sContextIdx, token, replication)
	})
	t.Run("TestDocumentApi", func(t *testing.T) {
		testDocumentApi(t, restClient, k8sContextIdx, token, replication)
	})
}

func testStargateNativeApi(t *testing.T, ctx context.Context, k8sContextIdx int, username string, password string, replication map[string]int) {
	connection := openCqlClientConnection(t, ctx, k8sContextIdx, username, password)
	defer connection.Close()
	tableName := fmt.Sprintf("table_%s", rand.String(6))
	keyspaceName := fmt.Sprintf("ks_%s", rand.String(6))
	createKeyspaceAndTableNative(t, connection, tableName, keyspaceName, replication)
	insertRowsNative(t, connection, 10, tableName, keyspaceName)
	checkRowCountNative(t, connection, 10, tableName, keyspaceName)
}

func testSchemaApi(t *testing.T, restClient *resty.Client, k8sContextIdx int, token string, replication map[string]int) {
	tableName := fmt.Sprintf("table_%s", rand.String(6))
	keyspaceName := fmt.Sprintf("ks_%s", rand.String(6))
	createKeyspaceAndTableRest(t, restClient, k8sContextIdx, token, tableName, keyspaceName, replication)
	insertRowsRest(t, restClient, k8sContextIdx, token, 10, tableName, keyspaceName)
	checkRowCountRest(t, restClient, k8sContextIdx, token, 10, tableName, keyspaceName)
}

func testDocumentApi(t *testing.T, restClient *resty.Client, k8sContextIdx int, token string, replication map[string]int) {
	documentNamespace := fmt.Sprintf("ns_%s", rand.String(6))
	documentId := fmt.Sprintf("watchmen_%s", rand.String(6))
	createDocumentNamespace(t, restClient, k8sContextIdx, token, documentNamespace, replication)
	writeDocument(t, restClient, k8sContextIdx, token, documentNamespace, documentId)
	readDocument(t, restClient, k8sContextIdx, token, documentNamespace, documentId)
}

func createKeyspaceAndTableRest(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, tableName, keyspaceName string, replication map[string]int) {
	timeout := 2 * time.Minute
	interval := 1 * time.Second
	require.Eventually(t, func() bool {
		keyspaceUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/keyspaces", k8sContextIdx)
		keyspaceJson := fmt.Sprintf(`{"name":"%v","datacenters":%v}`, keyspaceName, formatReplicationForRestApi(replication))
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetHeader("X-Cassandra-Token", token).
			SetBody(keyspaceJson)
		response, err := request.Post(keyspaceUrl)
		return err == nil && response.StatusCode() == http.StatusCreated
	}, timeout, interval, "Create keyspace with Schema API failed")
	require.Eventually(t, func() bool {
		keyspaceUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/keyspaces/%v", k8sContextIdx, keyspaceName)
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetHeader("X-Cassandra-Token", token)
		response, err := request.Get(keyspaceUrl)
		return err == nil && response.StatusCode() == http.StatusOK
	}, timeout, interval, "Retrieve keyspace with Schema API failed")
	require.Eventually(t, func() bool {
		tableUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/keyspaces/%s/tables", k8sContextIdx, keyspaceName)
		tableJson := fmt.Sprintf(
			`{ 
          "name": "%v", 
          "ifNotExists": true, 
		  "columnDefinitions": [
			{ "name": "pk", "typeDefinition": "int", "static": false },
			{ "name": "cc", "typeDefinition": "int", "static": false },
			{ "name": "v" , "typeDefinition": "int", "static": false }
		  ],
		  "primaryKey": { "partitionKey": [ "pk" ], "clusteringKey": [ "cc" ] }
		}`, tableName)
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetHeader("X-Cassandra-Token", token).
			SetBody(tableJson)
		response, err := request.Post(tableUrl)
		return err == nil && response.StatusCode() == http.StatusCreated
	}, timeout, interval, "Create table with Schema API failed")
	require.Eventually(t, func() bool {
		tableUrl := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/keyspaces/%v/tables/%v", k8sContextIdx, keyspaceName, tableName)
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetHeader("X-Cassandra-Token", token)
		response, err := request.Get(tableUrl)
		return err == nil && response.StatusCode() == http.StatusOK
	}, timeout, interval, "Retrieve table with Schema API failed")
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
	if assert.NoError(t, err, "Retrieve rows with Schema API failed") &&
		assert.Equal(t, http.StatusOK, response.StatusCode(), "Expected retrieve data request to return 200") {
		data := string(response.Body())
		expected := fmt.Sprintf(`"count":%v`, nbRows)
		assert.Contains(t, data, expected, "Expected response body to contain count:%d", nbRows)
	}
}

func createDocumentNamespace(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, documentNamespace string, replication map[string]int) string {
	timeout := 2 * time.Minute
	interval := 1 * time.Second
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/namespaces", k8sContextIdx)
		documentNamespaceJson := fmt.Sprintf(`{"name":"%s","datacenters":%v}`, documentNamespace, formatReplicationForRestApi(replication))
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetHeader("X-Cassandra-Token", token).
			SetBody(documentNamespaceJson)
		response, err := request.Post(url)
		return err == nil && response.StatusCode() == http.StatusCreated
	}, timeout, interval, "Create namespace with Document API failed")
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/schemas/namespaces/%v", k8sContextIdx, documentNamespace)
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetHeader("X-Cassandra-Token", token)
		response, err := request.Get(url)
		return err == nil && response.StatusCode() == http.StatusOK
	}, timeout, interval, "Retrieve namespace with Document API failed")
	return documentNamespace
}

const (
	awesomeMovieDirector = "Zack Snyder"
	awesomeMovieName     = "Watchmen"
)

func writeDocument(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, documentNamespace, documentId string) {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/namespaces/%s/collections/movies/%s", k8sContextIdx, documentNamespace, documentId)
	awesomeMovieDocument := map[string]string{"Director": awesomeMovieDirector, "Name": awesomeMovieName}
	response, err := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token).
		SetBody(awesomeMovieDocument).
		Put(url)
	if assert.NoError(t, err, "Failed writing Stargate document") {
		stargateResponse := string(response.Body())
		expectedResponse := fmt.Sprintf("{\"documentId\":\"%s\"}", documentId)
		assert.Equal(t, expectedResponse, stargateResponse, "Unexpected response from Stargate: '%s'", stargateResponse)
	}
}

func readDocument(t *testing.T, restClient *resty.Client, k8sContextIdx int, token, documentNamespace, documentId string) {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v2/namespaces/%s/collections/movies/%s", k8sContextIdx, documentNamespace, documentId)
	response, err := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Cassandra-Token", token).
		Get(url)
	require.NoError(t, err, "Failed to retrieve Stargate document")
	var genericJson map[string]interface{}
	err = json.Unmarshal(response.Body(), &genericJson)
	if assert.NoError(t, err, "Failed to decode Stargate response") &&
		assert.Contains(t, genericJson, "documentId") &&
		assert.Contains(t, genericJson, "data") {
		assert.Equal(t, documentId, genericJson["documentId"], "Expected JSON payload to contain a field documentId")
		if assert.IsType(t, map[string]interface{}{}, genericJson["data"], "Expected field data to be map[string]interface{}") {
			data := genericJson["data"].(map[string]interface{})
			assert.Equal(t, awesomeMovieDirector, data["Director"], "Expected JSON payload to contain a field data.Director")
			assert.Equal(t, awesomeMovieName, data["Name"], "Expected JSON payload to contain a field data.Name")
		}
	}
}

func authenticate(t *testing.T, restClient *resty.Client, k8sContextIdx int, username, password string) string {
	var result map[string]interface{}
	require.Eventually(t, func() bool {
		url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v1/auth", k8sContextIdx)
		body := map[string]string{"username": username, "password": password}
		request := restClient.NewRequest().
			SetHeader("Content-Type", "application/json").
			SetBody(body)
		response, err := request.Post(url)
		return err == nil &&
			response.StatusCode() == http.StatusCreated &&
			json.Unmarshal(response.Body(), &result) == nil
	}, time.Minute, time.Second, "Authentication via REST failed")
	token, found := result["authToken"]
	assert.True(t, found, "REST authentication response did not have expected authToken field")
	var tokenStr string
	if assert.IsType(t, "", token) {
		tokenStr = token.(string)
		assert.NotEmpty(t, tokenStr, "REST authentication response did not have expected authToken field")
	}
	return tokenStr
}

func openCqlClientConnection(t *testing.T, ctx context.Context, k8sContextIdx int, username, password string) *client.CqlClientConnection {
	contactPoint := fmt.Sprintf("stargate.127.0.0.1.nip.io:3%v942", k8sContextIdx)
	var credentials *client.AuthCredentials
	if username != "" {
		credentials = &client.AuthCredentials{Username: username, Password: password}
	}
	cqlClient := client.NewCqlClient(contactPoint, credentials)
	cqlClient.ConnectTimeout = 30 * time.Second
	cqlClient.ReadTimeout = 1 * time.Minute
	connection, err := cqlClient.ConnectAndInit(ctx, primitive.ProtocolVersion4, client.ManagedStreamId)
	require.NoError(t, err, "Failed to connect via CQL native port to %s", contactPoint)
	return connection
}

func createKeyspaceAndTableNative(t *testing.T, connection *client.CqlClientConnection, tableName, keyspaceName string, replication map[string]int) {
	require.Eventually(t, func() bool {
		response, err := sendQuery(connection, fmt.Sprintf(
			"CREATE KEYSPACE IF NOT EXISTS %s with replication = {'class':'NetworkTopologyStrategy', %s}",
			keyspaceName,
			formatReplicationForCql(replication),
		))
		if err != nil {
			return false
		}
		_, ok := response.Body.Message.(*message.SchemaChangeResult)
		return ok
	}, time.Minute, time.Second, "CREATE KEYSPACE via CQL failed")
	require.Eventually(t, func() bool {
		response, err := sendQuery(connection, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s.%s (id timeuuid PRIMARY KEY, val text)",
			keyspaceName,
			tableName,
		))
		if err != nil {
			return false
		}
		_, ok := response.Body.Message.(*message.SchemaChangeResult)
		return ok
	}, time.Minute, time.Second, "CREATE TABLE via CQL failed")
}

func insertRowsNative(t *testing.T, connection *client.CqlClientConnection, nbRows int, tableName, keyspaceName string) {
	for i := 0; i < nbRows; i++ {
		query := fmt.Sprintf("INSERT INTO %s.%s (id, val) VALUES (now(), '%d')", keyspaceName, tableName, i)
		response, err := sendQuery(connection, query)
		assert.NoError(t, err, "Query failed: %s", query)
		assert.IsType(t, &message.VoidResult{}, response.Body.Message, "Expected INSERT INTO response to be of type VoidResult")
	}
}

func checkRowCountNative(t *testing.T, connection *client.CqlClientConnection, nbRows int, tableName, keyspaceName string) {
	query := fmt.Sprintf("SELECT id FROM %s.%s", keyspaceName, tableName)
	response, err := sendQuery(connection, query)
	if assert.NoError(t, err, "Query failed: %s", query) &&
		assert.IsType(t, &message.RowsResult{}, response.Body.Message, "Expected SELECT response to be of type RowsResult") {
		result := response.Body.Message.(*message.RowsResult)
		assert.Len(t, result.Data, nbRows, "Expected SELECT query to return %d rows", nbRows)
	}
}

func sendQuery(connection *client.CqlClientConnection, query string) (*frame.Frame, error) {
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

	var result *frame.Frame
	var err2 error
	retry.Do(func() error {
		result, err2 = connection.SendAndReceive(request)
		if err2 != nil {
			return err2
		}
		return nil
	})
	return result, err2
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
