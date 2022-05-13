package medusa

import (
	"context"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandractrl "github.com/k8ssandra/k8ssandra-operator/controllers/k8ssandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
)

const (
	timeout  = time.Second * 5
	interval = time.Millisecond * 500
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
	testEnv1 := setupBackupTestEnv(t, ctx)
	defer testEnv1.Stop(t)
	t.Run("TestBackupDatacenter", testEnv1.ControllerTest(ctx, testBackupDatacenter))

	testEnv2 := setupRestoreTestEnv(t, ctx)
	defer testEnv2.Stop(t)
	t.Run("TestRestoreDatacenter", testEnv2.ControllerTest(ctx, testInPlaceRestore))

	testEnv3 := setupMedusaBackupTestEnv(t, ctx)
	defer testEnv3.Stop(t)
	t.Run("TestMedusaBackupDatacenter", testEnv3.ControllerTest(ctx, testMedusaBackupDatacenter))

	testEnv4 := setupMedusaTaskTestEnv(t, ctx)
	defer testEnv4.Stop(t)
	t.Run("TestMedusaTasks", testEnv4.ControllerTest(ctx, testMedusaTasks))

	testEnv5 := setupMedusaRestoreJobTestEnv(t, ctx)
	defer testEnv5.Stop(t)
	defer cancel()
	t.Run("TestMedusaRestoreDatacenter", testEnv5.ControllerTest(ctx, testMedusaRestoreDatacenter))
}

func setupBackupTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv := &testutils.MultiClusterTestEnv{
		NumDataPlanes: 1,
		BeforeTest: func(t *testing.T) {
			managementApi.SetT(t)
			managementApi.UseDefaultAdapter()
		},
	}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()
	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	err := testEnv.Start(ctx, t, func(controlPlaneMgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&k8ssandractrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           controlPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
		}).SetupWithManager(controlPlaneMgr, clusters)
		if err != nil {
			return err
		}
		for _, env := range testEnv.GetDataPlaneEnvTests() {
			dataPlaneMgr, err := ctrl.NewManager(env.Config, ctrl.Options{Scheme: scheme.Scheme})
			if err != nil {
				return err
			}
			err = (&CassandraBackupReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}
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

func setupRestoreTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv := &testutils.MultiClusterTestEnv{
		NumDataPlanes: 1,
		BeforeTest: func(t *testing.T) {
			managementApi.SetT(t)
			managementApi.UseDefaultAdapter()
		},
	}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	medusaClientFactory = NewMedusaClientFactory()

	err := testEnv.Start(ctx, t, func(controlPlaneMgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&k8ssandractrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           controlPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
		}).SetupWithManager(controlPlaneMgr, clusters)
		if err != nil {
			return err
		}
		for _, env := range testEnv.GetDataPlaneEnvTests() {
			dataPlaneMgr, err := ctrl.NewManager(env.Config, ctrl.Options{Scheme: scheme.Scheme})
			if err != nil {
				return err
			}
			err = (&MedusaTaskReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}

			err = (&CassandraRestoreReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}
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

func setupMedusaBackupTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv := &testutils.MultiClusterTestEnv{
		NumDataPlanes: 1,
		BeforeTest: func(t *testing.T) {
			managementApi.SetT(t)
			managementApi.UseDefaultAdapter()
		},
	}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	medusaClientFactory = NewMedusaClientFactory()

	err := testEnv.Start(ctx, t, func(controlPlaneMgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&k8ssandractrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           controlPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
		}).SetupWithManager(controlPlaneMgr, clusters)
		if err != nil {
			return err
		}

		for _, env := range testEnv.GetDataPlaneEnvTests() {
			dataPlaneMgr, err := ctrl.NewManager(env.Config, ctrl.Options{Scheme: scheme.Scheme})
			if err != nil {
				return err
			}
			err = (&MedusaBackupJobReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}
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

func setupMedusaRestoreJobTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv := &testutils.MultiClusterTestEnv{
		NumDataPlanes: 1,
		BeforeTest: func(t *testing.T) {
			managementApi.SetT(t)
			managementApi.UseDefaultAdapter()
		},
	}

	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	medusaClientFactory = NewMedusaClientFactory()

	err := testEnv.Start(ctx, t, func(controlPlaneMgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&k8ssandractrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           controlPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
		}).SetupWithManager(controlPlaneMgr, clusters)
		if err != nil {
			return err
		}

		for _, env := range testEnv.GetDataPlaneEnvTests() {
			dataPlaneMgr, err := ctrl.NewManager(env.Config, ctrl.Options{Scheme: scheme.Scheme})
			if err != nil {
				return err
			}
			err = (&MedusaTaskReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}

			err = (&MedusaRestoreJobReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}
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

func setupMedusaTaskTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv := &testutils.MultiClusterTestEnv{
		NumDataPlanes: 1,
		BeforeTest: func(t *testing.T) {
			managementApi.SetT(t)
			managementApi.UseDefaultAdapter()
		},
	}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	medusaClientFactory = NewMedusaClientFactory()

	err := testEnv.Start(ctx, t, func(controlPlaneMgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&k8ssandractrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           controlPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
		}).SetupWithManager(controlPlaneMgr, clusters)
		if err != nil {
			return err
		}

		for _, env := range testEnv.GetDataPlaneEnvTests() {
			dataPlaneMgr, err := ctrl.NewManager(env.Config, ctrl.Options{Scheme: scheme.Scheme})
			if err != nil {
				return err
			}
			err = (&MedusaTaskReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}
			err = (&MedusaBackupJobReconciler{
				ReconcilerConfig: reconcilerConfig,
				Client:           dataPlaneMgr.GetClient(),
				Scheme:           scheme.Scheme,
				ClientFactory:    medusaClientFactory,
			}).SetupWithManager(dataPlaneMgr)
			if err != nil {
				return err
			}
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
