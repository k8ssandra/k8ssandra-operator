package controllers

import (
	"context"
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"testing"
	"time"

	"github.com/bombsimon/logrusr"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/sirupsen/logrus"
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
	timeout          = time.Second * 5
	interval         = time.Millisecond * 500
	clusterProtoName = "cluster-%d"
)

var (
	seedsResolver = &fakeSeedsResolver{}
)

func TestControllers(t *testing.T) {
	ctx := ctrl.SetupSignalHandler()

	log := logrusr.NewLogger(logrus.New())
	logf.SetLogger(log)

	if err := os.Setenv(REQUEUE_DEFAULT_DELAY_ENV_VAR, "500ms"); err != nil {
		t.Fatalf("failed to set value for %s env var: %s", REQUEUE_DEFAULT_DELAY_ENV_VAR, err)
	}
	if err := os.Setenv(REQUEUE_LONG_DELAY_ENV_VAR, "1s"); err != nil {
		t.Fatalf("failed to set value for %s env var: %s", REQUEUE_LONG_DELAY_ENV_VAR, err)
	}
	InitConfig()

	t.Run("K8ssandraCluster", func(t *testing.T) {
		kcCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		testK8ssandraCluster(kcCtx, t)
	})

	t.Run("Stargate", func(t *testing.T) {
		stargateCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		testStargate(stargateCtx, t)
	})
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

type TestEnv struct {
	*envtest.Environment

	TestClient client.Client
}

func (e *TestEnv) Start(ctx context.Context, t *testing.T, initReconcilers func(mgr manager.Manager) error) error {
	// Prevent the metrics listener being created (it binds to 8080 for all testEnvs)
	metrics.DefaultBindAddress = "0"

	if err := registerApis(); err != nil {
		return err
	}

	e.Environment = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "config", "cass-operator", "crd", "bases")},
	}

	cfg, err := e.Environment.Start()
	if err != nil {
		return err
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return err
	}

	err = initReconcilers(k8sManager)
	if err != nil {
		return err
	}

	go func() {
		err = k8sManager.Start(ctx)
		if err != nil {
			t.Errorf("failed to start manager: %s", err)
		}
	}()

	e.TestClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	return err
}

func (e *TestEnv) Stop(t *testing.T) {
	if e.Environment != nil {
		err := e.Environment.Stop()
		if err != nil {
			t.Errorf("failed to stop test environment: %s", err)
		}
	}
}

type MultiClusterTestEnv struct {
	// Clients is a mapping of cluster (or k8s context) names to Client objects. This map
	// is used to create the ClientCache as well as to initialize the Framework object
	// for each test.
	Clients map[string]client.Client

	// testEnvs is a list of the test environments that are created
	testEnvs []*envtest.Environment
}

func NewMultiClusterTestEnv() *MultiClusterTestEnv {
	return &MultiClusterTestEnv{
		Clients:  make(map[string]client.Client, 0),
		testEnvs: make([]*envtest.Environment, 0),
	}
}

func (e *MultiClusterTestEnv) Start(ctx context.Context, t *testing.T, initReconcilers func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error) error {
	// Prevent the metrics listener being created (it binds to 8080 for all testEnvs)
	metrics.DefaultBindAddress = "0"

	if err := registerApis(); err != nil {
		return err
	}

	e.Clients = make(map[string]client.Client, 0)
	e.testEnvs = make([]*envtest.Environment, 0)
	cfgs := make([]*rest.Config, clustersToCreate)
	clusters := make([]cluster.Cluster, 0, clustersToCreate)

	for i := 0; i < clustersToCreate; i++ {
		clusterName := fmt.Sprintf(clusterProtoName, i)
		testEnv := &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "config", "crd", "bases"),
				filepath.Join("..", "config", "cass-operator", "crd", "bases")},
			ErrorIfCRDPathMissing:    true,
		}

		e.testEnvs = append(e.testEnvs, testEnv)

		cfg, err := testEnv.Start()
		if err != nil {
			return err
		}

		testClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
		if err != nil {
			return err
		}

		e.Clients[clusterName] = testClient
		cfgs[i] = cfg

		c, err := cluster.New(cfg, func(o *cluster.Options) {
			o.Scheme = scheme.Scheme
		})
		if err != nil {
			return err
		}
		clusters = append(clusters, c)
	}

	k8sManager, err := ctrl.NewManager(cfgs[0], ctrl.Options{
		Scheme: scheme.Scheme,
	})

	if err != nil {
		return err
	}

	for _, c := range clusters {
		if err = k8sManager.Add(c); err != nil {
			return err
		}
	}

	clientCache := clientcache.New(k8sManager.GetClient(), e.Clients["cluster-0"], scheme.Scheme)
	for ctxName, cli := range e.Clients {
		clientCache.AddClient(ctxName, cli)
	}

	if err = initReconcilers(k8sManager, clientCache, clusters); err != nil {
		return err
	}

	go func() {
		err = k8sManager.Start(ctx)
		if err != nil {
			t.Errorf("failed to start manager: %s", err)
		}
	}()

	return nil
}

func (e *MultiClusterTestEnv) Stop(t *testing.T) {
	for _, testEnv := range e.testEnvs {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop test environment: %s", err)
		}
	}
}

type ControllerTest func(*testing.T, context.Context, *framework.Framework, string)

func (e *MultiClusterTestEnv) ControllerTest(ctx context.Context, test ControllerTest) func(*testing.T) {
	namespace := rand.String(9)
	return func(t *testing.T) {
		primaryCluster := fmt.Sprintf(clusterProtoName, 0)
		controlPlaneCluster := e.Clients[primaryCluster]
		f := framework.NewFramework(controlPlaneCluster, primaryCluster, e.Clients)

		if err := f.CreateNamespace(namespace); err != nil {
			t.Fatalf("failed to create namespace %s: %v", namespace, err)
		}

		seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
			return []string{}, nil
		}

		test(t, ctx, f, namespace)
	}
}

type fakeSeedsResolver struct {
	callback func(dc *cassdcapi.CassandraDatacenter) ([]string, error)
}

func (r *fakeSeedsResolver) ResolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error) {
	return r.callback(dc)
}
