package framework

import (
	"context"
	"fmt"
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

	//cfg, err := ctrl.GetConfig()
	//require.NoError(t, err, "failed to get *rest.Config")
	//
	//Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	//require.NoError(t, err, "failed to create controller-runtime client")
}

// Framework provides methods for use in both integration and e2e tests.
type Framework struct {
	// Client is the client for the control plane cluster, i.e., the cluster in which the
	// K8ssandraCluster controller is deployed. Note that this may also be one of the
	// remote clusters.
	Client client.Client

	// The Kubernetes context in which the K8ssandraCluser controller is running.
	ControlPlaneContext string

	// RemoteClients is mapping of Kubernetes context names to clients.
	remoteClients map[string]client.Client

	logger logr.Logger
}

type ClusterKey struct {
	types.NamespacedName

	K8sContext string
}

func (k ClusterKey) String() string {
	return k.K8sContext + string(types.Separator) + k.Namespace + string(types.Separator) + k.Name
}

func NewFramework(client client.Client, controlPlanContext string, remoteClients map[string]client.Client) *Framework {
	var log logr.Logger
	log = logrusr.NewLogger(logrus.New())

	// TODO ControlPlaneContext should default to the first context in the kubeconfig file. We should also provide a flag to override it.
	return &Framework{Client: client, ControlPlaneContext: controlPlanContext, remoteClients: remoteClients, logger: log}
}

func (f *Framework) CreateNamespace(name string) error {
	for k8sContext, remoteClient := range f.remoteClients {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		f.logger.Info("creating namespace", "Namespace", name, "Context", k8sContext)
		if err := remoteClient.Create(context.Background(), namespace); err != nil {
			return err
		}
	}

	return nil
}

func (f *Framework) k8sContextNotFound(k8sContext string) error {
	return fmt.Errorf("context %s not found", k8sContext)
}

// WaitForDeploymentToBeReady Blocks until the Deployment is ready. If
// ClusterKey.K8sContext is empty, this method blocks until the deployment is ready in all
// remote clusters.
func (f *Framework) WaitForDeploymentToBeReady(key ClusterKey, timeout, interval time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		if len(key.K8sContext) == 0 {
			for _, remoteClient := range f.remoteClients {
				deployment := &appsv1.Deployment{}
				if err := remoteClient.Get(context.TODO(), key.NamespacedName, deployment); err != nil {
					f.logger.Error(err, "failed to get deployment", "key", key)
					return false, err
				}

				if deployment.Status.Replicas != deployment.Status.ReadyReplicas {
					return false, nil
				}
			}

			return true, nil
		}

		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			return false, f.k8sContextNotFound(key.K8sContext)
		}

		deployment := &appsv1.Deployment{}
		if err := remoteClient.Get(context.TODO(), key.NamespacedName, deployment); err != nil {
			f.logger.Error(err, "failed to get deployment", "key", key)
			return false, err
		}
		return deployment.Status.Replicas == deployment.Status.ReadyReplicas, nil
	})
}

func (f *Framework) DeleteK8ssandraClusters(namespace string) error {
	// TODO This will need to be updated when we add a finalizer in K8ssandraCluster
	// We will want to block until that finalizer is removed.

	f.logger.Info("deleting all K8ssandraClusters", "Namespace", namespace)
	k8ssandra := &api.K8ssandraCluster{}
	return f.Client.DeleteAllOf(context.TODO(), k8ssandra, client.InNamespace(namespace))
}

// NewWithDatacenter is a function generator for withDatacenter that is bound to ctx, and key.
func (f *Framework) NewWithDatacenter(ctx context.Context, key ClusterKey) func(func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func(condition func(dc *cassdcapi.CassandraDatacenter) bool) func() bool {
		return f.withDatacenter(ctx, key, condition)
	}
}

// withDatacenter Fetches the CassandraDatacenter specified by key and then calls condition.
func (f *Framework) withDatacenter(ctx context.Context, key ClusterKey, condition func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			f.logger.Error(f.k8sContextNotFound(key.K8sContext), "cannot lookup CassandraDatacenter", "key", key)
			return false
		}

		dc := &cassdcapi.CassandraDatacenter{}
		if err := remoteClient.Get(ctx, key.NamespacedName, dc); err == nil {
			return condition(dc)
		} else {
			f.logger.Error(err, "failed to get CassandraDatacenter", "key", key)
			return false
		}
	}
}

func (f *Framework) DatacenterExists(ctx context.Context, key ClusterKey) func() bool {
	withDc := f.NewWithDatacenter(ctx, key)
	return withDc(func(dc *cassdcapi.CassandraDatacenter) bool {
		return true
	})
}

// NewWithStargate is a function generator for withStargate that is bound to ctx, and key.
func (f *Framework) NewWithStargate(ctx context.Context, key ClusterKey) func(func(stargate *api.Stargate) bool) func() bool {
	return func(condition func(*api.Stargate) bool) func() bool {
		return f.withStargate(ctx, key, condition)
	}
}

// withStargate Fetches the stargate specified by key and then calls condition.
func (f *Framework) withStargate(ctx context.Context, key ClusterKey, condition func(*api.Stargate) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			f.logger.Error(f.k8sContextNotFound(key.K8sContext), "cannot lookup Stargate", "key", key)
			return false
		}
		stargate := &api.Stargate{}
		if err := remoteClient.Get(ctx, key.NamespacedName, stargate); err == nil {
			return condition(stargate)
		} else {
			f.logger.Error(err, "failed to get Stargate", "key", key)
			return false
		}
	}
}

func (f *Framework) StargateExists(ctx context.Context, key ClusterKey) func() bool {
	withStargate := f.NewWithStargate(ctx, key)
	return withStargate(func(dc *api.Stargate) bool {
		return true
	})
}
