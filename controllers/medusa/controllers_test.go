package medusa

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassimages "github.com/k8ssandra/cass-operator/pkg/images"
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
	t.Run("TestMedusaTasks", testEnv.ControllerTest(ctx, testMedusaTasks))
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
			ImageRegistry:    getTestImageRegistry(),
		}).SetupWithManager(controlPlaneMgr, clusters)
		if err != nil {
			return err
		}
		return nil
	}, func(dataPlaneMgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		if err := (&MedusaBackupJobReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           dataPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientFactory:    medusaClientFactory,
			ImageRegistry:    getTestImageRegistry(),
		}).SetupWithManager(dataPlaneMgr); err != nil {
			return err
		}

		if err := (&MedusaTaskReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           dataPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientFactory:    medusaClientFactory,
			ImageRegistry:    getTestImageRegistry(),
		}).SetupWithManager(dataPlaneMgr); err != nil {
			return err
		}

		if err := (&MedusaRestoreJobReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           dataPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientFactory:    medusaRestoreClientFactory,
			ImageRegistry:    getTestImageRegistry(),
		}).SetupWithManager(dataPlaneMgr); err != nil {
			return err
		}

		if err := (&MedusaConfigurationReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           dataPlaneMgr.GetClient(),
			Scheme:           scheme.Scheme,
			ImageRegistry:    getTestImageRegistry(),
		}).SetupWithManager(dataPlaneMgr); err != nil {
			return err
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

var (
	regOnce           sync.Once
	imageRegistryTest cassimages.ImageRegistry
)

func getTestImageRegistry() cassimages.ImageRegistry {
	regOnce.Do(func() {
		p := filepath.Clean("../../test/testdata/imageconfig/image_config_test.yaml")
		data, err := os.ReadFile(p)
		if err == nil {
			if r, e := cassimages.NewImageRegistryV2(data); e == nil {
				imageRegistryTest = r
			}
		}
	})
	return imageRegistryTest
}
