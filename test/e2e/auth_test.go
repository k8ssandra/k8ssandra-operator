package e2e

import (
	"context"
	"fmt"
	cqlclient "github.com/datastax/go-cassandra-native-protocol/client"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/resty.v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

func multiDcAuthOnOff(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}

	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}

	reaper1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc1-reaper"}}
	reaper2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc2-reaper"}}

	stargate1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc1-stargate"}}
	stargate2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cluster1-dc2-stargate"}}

	superuserSecretKey := types.NamespacedName{Namespace: namespace, Name: secret.DefaultSuperuserSecretName("cluster1")}
	reaperCqlSecretKey := types.NamespacedName{Namespace: namespace, Name: reaper.DefaultUserSecretName("cluster1")}
	reaperJmxSecretKey := types.NamespacedName{Namespace: namespace, Name: reaper.DefaultJmxUserSecretName("cluster1")}

	// cluster has auth turned off initially
	checkSecrets(t, f, ctx, kcKey, superuserSecretKey, reaperCqlSecretKey, reaperJmxSecretKey, false)
	waitForAllComponentsReady(t, f, ctx, kcKey, dc1Key, dc2Key, stargate1Key, stargate2Key, reaper1Key, reaper2Key)

	t.Log("deploying Stargate and Reaper ingress routes in both clusters")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "cluster1-dc1-stargate-service", "", "")
	f.DeployStargateIngresses(t, "kind-k8ssandra-1", 1, namespace, "cluster1-dc2-stargate-service", "", "")
	f.DeployReaperIngresses(t, ctx, "kind-k8ssandra-0", 0, namespace, "cluster1-dc1-reaper-service")
	f.DeployReaperIngresses(t, ctx, "kind-k8ssandra-1", 1, namespace, "cluster1-dc2-reaper-service")
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-1", namespace)

	pod1Name := "cluster1-dc1-default-sts-0"
	pod2Name := "cluster1-dc2-default-sts-0"
	replication := map[string]int{"dc1": 1, "dc2": 1}

	testAuthenticationDisabled(t, f, ctx, namespace, replication, pod1Name, pod2Name)

	// turn auth on
	toggleAuthentication(t, f, ctx, kcKey, true)
	checkSecrets(t, f, ctx, kcKey, superuserSecretKey, reaperCqlSecretKey, reaperJmxSecretKey, true)
	waitForAllComponentsReady(t, f, ctx, kcKey, dc1Key, dc2Key, stargate1Key, stargate2Key, reaper1Key, reaper2Key)
	testAuthenticationEnabled(t, f, ctx, namespace, kcKey, replication, pod1Name, pod2Name)

	// turn auth off again
	toggleAuthentication(t, f, ctx, kcKey, false)
	checkSecrets(t, f, ctx, kcKey, superuserSecretKey, reaperCqlSecretKey, reaperJmxSecretKey, true)
	waitForAllComponentsReady(t, f, ctx, kcKey, dc1Key, dc2Key, stargate1Key, stargate2Key, reaper1Key, reaper2Key)
	testAuthenticationDisabled(t, f, ctx, namespace, replication, pod1Name, pod2Name)
}

func checkSecrets(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	superuserSecretKey types.NamespacedName,
	reaperCqlSecretKey types.NamespacedName,
	reaperJmxSecretKey types.NamespacedName,
	expectReaperSecretsCreated bool,
) {
	t.Log("check that superuser secret exists in both contexts")
	checkSecretExists(t, f, ctx, kcKey, framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: superuserSecretKey})
	checkSecretExists(t, f, ctx, kcKey, framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: superuserSecretKey})
	if expectReaperSecretsCreated {
		t.Log("check that reaper CQL secret exists in both contexts")
		checkSecretExists(t, f, ctx, kcKey, framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: reaperCqlSecretKey})
		checkSecretExists(t, f, ctx, kcKey, framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: reaperCqlSecretKey})
		t.Log("check that reaper JMX secret exists in both contexts")
		checkSecretExists(t, f, ctx, kcKey, framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: reaperJmxSecretKey})
		checkSecretExists(t, f, ctx, kcKey, framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: reaperJmxSecretKey})
	} else {
		t.Log("check that reaper CQL secret wasn't created in neither context")
		checkSecretDoesNotExist(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: reaperCqlSecretKey})
		checkSecretDoesNotExist(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: reaperCqlSecretKey})
		t.Log("check that reaper JMX secret wasn't created in neither context")
		checkSecretDoesNotExist(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: reaperJmxSecretKey})
		checkSecretDoesNotExist(t, f, ctx, framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: reaperJmxSecretKey})
	}
}

func waitForAllComponentsReady(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dc1Key, dc2Key framework.ClusterKey,
	stargate1Key, stargate2Key framework.ClusterKey,
	reaper1Key, reaper2Key framework.ClusterKey,
) {
	checkDatacenterReady(t, ctx, dc1Key, f)
	checkDatacenterReady(t, ctx, dc2Key, f)
	checkStargateReady(t, f, ctx, stargate1Key)
	checkStargateK8cStatusReady(t, f, ctx, kcKey, dc1Key)
	checkReaperReady(t, f, ctx, reaper1Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc1Key)
	checkStargateReady(t, f, ctx, stargate2Key)
	checkStargateK8cStatusReady(t, f, ctx, kcKey, dc2Key)
	checkReaperReady(t, f, ctx, reaper2Key)
	checkReaperK8cStatusReady(t, f, ctx, kcKey, dc2Key)
	// we need to wait until the deployments are fully rolled out before continuing, to avoid hitting an old
	// pod that has authentication enabled while we just turned it off.
	options1 := kubectl.Options{Namespace: kcKey.Namespace, Context: "kind-k8ssandra-0"}
	options2 := kubectl.Options{Namespace: kcKey.Namespace, Context: "kind-k8ssandra-1"}
	err := kubectl.RolloutStatus(options1, "deployment", "cluster1-dc1-default-stargate-deployment")
	assert.NoError(t, err)
	err = kubectl.RolloutStatus(options1, "deployment", "cluster1-dc1-reaper")
	assert.NoError(t, err)
	err = kubectl.RolloutStatus(options2, "deployment", "cluster1-dc2-default-stargate-deployment")
	assert.NoError(t, err)
	err = kubectl.RolloutStatus(options2, "deployment", "cluster1-dc2-reaper")
	assert.NoError(t, err)
}

func toggleAuthentication(t *testing.T, f *framework.E2eFramework, ctx context.Context, kcKey types.NamespacedName, on bool) {
	t.Logf("toggle authentication %v", on)
	var kc v1alpha1.K8ssandraCluster
	err := f.Client.Get(ctx, kcKey, &kc)
	require.NoError(t, err, "failed to get K8ssandraCluster %v", kcKey)
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Auth = pointer.Bool(on)
	err = f.Client.Patch(ctx, &kc, patch)
	require.NoError(t, err, "failed to patch K8ssandraCluster %v", kcKey)
}

func testAuthenticationDisabled(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	namespace string,
	replication map[string]int,
	pod1Name, pod2Name string,
) {
	t.Run("TestJmxAccessAuthDisabled", func(t *testing.T) {
		t.Run("Local", func(t *testing.T) {
			t.Log("check that nodes in different dcs can see each other (auth disabled, local JMX)")
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-0", namespace, pod1Name, 2)
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-1", namespace, pod2Name, 2)
		})
		t.Run("Remote", func(t *testing.T) {
			t.Log("check that nodes in different dcs can see each other (auth disabled, remote JMX)")
			pod1IP, pod2IP := getPodIPs(t, f, namespace, pod1Name, pod2Name)
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-0", namespace, pod1Name, 2, "-h", pod2IP)
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-1", namespace, pod2Name, 2, "-h", pod1IP)
		})
	})
	t.Run("TestApisAuthDisabled", func(t *testing.T) {
		t.Run("Stargate", func(t *testing.T) {
			// Stargate REST APIs are only accessible when PasswordAuthenticator is in use, because when obtaining a new
			// token, the username will be checked against the system_auth.roles table directly.
			// Therefore, we only test the CQL API here.
			// See https://github.com/stargate/stargate/issues/792
			testStargateNativeApi(t, ctx, 0, "", "", replication)
			testStargateNativeApi(t, ctx, 1, "", "", replication)
		})
		t.Run("Reaper", func(t *testing.T) {
			testReaperApi(t, ctx, 0, "cluster1", reaperapi.DefaultKeyspace)
			testReaperApi(t, ctx, 1, "cluster1", reaperapi.DefaultKeyspace)
		})
	})
}

func testAuthenticationEnabled(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	namespace string,
	kcKey types.NamespacedName,
	replication map[string]int,
	pod1Name, pod2Name string,
) {
	t.Log("retrieve superuser credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, kcKey.Namespace, "cluster1")
	require.NoError(t, err, "failed to retrieve superuser credentials")
	t.Run("TestJmxAccessAuthEnabled", func(t *testing.T) {
		t.Run("Local", func(t *testing.T) {
			t.Log("check that nodes in different dcs can see each other (auth enabled, local JMX)")
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-0", namespace, pod1Name, 2, "-u", username, "-pw", password)
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-1", namespace, pod2Name, 2, "-u", username, "-pw", password)
			checkLocalJmxFailsWithNoCredentials(t, f, "kind-k8ssandra-0", namespace, pod1Name)
			checkLocalJmxFailsWithNoCredentials(t, f, "kind-k8ssandra-1", namespace, pod2Name)
			checkLocalJmxFailsWithWrongCredentials(t, f, "kind-k8ssandra-0", namespace, pod1Name)
			checkLocalJmxFailsWithWrongCredentials(t, f, "kind-k8ssandra-1", namespace, pod2Name)
		})
		t.Run("Remote", func(t *testing.T) {
			t.Log("check that nodes in different dcs can see each other (auth enabled, remote JMX)")
			pod1IP, pod2IP := getPodIPs(t, f, namespace, pod1Name, pod2Name)
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-0", namespace, pod1Name, 2, "-h", pod2IP, "-u", username, "-pw", password)
			checkNodeToolStatusUN(t, f, "kind-k8ssandra-1", namespace, pod2Name, 2, "-h", pod1IP, "-u", username, "-pw", password)
			checkRemoteJmxFailsWithNoCredentials(t, f, "kind-k8ssandra-0", namespace, pod1Name, pod2IP)
			checkRemoteJmxFailsWithNoCredentials(t, f, "kind-k8ssandra-1", namespace, pod2Name, pod1IP)
			checkRemoteJmxFailsWithWrongCredentials(t, f, "kind-k8ssandra-0", namespace, pod1Name, pod2IP)
			checkRemoteJmxFailsWithWrongCredentials(t, f, "kind-k8ssandra-1", namespace, pod2Name, pod1IP)
		})
	})
	t.Run("TestApisAuthEnabled", func(t *testing.T) {
		t.Run("Stargate", func(t *testing.T) {
			testStargateApis(t, ctx, "kind-k8ssandra-0", 0, username, password, replication)
			testStargateApis(t, ctx, "kind-k8ssandra-1", 1, username, password, replication)
			checkStargateCqlConnectionFailsWithNoCredentials(t, ctx, 0)
			checkStargateCqlConnectionFailsWithNoCredentials(t, ctx, 1)
			checkStargateCqlConnectionFailsWithWrongCredentials(t, ctx, 0)
			checkStargateCqlConnectionFailsWithWrongCredentials(t, ctx, 1)
			restClient := resty.New()
			checkStargateTokenAuthFailsWithNoCredentials(t, restClient, 0)
			checkStargateTokenAuthFailsWithNoCredentials(t, restClient, 1)
			checkStargateTokenAuthFailsWithWrongCredentials(t, restClient, 0)
			checkStargateTokenAuthFailsWithWrongCredentials(t, restClient, 1)
		})
		t.Run("Reaper", func(t *testing.T) {
			// Note: reaper REST api is currently always unauthenticated
			testReaperApi(t, ctx, 0, "cluster1", reaperapi.DefaultKeyspace)
			testReaperApi(t, ctx, 1, "cluster1", reaperapi.DefaultKeyspace)
		})
	})
}

func checkLocalJmxFailsWithNoCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod string) {
	_, err := f.GetNodeToolStatusUN(k8sContext, namespace, pod)
	if assert.Error(t, err, "expected unauthenticated local JMX connection on pod %v to fail", pod) {
		assert.Contains(t, err.Error(), "Credentials required")
	}
}

func checkLocalJmxFailsWithWrongCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod string) {
	_, err := f.GetNodeToolStatusUN(k8sContext, namespace, pod, "-u", "nonexistent", "-pw", "irrelevant")
	if assert.Error(t, err, "expected local JMX connection with wrong credentials on pod %v to fail", pod) {
		assert.Contains(t, err.Error(), "Invalid username or password")
	}
}

func checkRemoteJmxFailsWithNoCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod, host string) {
	_, err := f.GetNodeToolStatusUN(k8sContext, namespace, pod, "-h", host)
	if assert.Error(t, err, "expected unauthenticated remote JMX connection from pod %v to host %v to fail", pod, host) {
		assert.Contains(t, err.Error(), "Credentials required")
	}
}

func checkRemoteJmxFailsWithWrongCredentials(t *testing.T, f *framework.E2eFramework, k8sContext, namespace, pod, host string) {
	_, err := f.GetNodeToolStatusUN(k8sContext, namespace, pod, "-u", "nonexistent", "-pw", "irrelevant", "-h", host)
	if assert.Error(t, err, "expected remote JMX connection with wrong credentials from pod %v to host %v to fail", pod) {
		assert.Contains(t, err.Error(), "Invalid username or password")
	}
}

func checkStargateCqlConnectionFailsWithNoCredentials(t *testing.T, ctx context.Context, k8sContextIdx int) {
	contactPoint := fmt.Sprintf("stargate.127.0.0.1.nip.io:3%v942", k8sContextIdx)
	cqlClient := cqlclient.NewCqlClient(contactPoint, nil)
	cqlClient.ConnectTimeout = 30 * time.Second
	cqlClient.ReadTimeout = 3 * time.Minute
	_, err := cqlClient.ConnectAndInit(ctx, primitive.ProtocolVersion4, cqlclient.ManagedStreamId)
	if assert.Error(t, err, "expected unauthenticated CQL connection to fail") {
		assert.Contains(t, err.Error(), "expected READY, got AUTHENTICATE")
	}
}

func checkStargateCqlConnectionFailsWithWrongCredentials(t *testing.T, ctx context.Context, k8sContextIdx int) {
	contactPoint := fmt.Sprintf("stargate.127.0.0.1.nip.io:3%v942", k8sContextIdx)
	cqlClient := cqlclient.NewCqlClient(contactPoint, &cqlclient.AuthCredentials{Username: "nonexistent", Password: "irrelevant"})
	cqlClient.ConnectTimeout = 30 * time.Second
	cqlClient.ReadTimeout = 3 * time.Minute
	_, err := cqlClient.ConnectAndInit(ctx, primitive.ProtocolVersion4, cqlclient.ManagedStreamId)
	if assert.Error(t, err, "expected wrong credentials CQL connection to fail") {
		assert.Contains(t, err.Error(), "expected AUTH_CHALLENGE or AUTH_SUCCESS, got ERROR AUTHENTICATION ERROR")
	}
}

func checkStargateTokenAuthFailsWithNoCredentials(t *testing.T, restClient *resty.Client, k8sContextIdx int) {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v1/auth", k8sContextIdx)
	response, err := restClient.NewRequest().Post(url)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode(), "Expected auth request to return 400")
}

func checkStargateTokenAuthFailsWithWrongCredentials(t *testing.T, restClient *resty.Client, k8sContextIdx int) {
	url := fmt.Sprintf("http://stargate.127.0.0.1.nip.io:3%v080/v1/auth", k8sContextIdx)
	body := map[string]string{"username": "nonexistent", "password": "irrelevant"}
	response, err := restClient.NewRequest().SetHeader("Content-Type", "application/json").SetBody(body).Post(url)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, response.StatusCode(), "Expected auth request to return 401")
}

func getPodIPs(t *testing.T, f *framework.E2eFramework, namespace, pod1Name, pod2Name string) (string, string) {
	pod1IP, err := f.GetPodIP("kind-k8ssandra-0", namespace, pod1Name)
	require.NoError(t, err, "failed to get pod %s IP in context kind-k8ssandra-0", pod1Name)
	pod2IP, err := f.GetPodIP("kind-k8ssandra-1", namespace, pod2Name)
	require.NoError(t, err, "failed to get pod %s IP in context kind-k8ssandra-1", pod2Name)
	return pod1IP, pod2IP
}
