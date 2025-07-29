package medusa

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandractrl "github.com/k8ssandra/k8ssandra-operator/controllers/k8ssandra"
	secretswebhook "github.com/k8ssandra/k8ssandra-operator/controllers/secrets-webhook"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
)

const (
	timeout  = time.Second * 5
	interval = time.Millisecond * 100
)

var (
	defaultStorageClass = "default"
	seedsResolver       = &fakeSeedsResolver{}
	managementApi       = &testutils.FakeManagementApiFactory{}
	medusaClientFactory = NewMedusaClientFactory()
)

func TestCassandraBackupRestore(t *testing.T) {
	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)

	testEnv := setupMedusaBackupTestEnv(t, ctx)
	defer testEnv.Stop(t)
	t.Run("TestMedusaBackupDatacenter", testEnv.ControllerTest(ctx, testMedusaBackupDatacenter))

	// TODO: Uncomment this when we have a way to test medusa tasks without flakes
	// t.Run("TestMedusaTasks", testEnv.ControllerTest(ctx, testMedusaTasks))

	t.Run("TestMedusaRestoreDatacenter", testEnv.ControllerTest(ctx, testMedusaRestoreDatacenter))
	t.Run("TestValidationErrorStopsRestore", testEnv.ControllerTest(ctx, testValidationErrorStopsRestore))
	t.Run("TestMedusaConfiguration", testEnv.ControllerTest(ctx, testMedusaConfiguration))

	// This cancel is called here to ensure the correct ordering for defer, as testEnv.Stop() calls must be done before the context is cancelled
	defer cancel()
}

func setupMedusaBackupTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv := &testutils.MultiClusterTestEnv{
		NumDataPlanes: 1,
		BeforeTest: func(t *testing.T) {
			managementApi.SetT(t)
			managementApi.UseDefaultAdapter()
		},
		AfterTest: func(t *testing.T) {
			medusaClientFactory.Clear()
		},
	}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 50 * time.Millisecond
	reconcilerConfig.LongDelay = 150 * time.Millisecond

	medusaClientFactory = NewMedusaClientFactory()
	medusaRestoreClientFactory := NewMedusaClientRestoreFactory()

	err := testEnv.Start(ctx, t, func(controlPlaneMgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&k8ssandractrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           controlPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
			Recorder:         controlPlaneMgr.GetEventRecorderFor("cassandrabackup-controller"),
		}).SetupWithManager(controlPlaneMgr, clusters)
		if err != nil {
			return err
		}

		for _, env := range testEnv.GetDataPlaneEnvTests() {
			whServer := webhook.NewServer(webhook.Options{
				Port:    env.WebhookInstallOptions.LocalServingPort,
				Host:    env.WebhookInstallOptions.LocalServingHost,
				CertDir: env.WebhookInstallOptions.LocalServingCertDir,
				TLSOpts: []func(*tls.Config){func(config *tls.Config) {}},
			})

			dataPlaneMgr, err := ctrl.NewManager(env.Config, ctrl.Options{
				Scheme:         scheme.Scheme,
				WebhookServer:  whServer,
				Metrics:        metricsserver.Options{BindAddress: "0"},
				LeaderElection: false,
			})
			if err != nil {
				return err
			}
			if err := (&MedusaBackupJobReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr); err != nil {
				return err
			}

			if err := (&MedusaTaskReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr); err != nil {
				return err
			}

			if err := (&MedusaRestoreJobReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaRestoreClientFactory,
			}).SetupWithManager(dataPlaneMgr); err != nil {
				return err
			}

			if err := (&MedusaConfigurationReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
			}).SetupWithManager(dataPlaneMgr); err != nil {
				return err
			}

			secretswebhook.SetupSecretsInjectorWebhook(dataPlaneMgr)

			go func() {
				err := dataPlaneMgr.Start(ctx)
				if err != nil {
					t.Errorf("failed to start manager: %s", err)
				}
			}()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}
	return testEnv
}

type fakeSeedsResolver struct {
	callback func(dc *cassdcapi.CassandraDatacenter) ([]string, error)
}
