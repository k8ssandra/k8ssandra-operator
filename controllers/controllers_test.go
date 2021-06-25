package controllers

import (
	"context"
	"github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
	"time"
)

var cfg *rest.Config
var testClient client.Client
var testEnv *envtest.Environment

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
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

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "config", "cass-operator", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	require.NoError(err, "failed to start test environment")

	require.NoError(registerApis(), "failed to register apis with scheme")

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	require.NoError(err, "failed to create controller-runtime manager")

	var log logr.Logger
	log = logrusr.NewLogger(logrus.New())
	logf.SetLogger(log)

	err = (&K8ssandraClusterReconciler{
		Client: k8sManager.GetClient(),
		Scheme: scheme.Scheme,
	}).SetupWithManager(k8sManager)
	require.NoError(err, "Failed to set up K8ssandraClusterReconciler")

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		assert.NoError(t, err, "failed to start manager")
	}()

	testClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(err, "failed to create controller-runtime client")
}

func afterSuite(t *testing.T) {
	if testEnv != nil {
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
	namespace := rand.String(9)
	return func(t *testing.T) {
		f := framework.NewFramework(testClient)

		if err := f.CreateNamespace(namespace); err != nil {
			t.Fatalf("failed to create namespace %s: %v", namespace, err)
		}

		test(t, ctx, f, namespace)
	}
}
