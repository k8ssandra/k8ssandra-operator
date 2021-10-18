package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bombsimon/logrusr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
)

const (
	clustersToCreate    = 3
	clusterProtoName    = "cluster-%d"
	cassOperatorVersion = "v1.8.0"
)

var (
	controlCluster = fmt.Sprintf(clusterProtoName, 0)
)

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
			filepath.Join("..", "..", "build", "crd", "k8ssandra-operator"),
			filepath.Join("..", "..", "build", "crd", "cass-operator")},
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
	// Clients is a mapping of cluster (or k8s context) names to Client objects. Note that
	// these are no-cache clients  as they are intended for use by the tests.
	Clients map[string]client.Client

	// testEnvs is a list of the test environments that are created
	testEnvs []*envtest.Environment

	clustersToCreate int
}

func (e *MultiClusterTestEnv) Start(ctx context.Context, t *testing.T, initReconcilers func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error) error {
	// Prevent the metrics listener being created (it binds to 8080 for all testEnvs)
	metrics.DefaultBindAddress = "0"

	// if err := prepareCRDs(); err != nil {
	// 	t.Fatalf("failed to prepare CRDs: %s", err)
	// }

	if err := registerApis(); err != nil {
		return err
	}

	e.clustersToCreate = clustersToCreate
	e.Clients = make(map[string]client.Client)
	e.testEnvs = make([]*envtest.Environment, 0)
	cfgs := make([]*rest.Config, e.clustersToCreate)
	clusters := make([]cluster.Cluster, 0, e.clustersToCreate)

	for i := 0; i < e.clustersToCreate; i++ {
		clusterName := fmt.Sprintf(clusterProtoName, i)
		testEnv := &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "..", "build", "crd", "k8ssandra-operator"),
				filepath.Join("..", "..", "build", "crd", "cass-operator"),
			},
			ErrorIfCRDPathMissing: true,
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

	clientCache := clientcache.New(k8sManager.GetClient(), e.Clients[controlCluster], scheme.Scheme)
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

		test(t, ctx, f, namespace)
	}
}

func TestSetup(t *testing.T) context.Context {
	ctx := ctrl.SetupSignalHandler()

	log := logrusr.NewLogger(logrus.New())
	logf.SetLogger(log)

	if err := prepareCRDs(); err != nil {
		t.Fatalf("failed to prepare CRDs: %s", err)
	}

	return ctx
}

// prepareCRDs runs kustomize build over the k8ssandra-operator and cass-operator CRDs and
// writes them to the build/crd directory. This only needs to be call once for the whole
// test suite.
func prepareCRDs() error {
	k8ssandraOperatorTargetDir := filepath.Join("..", "..", "build", "crd", "k8ssandra-operator")
	if err := os.MkdirAll(k8ssandraOperatorTargetDir, 0755); err != nil {
		return err
	}

	cassOperatorTargetDir := filepath.Join("..", "..", "build", "crd", "cass-operator")
	if err := os.MkdirAll(cassOperatorTargetDir, 0755); err != nil {
		return err
	}

	k8ssandraOperatorSrcDir := filepath.Join("..", "..", "config", "crd")

	buf, err := kustomize.BuildDir(k8ssandraOperatorSrcDir)
	if err != nil {
		return err
	}
	k8ssandraOperatorCrdPath := filepath.Join(k8ssandraOperatorTargetDir, "crd.yaml")
	if err = os.WriteFile(k8ssandraOperatorCrdPath, buf.Bytes(), 0644); err != nil {
		return err
	}

	cassOperatorCrd := "github.com/k8ssandra/cass-operator/config/crd?ref=" + cassOperatorVersion
	buf, err = kustomize.BuildUrl(cassOperatorCrd)
	if err != nil {
		return err
	}
	cassOperatorCrdPath := filepath.Join(cassOperatorTargetDir, "crd.yaml")
	return os.WriteFile(cassOperatorCrdPath, buf.Bytes(), 0644)
}

func registerApis() error {
	if err := api.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := cassdcapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := stargateapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := configapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := replicationapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	return nil
}
