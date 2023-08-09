package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	"k8s.io/apimachinery/pkg/types"
)

// https://stargate.io/docs/stargate/1.0/developers-guide/authnz.html#_jwt_based_authenticationauthorization
func stargateJwt(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	ctx, cancelFunc := context.WithCancel(ctx)
	jwtDeployKeycloak(t, f, ctx, namespace)
	defer jwtUndeployKeycloak(t, f, namespace, cancelFunc)
	dc1Key := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dc1Key, f)
	jwtCreateAndPopulateSchema(t, f, ctx, namespace)
	stargateKey := framework.ClusterKey{K8sContext: f.DataPlaneContexts[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-real-dc1-stargate"}}
	checkStargateReady(t, f, ctx, stargateKey)
	stargateRestHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].StargateRest
	stargateGrpcHostAndPort := ingressConfigs[f.DataPlaneContexts[0]].StargateGrpc
	f.DeployStargateIngresses(t, f.DataPlaneContexts[0], namespace, "cluster1-real-dc1-stargate-service", stargateRestHostAndPort, stargateGrpcHostAndPort)
	defer f.UndeployAllIngresses(t, f.DataPlaneContexts[0], namespace)
	restClient := resty.New()
	adminToken := jwtCreateAdminAccessToken(t, restClient)
	jwtCreateStargateUser(t, restClient, adminToken)
	userToken := jwtCreateStargateUserAccessToken(t, restClient)
	assert.Eventually(t, func() bool {
		statusCode := jwtCheckStargateUserToken(t, restClient, f.DataPlaneContexts[0], userToken, 9876)
		return statusCode == http.StatusOK
	}, time.Minute, 5*time.Second, "failed to retrieve shopping cart for user 9876")
	assert.Eventually(t, func() bool {
		statusCode := jwtCheckStargateUserToken(t, restClient, f.DataPlaneContexts[0], userToken, 1234)
		return statusCode == http.StatusUnauthorized
	}, time.Minute, 5*time.Second, "failed to retrieve shopping cart for user 1234")
}

func jwtDeployKeycloak(t *testing.T, f *framework.E2eFramework, ctx context.Context, namespace string) {
	opts := kubectl.Options{
		Namespace: namespace,
		Context:   f.DataPlaneContexts[0],
	}
	err := kubectl.Apply(opts, filepath.Join("..", "testdata", "keycloak", "stargate-realm.yaml"))
	require.NoError(t, err)
	err = kubectl.Apply(opts, filepath.Join("..", "testdata", "keycloak", "keycloak.yaml"))
	require.NoError(t, err)
	err = kubectl.WaitForCondition(opts, "ready", "pod/keycloak-stargate", "--timeout=2m")
	require.NoError(t, err)
	go func() {
		_ = kubectl.PortForward(opts, ctx, "pod/keycloak-stargate", 50080, 8080)
	}()
}

func jwtUndeployKeycloak(t *testing.T, f *framework.E2eFramework, namespace string, cancelFunc context.CancelFunc) {
	cancelFunc()
	opts := kubectl.Options{
		Namespace: namespace,
		Context:   f.DataPlaneContexts[0],
	}
	err := kubectl.Delete(opts, filepath.Join("..", "testdata", "keycloak", "keycloak.yaml"))
	require.NoError(t, err)
	err = kubectl.Delete(opts, filepath.Join("..", "testdata", "keycloak", "stargate-realm.yaml"))
	require.NoError(t, err)
}

func jwtCreateAndPopulateSchema(t *testing.T, f *framework.E2eFramework, ctx context.Context, namespace string) {
	jwtExecuteCql(t, f, ctx, namespace, "CREATE KEYSPACE IF NOT EXISTS store WITH REPLICATION = {'class':'NetworkTopologyStrategy', 'real-dc1':'1'}")
	jwtExecuteCql(t, f, ctx, namespace, "CREATE TABLE IF NOT EXISTS store.shopping_cart (userid text PRIMARY KEY, item_count int, last_update_timestamp timestamp);")
	jwtExecuteCql(t, f, ctx, namespace, "INSERT INTO store.shopping_cart (userid, item_count, last_update_timestamp) VALUES ('9876', 2, toTimeStamp(toDate(now())))")
	jwtExecuteCql(t, f, ctx, namespace, "INSERT INTO store.shopping_cart (userid, item_count, last_update_timestamp) VALUES ('1234', 5, toTimeStamp(toDate(now())))")
	jwtExecuteCql(t, f, ctx, namespace, "CREATE ROLE IF NOT EXISTS 'web_user' WITH PASSWORD = 'web_user' AND LOGIN = TRUE")
	jwtExecuteCql(t, f, ctx, namespace, "GRANT MODIFY ON TABLE store.shopping_cart TO web_user")
	jwtExecuteCql(t, f, ctx, namespace, "GRANT SELECT ON TABLE store.shopping_cart TO web_user")
}

func jwtExecuteCql(t *testing.T, f *framework.E2eFramework, ctx context.Context, namespace, query string) {
	_, err := f.ExecuteCql(ctx, f.DataPlaneContexts[0], namespace, "cluster1", "cluster1-real-dc1-default-sts-0", query)
	require.NoError(t, err)
}

func jwtCreateAdminAccessToken(t *testing.T, restClient *resty.Client) string {
	return jwtCreateAccessToken(t, restClient, "master", "admin", "admin", "admin-cli")
}

func jwtCreateStargateUserAccessToken(t *testing.T, restClient *resty.Client) string {
	return jwtCreateAccessToken(t, restClient, "stargate", "testuser1", "testuser1", "user-service")
}

func jwtCreateAccessToken(t *testing.T, restClient *resty.Client, realm, username, password, clientId string) string {
	request := restClient.NewRequest().
		SetFormData(map[string]string{"username": username, "password": password, "grant_type": "password", "client_id": clientId})
	response, err := request.Post(fmt.Sprintf("http://localhost:50080/realms/%s/protocol/openid-connect/token", realm))
	require.NoError(t, err)
	// response body format:
	// {
	//    "access_token": "...",
	//    "expires_in": 60,
	//    "refresh_expires_in": 1800,
	//    "refresh_token": "...",
	//    "token_type": "Bearer",
	//    "not-before-policy": 0,
	//    "session_state": "06d924de-2274-45da-86ee-b15877e67f38",
	//    "scope": "profile email"
	//}
	var jsonResponse map[string]interface{}
	err = json.Unmarshal(response.Body(), &jsonResponse)
	require.NoError(t, err)
	return jsonResponse["access_token"].(string)
}

func jwtCreateStargateUser(t *testing.T, restClient *resty.Client, adminToken string) {
	request := restClient.NewRequest().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+adminToken).
		SetBody(map[string]interface{}{
			"username":      "testuser1",
			"enabled":       true,
			"emailVerified": true,
			"attributes": map[string]interface{}{
				"userid": []string{
					"9876",
				},
				"role": []string{
					"web_user",
				},
			},
			"credentials": []map[string]interface{}{
				{
					"type":      "password",
					"value":     "testuser1",
					"temporary": "false",
				},
			},
		})
	response, err := request.Post("http://localhost:50080/admin/realms/stargate/users")
	require.NoError(t, err)
	require.Equal(t, 201, response.StatusCode())
}

func jwtCheckStargateUserToken(t *testing.T, restClient *resty.Client, k8sContext, token string, userId int) int {
	stargateRestHostAndPort := ingressConfigs[k8sContext].StargateRest
	request := restClient.NewRequest().SetHeader("X-Cassandra-Token", token)
	response, err := request.Get(fmt.Sprintf(
		"http://%s/v2/keyspaces/store/shopping_cart/%v",
		stargateRestHostAndPort,
		userId,
	))
	require.NoError(t, err)
	return response.StatusCode()
}
