package controllers

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	"testing"
	"time"

	"github.com/bombsimon/logrusr"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	clustersToCreate = 2
	timeout          = time.Second * 30
	interval         = time.Second * 1
	clusterProtoName = "cluster-%d"
)

var (
	testClients   = make(map[string]client.Client, clustersToCreate)
	testEnvs      = make([]*envtest.Environment, clustersToCreate)
	seedsResolver = &fakeSeedsResolver{}
)

func TestControllers(t *testing.T) {
	defer afterSuite(t)
	beforeSuite(t)

	ctx := context.Background()

	t.Run("CreateSingleDcCluster", controllerTest(ctx, createSingleDcCluster))
	t.Run("CreateMultiDcCluster", controllerTest(ctx, createMultiDcCluster))
	t.Run("TestStargate", testStargate)
}

func beforeSuite(t *testing.T) {
	require := require.New(t)

	log := logrusr.NewLogger(logrus.New())
	logf.SetLogger(log)

	// Prevent the metrics listener being created (it binds to 8080 for all testEnvs)
	metrics.DefaultBindAddress = "0"

	require.NoError(registerApis(), "failed to register apis with scheme")

	cfgs := make([]*rest.Config, clustersToCreate)
	clientsCache := &testClientCache{clients: make(map[string]client.Client, 0)}

	for i := 0; i < clustersToCreate; i++ {
		clusterName := fmt.Sprintf(clusterProtoName, i)
		testEnv := &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "config", "crd", "bases"),
				filepath.Join("..", "config", "cass-operator", "crd", "bases")},
		}

		testEnvs[i] = testEnv

		cfg, err := testEnv.Start()
		require.NoError(err, "failed to start test environment")
		testClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
		require.NoError(err, "failed to create controller-runtime client")

		clientsCache.clients[clusterName] = testClient

		testClients[clusterName] = testClient
		cfgs[i] = cfg
	}

	k8sManager, err := ctrl.NewManager(cfgs[0], ctrl.Options{
		Scheme: scheme.Scheme,
	})
	require.NoError(err, "failed to create controller-runtime manager")

	clientCache := clientcache.New(k8sManager.GetClient(), testClients["cluster-0"], scheme.Scheme)
	for ctxName, cli := range testClients {
		clientCache.AddClient(ctxName, cli)
	}

	additionalClusters := make([]cluster.Cluster, 0, clustersToCreate-1)

	// We start only one reconciler, for the clusters number 0
	err = (&K8ssandraClusterReconciler{
		Client:        k8sManager.GetClient(),
		Scheme:        scheme.Scheme,
		ClientCache:   clientsCache,
		SeedsResolver: seedsResolver,
	}).SetupWithManager(k8sManager, additionalClusters)
	require.NoError(err, "Failed to set up K8ssandraClusterReconciler with multicluster test")

	err = (&StargateReconciler{
		Client: k8sManager.GetClient(),
		Scheme: scheme.Scheme,
	}).SetupWithManager(k8sManager)
	require.NoError(err, "Failed to set up StargateReconciler")

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		assert.NoError(t, err, "failed to start manager")
	}()
}

func afterSuite(t *testing.T) {
	for _, testEnv := range testEnvs {
		err := testEnv.Stop()
		assert.NoError(t, err, "failed to stop test environment")
	}
}

func registerApis() error {
	if err := api.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := cassdcapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	return nil
}

type ControllerTest func(*testing.T, context.Context, *framework.Framework, string)

func controllerTest(ctx context.Context, test ControllerTest) func(*testing.T) {
	namespace := rand.String(9)
	return func(t *testing.T) {
		primaryCluster := fmt.Sprintf(clusterProtoName, 0)
		controlPlaneCluster := testClients[primaryCluster]
		f := framework.NewFramework(controlPlaneCluster, primaryCluster, testClients)

		if err := f.CreateNamespace(namespace); err != nil {
			t.Fatalf("failed to create namespace %s: %v", namespace, err)
		}

		seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
			return []string{}, nil
		}

		test(t, ctx, f, namespace)
	}
}

type testClientCache struct {
	// Maps k8s context to client. The real impl maps the K8ssandraCluster key to a
	// map[string]client.Client where the key is the k8s context name. I am keeping
	// the mapping here simple since the real impl is likely going to get a complete
	// make over.
	clients map[string]client.Client
}

func (c *testClientCache) GetClient(nsName types.NamespacedName, contextsSecret, k8sContextName string) (client.Client, error) {
	remoteClient, _ := c.clients[k8sContextName]
	return remoteClient, nil
}

type fakeSeedsResolver struct {
	callback func(dc *cassdcapi.CassandraDatacenter) ([]string, error)
}

func (r *fakeSeedsResolver) ResolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error) {
	return r.callback(dc)
}
