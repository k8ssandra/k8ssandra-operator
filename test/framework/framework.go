package framework

import (
	"context"
	"github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

var (
	Client client.Client
)

// TODO Add a Framework type and make functions method on that type
// By making these functions methods we can pass the testing.T and namespace arguments just
// once in the constructor. We can also include defaults for the timeout and interval
// parameters that show up in multiple functions.

func Init(t *testing.T) {
	var err error

	err = api.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator")

	err = cassdcapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for cass-operator")

	cfg, err := ctrl.GetConfig()
	require.NoError(t, err, "failed to get *rest.Config")

	Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err, "failed to create controller-runtime client")
}

type Framework struct {
	Client client.Client

	logger logr.Logger
}

func NewFramework(client client.Client) *Framework {
	var log logr.Logger
	log = logrusr.NewLogger(logrus.New())

	return &Framework{Client: client, logger: log}
}

func (f *Framework) CreateNamespace(name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	f.logger.WithValues("creating namespace", "Namespace", name)
	if err := f.Client.Create(context.Background(), namespace); err != nil {
		return err
	}
	return nil
}

// WaitForDeploymentToBeReady Blocks until the Deployment is ready.
func (f *Framework) WaitForDeploymentToBeReady(key types.NamespacedName, timeout, interval time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		deployment := &appsv1.Deployment{}
		if err := Client.Get(context.TODO(), key, deployment); err != nil {
			f.logger.Error(err, "failed to get deployment", "key", key)
			return false, err
		}
		return deployment.Status.Replicas == deployment.Status.ReadyReplicas, nil
	})
}

func CreateNamespace(t *testing.T, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	t.Logf("creating namespace %s", name)
	if err := Client.Create(context.Background(), namespace); err != nil {
		return err
	}
	return nil
}

func (f *Framework) DeleteK8ssandraClusters(namespace string) error {
	// TODO This will need to be updated when we add a finalizer in K8ssandraCluster
	// We will want to block until that finalizer is removed.

	f.logger.Info("deleting all K8ssandraClusters", "Namespace", namespace)
	k8ssandra := &api.K8ssandraCluster{}
	return Client.DeleteAllOf(context.TODO(), k8ssandra, client.InNamespace(namespace))
}

// NewWithDatacenter is a function generator for withDatacenter that is bound to ctx, and key.
func (f *Framework) NewWithDatacenter(ctx context.Context, key types.NamespacedName) func(func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func(condition func(dc *cassdcapi.CassandraDatacenter) bool) func() bool {
		return f.withDatacenter(ctx, key, condition)
	}
}

// withDatacenter Fetches the CassandraDatacenter specified by key and then calls condition.
func (f *Framework) withDatacenter(ctx context.Context, key types.NamespacedName, condition func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func() bool {
		dc := &cassdcapi.CassandraDatacenter{}
		if err := f.Client.Get(ctx, key, dc); err == nil {
			return condition(dc)
		} else {
			f.logger.Error(err, "failed to get CassandraDatacenter", "key", key)
			return false
		}
	}
}

func (f *Framework) DatacenterExists(ctx context.Context, key types.NamespacedName) func() bool {
	withDc := f.NewWithDatacenter(ctx, key)
	return withDc(func(dc *cassdcapi.CassandraDatacenter) bool {
		return true
	})
}
