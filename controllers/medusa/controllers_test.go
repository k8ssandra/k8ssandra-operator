package medusa

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	ctrl "github.com/k8ssandra/k8ssandra-operator/controllers/k8ssandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	testEnv             *testutils.MultiClusterTestEnv
	seedsResolver       = &fakeSeedsResolver{}
	managementApi       = &fakeManagementApiFactory{}
	medusaClientFactory *fakeMedusaClientFactory
)

func TestMedusaBackupRestore(t *testing.T) {
	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)
	testEnv1 := setupBackupTestEnv(t, ctx)
	defer testEnv1.Stop(t)
	defer cancel()

	t.Run("TestBackupDatacenter", testEnv1.ControllerTest(ctx, testBackupDatacenter))

	testEnv2 := setupRestoreTestEnv(t, ctx)
	defer testEnv2.Stop(t)
	defer cancel()
	t.Run("TestRestoreDatacenter", testEnv2.ControllerTest(ctx, testInPlaceRestore))

}

func setupBackupTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv = &testutils.MultiClusterTestEnv{}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	medusaClientFactory = NewMedusaClientFactory()

	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&ctrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
		}).SetupWithManager(mgr, clusters)
		if err != nil {
			return err
		}
		err = (&CassandraBackupReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientFactory:    medusaClientFactory,
		}).SetupWithManager(mgr)
		return err
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}
	return testEnv
}

func setupRestoreTestEnv(t *testing.T, ctx context.Context) *testutils.MultiClusterTestEnv {
	testEnv = &testutils.MultiClusterTestEnv{}
	seedsResolver.callback = func(dc *cassdcapi.CassandraDatacenter) ([]string, error) {
		return []string{}, nil
	}

	reconcilerConfig := config.InitConfig()

	reconcilerConfig.DefaultDelay = 100 * time.Millisecond
	reconcilerConfig.LongDelay = 300 * time.Millisecond

	medusaClientFactory = NewMedusaClientFactory()

	err := testEnv.Start(ctx, t, func(mgr manager.Manager, clientCache *clientcache.ClientCache, clusters []cluster.Cluster) error {
		err := (&ctrl.K8ssandraClusterReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientCache:      clientCache,
			ManagementApi:    managementApi,
		}).SetupWithManager(mgr, clusters)
		if err != nil {
			return err
		}

		err = (&CassandraRestoreReconciler{
			ReconcilerConfig: reconcilerConfig,
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
		}).SetupWithManager(mgr)
		return err
	})
	if err != nil {
		t.Fatalf("failed to start test environment: %s", err)
	}
	return testEnv
}

type fakeSeedsResolver struct {
	callback func(dc *cassdcapi.CassandraDatacenter) ([]string, error)
}

func (r *fakeSeedsResolver) ResolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error) {
	return r.callback(dc)
}

type fakeManagementApiFactory struct {
}

func (f fakeManagementApiFactory) NewManagementApiFacade(context.Context, *cassdcapi.CassandraDatacenter, client.Client, logr.Logger) (cassandra.ManagementApiFacade, error) {
	m := new(mocks.ManagementApiFacade)
	m.On("EnsureKeyspaceReplication", mock.Anything, mock.Anything).Return(nil)
	m.On("ListTables", stargate.AuthKeyspace).Return([]string{"token"}, nil)
	m.On("CreateTable", mock.MatchedBy(func(def *httphelper.TableDefinition) bool {
		return def.KeyspaceName == stargate.AuthKeyspace && def.TableName == stargate.AuthTable
	})).Return(nil)
	return m, nil
}
