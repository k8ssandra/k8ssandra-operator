package medusa

import (
	"context"
	"testing"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var cfg *rest.Config
var testClient client.Client
var testEnv *envtest.Environment
var medusaClientFactory *fakeMedusaClientFactory

const (
	TestCassandraDatacenterName = "dc1"
	requeueAfter                = 2 * time.Second
	timeout                     = time.Second * 3
	interval                    = time.Millisecond * 250
)

func TestControllers(t *testing.T) {
	defer afterSuite(t)

	ctx := testutils.TestSetup(t)
	ctx, cancel := context.WithCancel(ctx)
	testEnv := &testutils.TestEnv{}
	namespace := "default"

	require := require.New(t)

	medusaClientFactory = NewMedusaClientFactory()

	err := testEnv.Start(ctx, t, func(mgr manager.Manager) error {
		err := (&CassandraBackupReconciler{
			ReconcilerConfig: config.InitConfig(),
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
			ClientFactory:    medusaClientFactory,
		}).SetupWithManager(mgr)
		if err != nil {
			return err
		}
		err2 := (&CassandraRestoreReconciler{
			ReconcilerConfig: config.InitConfig(),
			Client:           mgr.GetClient(),
			Scheme:           scheme.Scheme,
		}).SetupWithManager(mgr)
		return err2

	})
	require.NoError(err, "failed to start test environment")

	//require.NoError(registerApis(), "failed to register apis with scheme")

	defer testEnv.Stop(t)
	defer cancel()

	t.Run("Create Datacenter backup", controllerTest(t, namespace, testBackupDatacenter, testEnv.TestClient))
	t.Run("Restore backup in place", controllerTest(t, namespace, testInPlaceRestore, testEnv.TestClient))
}

func afterSuite(t *testing.T) {
	if testEnv != nil {
		err := testEnv.Stop()
		assert.NoError(t, err, "failed to stop test environment")
	}
}

type ControllerTest func(*testing.T, string, client.Client)

func controllerTest(t *testing.T, namespace string, test ControllerTest, testClient client.Client) func(*testing.T) {

	return func(t *testing.T) {
		test(t, namespace, testClient)
	}
}

func deleteNamespace(t *testing.T, ctx context.Context, namespace string) {
	//err := testClient.Delete(ctx, )
}
