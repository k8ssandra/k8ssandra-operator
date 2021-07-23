package controllers

import (
	"context"
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
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	clustersToCreate = 2
	timeout          = time.Second * 10
	interval         = time.Millisecond * 250
)

var (
	cfgs        = make([]*rest.Config, clustersToCreate)
	testClients = make([]client.Client, clustersToCreate)
	testEnvs    = make([]*envtest.Environment, clustersToCreate)
)

func TestControllers(t *testing.T) {
	defer afterSuite(t)
	beforeSuite(t)

	ctx := context.Background()

	t.Run("Create Single DC cluster", controllerTest(ctx, createSingleDcCluster))
	t.Run("Create multi-DC cluster in one namespace", controllerTest(ctx, createMultiDcCluster))
}

func beforeSuite(t *testing.T) {
	require := require.New(t)
	log := logrusr.NewLogger(logrus.New())
	logf.SetLogger(log)

	// Prevent the metrics listener being created (it binds to 8080 for all testEnvs)
	metrics.DefaultBindAddress = "0"

	require.NoError(registerApis(), "failed to register apis with scheme")

	signalCtx := ctrl.SetupSignalHandler()

	k8sManagers := make([]manager.Manager, clustersToCreate)

	for i := 0; i < clustersToCreate; i++ {
		testEnv := &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "config", "crd", "bases"),
				filepath.Join("..", "config", "cass-operator", "crd", "bases")},
		}

		testEnvs[i] = testEnv

		cfg, err := testEnv.Start()
		require.NoError(err, "failed to start test environment")

		k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme.Scheme,
		})
		require.NoError(err, "failed to create controller-runtime manager")

		k8sManagers[i] = k8sManager

		testClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
		require.NoError(err, "failed to create controller-runtime client")

		testClients[i] = testClient
		cfgs[i] = cfg
	}

	// We start only one reconciler, for the clusters number 0
	err := (&K8ssandraClusterReconciler{
		Client: k8sManagers[0].GetClient(),
		Scheme: scheme.Scheme,
	}).SetupWithManager(k8sManagers[0])
	require.NoError(err, "Failed to set up K8ssandraClusterReconciler")

	for i := 0; i < clustersToCreate; i++ {
		go func(i int) {
			ctx, cancel := context.WithCancel(signalCtx)
			err = k8sManagers[i].Start(ctx)
			if cancel != nil {
				cancel()
			}
			assert.NoError(t, err, "failed to start manager")
		}(i)
	}

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
	// Test code is temporarily stubbed out until we sort out
	// https://github.com/k8ssandra/k8ssandra-operator/issues/35.

	//namespace := rand.String(9)
	return func(t *testing.T) {
		//f := framework.NewFramework(testClient)
		//
		//if err := f.CreateNamespace(namespace); err != nil {
		//	t.Fatalf("failed to create namespace %s: %v", namespace, err)
		//}
		//
		//test(t, ctx, f, namespace)
	}
}
