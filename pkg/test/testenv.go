package test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"

	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	secretswebhook "github.com/k8ssandra/k8ssandra-operator/controllers/secrets-webhook"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	controlapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
)

const (
	clustersToCreate          = 3
	clusterProtoName          = "cluster-%d-%s"
	cassOperatorVersion       = "v1.26.0"
	prometheusOperatorVersion = "v0.9.0"
)

type TestEnv struct {
	*envtest.Environment

	TestClient client.Client
}

func (e *TestEnv) Start(ctx context.Context, t *testing.T, initReconcilers func(mgr manager.Manager) error) error {
	// Prevent the metrics listener being created (it binds to 8080 for all testEnvs)

	if err := registerApis(); err != nil {
		return err
	}

	e.Environment = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "build", "crd", "k8ssandra-operator"),
			filepath.Join("..", "..", "build", "crd", "kube-prometheus"),
			filepath.Join("..", "..", "build", "crd", "cass-operator")},
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	if e.Environment.ControlPlane.APIServer == nil {
		e.Environment.ControlPlane.APIServer = &envtest.APIServer{}
	}

	e.Environment.ControlPlane.APIServer.Configure().Append("max-requests-inflight", "2000").
		Append("max-mutating-requests-inflight", "1000")

	cfg, err := e.Environment.Start()
	if err != nil {
		return err
	}

	cfg.QPS = 2000
	cfg.Burst = 10000

	webhookInstallOptions := &e.Environment.WebhookInstallOptions
	whServer := webhook.NewServer(webhook.Options{
		Port:    webhookInstallOptions.LocalServingPort,
		Host:    webhookInstallOptions.LocalServingHost,
		CertDir: webhookInstallOptions.LocalServingCertDir,
		TLSOpts: []func(*tls.Config){func(config *tls.Config) {}},
	})

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme.Scheme,
		WebhookServer:  whServer,
		LeaderElection: false,
		Metrics: server.Options{
			BindAddress: "0",
		},
	})
	if err != nil {
		return err
	}

	if initReconcilers != nil {
		err = initReconcilers(k8sManager)
		if err != nil {
			return err
		}
	}

	e.TestClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return err
	}

	clientCache := clientcache.New(e.TestClient, e.TestClient, scheme.Scheme)
	if err := api.SetupK8ssandraClusterWebhookWithManager(k8sManager, clientCache); err != nil {
		return err
	}
	secretswebhook.SetupSecretsInjectorWebhook(k8sManager)

	go func() {
		err = k8sManager.Start(ctx)
		if err != nil {
			t.Errorf("failed to start manager: %s", err)
		}
	}()

	err = waitForWebhookServer(webhookInstallOptions)

	return err
}

func waitForWebhookServer(webhookInstallOptions *envtest.WebhookInstallOptions) error {
	// wait for the webhook server to get ready
	var err error
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)

	for i := 0; i < 10; i++ {
		// This is eventually - it can take a while for this to start
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		_ = conn.Close()
		return nil
	}

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

	// NumDataPlanes is the number of data planes to create.
	NumDataPlanes int

	BeforeTest func(t *testing.T)

	AfterTest func(t *testing.T)

	controlPlane string
	dataPlanes   []string
}

func (e *MultiClusterTestEnv) Start(ctx context.Context, t *testing.T, initReconcilers func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error) error {

	// if err := prepareCRDs(); err != nil {
	// 	t.Fatalf("failed to prepare CRDs: %s", err)
	// }

	if err := registerApis(); err != nil {
		return err
	}

	e.Clients = make(map[string]client.Client)
	e.testEnvs = make([]*envtest.Environment, 0)
	clustersToCreate := e.NumDataPlanes + 1
	cfgs := make([]*rest.Config, clustersToCreate)
	clusters := make([]cluster.Cluster, 0, clustersToCreate)

	for i := range clustersToCreate {
		clusterName := fmt.Sprintf(clusterProtoName, i, rand.String(6))
		if i == 0 {
			e.controlPlane = clusterName
		} else {
			e.dataPlanes = append(e.dataPlanes, clusterName)
		}
		testEnv := &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "..", "build", "crd", "k8ssandra-operator"),
				filepath.Join("..", "..", "build", "crd", "kube-prometheus"),
				filepath.Join("..", "..", "build", "crd", "cass-operator"),
			},
			ErrorIfCRDPathMissing: true,
			WebhookInstallOptions: envtest.WebhookInstallOptions{
				Paths: []string{filepath.Join("..", "..", "config", "webhook")},
			},
		}
		if testEnv.ControlPlane.APIServer == nil {
			testEnv.ControlPlane.APIServer = &envtest.APIServer{}
		}
		testEnv.ControlPlane.APIServer.Configure().Append("max-requests-inflight", "2000").
			Append("max-mutating-requests-inflight", "1000")

		e.testEnvs = append(e.testEnvs, testEnv)

		cfg, err := testEnv.Start()
		if err != nil {
			return err
		}
		cfg.QPS = 2000
		cfg.Burst = 10000

		testClient, err := client.New(cfg, client.Options{
			Scheme: scheme.Scheme,
		})
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

	webhookInstallOptions := &e.testEnvs[0].WebhookInstallOptions
	whServer := webhook.NewServer(webhook.Options{
		Port:    webhookInstallOptions.LocalServingPort,
		Host:    webhookInstallOptions.LocalServingHost,
		CertDir: webhookInstallOptions.LocalServingCertDir,
		TLSOpts: []func(*tls.Config){func(config *tls.Config) {}},
	})

	k8sManager, err := ctrl.NewManager(cfgs[0], ctrl.Options{
		Scheme:         scheme.Scheme,
		WebhookServer:  whServer,
		LeaderElection: false,
		Metrics: server.Options{
			BindAddress: "0",
		},
	})

	if err != nil {
		return err
	}

	for _, c := range clusters {
		if err = k8sManager.Add(c); err != nil {
			return err
		}
	}

	clientCache := clientcache.New(k8sManager.GetClient(), e.Clients[e.controlPlane], scheme.Scheme)
	for ctxName, cli := range e.Clients {
		clientCache.AddClient(ctxName, cli)
	}

	if initReconcilers != nil {
		if err = initReconcilers(k8sManager, clientCache, clusters); err != nil {
			return err
		}
	}

	if err := api.SetupK8ssandraClusterWebhookWithManager(k8sManager, clientCache); err != nil {
		return err
	}
	secretswebhook.SetupSecretsInjectorWebhook(k8sManager)

	go func() {
		err = k8sManager.Start(ctx)
		if err != nil {
			t.Errorf("failed to start manager: %s", err)
		}
	}()

	err = waitForWebhookServer(webhookInstallOptions)

	return nil
}

func (e *MultiClusterTestEnv) Stop(t *testing.T) {
	for _, testEnv := range e.testEnvs {
		if err := testEnv.Stop(); err != nil {
			// t.Errorf("failed to stop test environment: %s", err)
			t.Logf("failed to stop test environment: %s", err)
		}
	}
}

func (e *MultiClusterTestEnv) GetControlPlaneEnvTest() *envtest.Environment {
	return e.testEnvs[0]
}

func (e *MultiClusterTestEnv) GetDataPlaneEnvTests() []*envtest.Environment {
	return e.testEnvs[1:]
}

type ControllerTest func(*testing.T, context.Context, *framework.Framework, string)

func (e *MultiClusterTestEnv) ControllerTest(ctx context.Context, test ControllerTest) func(*testing.T) {
	return func(t *testing.T) {
		namespace := "ns-" + framework.CleanupForKubernetes(rand.String(9))

		f := framework.NewFramework(e.Clients[e.controlPlane], e.controlPlane, e.dataPlanes, e.Clients)

		if err := f.CreateNamespace(namespace); err != nil {
			t.Fatalf("failed to create namespace %s: %v", namespace, err)
		}

		if e.BeforeTest != nil {
			e.BeforeTest(t)
		}

		test(t, ctx, f, namespace)

		if e.AfterTest != nil {
			e.AfterTest(t)
		}
	}
}

func TestSetup(t *testing.T) context.Context {
	ctx := ctrl.SetupSignalHandler()

	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("failed to created logger: %v", err)
	}
	log := zapr.NewLogger(logger)
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

	promOperatorTargetDir := filepath.Join("..", "..", "build", "crd", "kube-prometheus")
	if err := os.MkdirAll(promOperatorTargetDir, 0755); err != nil {
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
	if err = os.WriteFile(cassOperatorCrdPath, buf.Bytes(), 0644); err != nil {
		return err
	}

	promOperatorCrd := "https://github.com/prometheus-operator/kube-prometheus?ref=" + prometheusOperatorVersion
	buf, err = kustomize.BuildUrl(promOperatorCrd)
	if err != nil {
		return err
	}
	promOperatorCrdPath := filepath.Join(promOperatorTargetDir, "crd.yaml")
	return os.WriteFile(promOperatorCrdPath, buf.Bytes(), 0644)

}

func registerApis() error {
	if err := api.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := cassdcapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := cassctlapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := stargateapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := reaperapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := configapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := replicationapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := promapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := medusaapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := controlapi.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	if err := admissionv1.AddToScheme(scheme.Scheme); err != nil {
		return err
	}

	return nil
}
