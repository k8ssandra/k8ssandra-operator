package medusa

import (
	"context"
	ctrl "sigs.k8s.io/controller-runtime"
	"testing"
	"time"

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
	defaultStorageClass  = "default"
	medusaClientFactory  = NewMedusaClientFactory()
	managementApiFactory = &testutils.FakeManagementApiFactory{}
)

func TestMedusaBackupRestore(t *testing.T) {

	ctx, cancel := context.WithCancel(testutils.TestSetup(t))
	testEnv := &testutils.MultiClusterTestEnv{
		NumDataPlanes: 1,
		BeforeTest: func(t *testing.T) {
			managementApiFactory.SetT(t)
			managementApiFactory.UseDefaultAdapter()
		},
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
			ManagementApi:    managementApiFactory,
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

	defer testEnv.Stop(t)
	defer cancel()

	t.Run("TestBackupDatacenter", testEnv.ControllerTest(ctx, testBackupDatacenter))
	t.Run("TestRestoreDatacenter", testEnv.ControllerTest(ctx, testInPlaceRestore))

}
