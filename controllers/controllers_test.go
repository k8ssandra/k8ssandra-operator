package controllers

import (
	"context"
	"github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
)

var cfg *rest.Config
var testClient client.Client
var testEnv *envtest.Environment

func TestControllers(t *testing.T) {
	defer afterSuite(t)
	beforeSuite(t)

	ctx := context.Background()
	namespace := "default"

	t.Run("Create Datacenter", controllerTest(t, ctx, namespace, createDatacenter))
}

func beforeSuite(t *testing.T) {
	require := require.New(t)

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "build", "config", "crds")},
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

type ControllerTest func(*testing.T, context.Context, string)

func controllerTest(t *testing.T, ctx context.Context, namespace string, test ControllerTest) func(*testing.T) {
	//err := testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	//require.NoError(t, err, "failed to create namespace")

	return func(t *testing.T) {
		test(t, ctx, namespace)
	}
}
