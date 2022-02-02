package config

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
)

const (
	timeout  = time.Second * 5
	interval = time.Millisecond * 500
)

var (
	testEnv     *testutils.MultiClusterTestEnv
	scheme      *runtime.Scheme
	logger      logr.Logger
	cancelCalls int
	// secretFilter map[types.NamespacedName]types.NamespacedName
	reconciler *ClientConfigReconciler
	usedMgr    manager.Manager
	kubeConfig []byte
)

func TestClientConfigReconciler(t *testing.T) {
	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)
	testEnv = &testutils.MultiClusterTestEnv{}
	cancelCalls = 0
	shutDownFunc := func() {
		// We don't want to actually call the shutdown in these tests - we just want to know the cancel function was called
		cancelCalls++
	}
	// secretFilter = make(map[types.NamespacedName]types.NamespacedName)
	reconciler = &ClientConfigReconciler{}
	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		scheme = mgr.GetScheme()
		logger = mgr.GetLogger()
		reconciler.ClientCache = clientCache
		reconciler.Scheme = scheme
		// reconciler.secretFilter = secretFilter
		usedMgr = mgr
		return reconciler.SetupWithManager(mgr, shutDownFunc)
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}

	user, err := testEnv.GetControlPlaneEnvTest().ControlPlane.AddUser(envtest.User{
		Name:   "envtest-admin",
		Groups: []string{"system:masters"},
	}, nil)
	if err != nil {
		t.Fatalf("failed to create envtest user: %v", err)
	}

	kubeConfig, err = user.KubeConfig()
	if err != nil {
		t.Fatalf("failed to get envtest kubeconfig: %v", err)
	}

	defer testEnv.Stop(t)
	defer cancel()

	// Secret controller tests
	t.Run("InitClientConfigs", testEnv.ControllerTest(ctx, testInitClientConfigs))
	t.Run("SecretModification", testEnv.ControllerTest(ctx, testSecretModification))
	t.Run("ClientConfigDeletion", testEnv.ControllerTest(ctx, testConfigDeletion))
	t.Run("SecretDeletion", testEnv.ControllerTest(ctx, testSecretDeletion))
}

func insertKubeConfigSecret(ctx context.Context, localClient client.Client, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-kubeconfig-secret",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"kubeconfig": kubeConfig,
		},
	}

	err := localClient.Create(ctx, secret)
	return secret, err
}

func insertClientConfig(ctx context.Context, localClient client.Client, namespace, name, secretName string) (*configapi.ClientConfig, error) {
	clientConfig := &configapi.ClientConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: configapi.ClientConfigSpec{
			KubeConfigSecret: corev1.LocalObjectReference{
				Name: secretName,
			},
		},
	}

	err := localClient.Create(ctx, clientConfig)
	return clientConfig, err
}

func testInitClientConfigs(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	assert := assert.New(t)

	secret, err := insertKubeConfigSecret(ctx, f.Client, namespace)
	assert.NoError(err)

	clientConfig, err := insertClientConfig(ctx, f.Client, namespace, "envtest", secret.Name)
	assert.NoError(err)

	t.Log("Verify initClientConfig loads the clusters correctly")
	clusters, err := reconciler.InitClientConfigs(ctx, usedMgr, namespace)
	assert.NoError(err)
	assert.Equal(1, len(clusters))

	secretKey := types.NamespacedName{Namespace: namespace, Name: secret.Name}
	clientConfigKey := types.NamespacedName{Namespace: namespace, Name: clientConfig.Name}

	t.Log("Verify that the client updated secret and clientConfig with hashes")
	assert.Eventually(func() bool {
		currentConfig := &configapi.ClientConfig{}
		err = f.Client.Get(ctx, clientConfigKey, currentConfig)
		if err != nil {
			return false
		}

		_, found := reconciler.secretFilter[secretKey]
		return metav1.HasAnnotation(currentConfig.ObjectMeta, KubeSecretHashAnnotation) &&
			metav1.HasAnnotation(currentConfig.ObjectMeta, ClientConfigHashAnnotation) &&
			found
	}, timeout, interval)

	t.Log("Create clientConfig which has incorrect context-name")
	_, err = insertClientConfig(ctx, f.Client, namespace, "envtest-failed", secret.Name)
	assert.NoError(err)

	_, err = reconciler.InitClientConfigs(ctx, usedMgr, namespace)
	assert.Error(err)
}

func testSecretModification(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	// Intentionally different from previous test. This wants to test the internal behavior more, without
	// relying on the controller itself creating "necessary" requirements

	assert := assert.New(t)
	reconciler.secretFilter = make(map[types.NamespacedName]types.NamespacedName)

	secret, err := insertKubeConfigSecret(ctx, f.Client, namespace)
	assert.NoError(err)

	clientConfig, err := insertClientConfig(ctx, f.Client, namespace, "kind-k8ssandra-0", secret.Name)
	assert.NoError(err)

	configHash := utils.DeepHashString(clientConfig)
	secretHash := utils.DeepHashString(secret)

	metav1.SetMetaDataAnnotation(&clientConfig.ObjectMeta, KubeSecretHashAnnotation, secretHash)
	metav1.SetMetaDataAnnotation(&clientConfig.ObjectMeta, ClientConfigHashAnnotation, configHash)

	err = f.Client.Update(ctx, clientConfig)
	assert.NoError(err)

	// Now update the secretFilter
	secretKey := types.NamespacedName{Namespace: namespace, Name: secret.Name}
	clientConfigKey := types.NamespacedName{Namespace: namespace, Name: clientConfig.Name}
	reconciler.secretFilter[secretKey] = clientConfigKey

	// Store currentCount of cancelFunc
	currentCount := cancelCalls

	secretCurrent := &corev1.Secret{}
	err = f.Client.Get(ctx, secretKey, secretCurrent)
	assert.NoError(err)

	secretCurrent.Data["new-key"] = []byte("excellent")
	err = f.Client.Update(ctx, secretCurrent)
	assert.NoError(err)

	assert.Eventually(func() bool {
		return cancelCalls > currentCount
	}, timeout, interval)
}

func testConfigDeletion(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	assert := assert.New(t)

	t.Log("Insert clientConfig and secrets, load them to reconciler")
	secret, err := insertKubeConfigSecret(ctx, f.Client, namespace)
	assert.NoError(err)

	clientConfig, err := insertClientConfig(ctx, f.Client, namespace, "envtest", secret.Name)
	assert.NoError(err)

	_, err = reconciler.InitClientConfigs(ctx, usedMgr, namespace)
	assert.NoError(err)

	// Store currentCount of cancelFunc
	currentCount := cancelCalls

	t.Log("Delete ClientConfig and wait for shutdown call")
	err = f.Client.Delete(ctx, clientConfig)
	assert.NoError(err)

	assert.Eventually(func() bool {
		return cancelCalls > currentCount
	}, timeout, interval)
}

func testSecretDeletion(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	assert := assert.New(t)

	t.Log("Insert clientConfig and secrets, load them to reconciler")
	secret, err := insertKubeConfigSecret(ctx, f.Client, namespace)
	assert.NoError(err)

	_, err = insertClientConfig(ctx, f.Client, namespace, "envtest", secret.Name)
	assert.NoError(err)

	_, err = reconciler.InitClientConfigs(ctx, usedMgr, namespace)
	assert.NoError(err)

	// Store currentCount of cancelFunc
	currentCount := cancelCalls

	t.Log("Delete Secret and wait for shutdown call")
	err = f.Client.Delete(ctx, secret)
	assert.NoError(err)

	assert.Eventually(func() bool {
		return cancelCalls > currentCount
	}, timeout, interval)
}
