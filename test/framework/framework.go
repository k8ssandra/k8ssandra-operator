package framework

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	terratestlogger "github.com/gruntwork-io/terratest/modules/logger"
	terratesttesting "github.com/gruntwork-io/terratest/modules/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	err = stargateapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for stargate")

	err = configapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator configs")

	err = replicationapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator replication")

	err = cassdcapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for cass-operator")

	// cfg, err := ctrl.GetConfig()
	// require.NoError(t, err, "failed to get *rest.Config")
	//
	// Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	// require.NoError(t, err, "failed to create controller-runtime client")
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
	terratestlogger.Default = terratestlogger.New(&terratestLoggerBridge{logger: log})

	// TODO ControlPlaneContext should default to the first context in the kubeconfig file. We should also provide a flag to override it.
	return &Framework{Client: client, ControlPlaneContext: controlPlanContext, remoteClients: remoteClients, logger: log}
}

// Get fetches the object specified by key from the cluster specified by key. An error is
// returned is ClusterKey.K8sContext is not set or if there is no corresponding client.
func (f *Framework) Get(ctx context.Context, key ClusterKey, obj client.Object) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}

	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
	}

	return remoteClient.Get(ctx, key.NamespacedName, obj)
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

// SetDatacenterStatusReady fetches the CassandraDatacenter specified by key and persists
// a status update to make the CassandraDatacenter ready.
func (f *Framework) SetDatacenterStatusReady(ctx context.Context, key ClusterKey) error {
	now := metav1.Now()
	return f.PatchDatacenterStatus(ctx, key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
	})
}

// PatchDatacenterStatus fetches the datacenter specified by key, applies changes via
// updateFn, and then performs a patch operation. key.K8sContext must be set and must
// have a corresponding client.
func (f *Framework) PatchDatacenterStatus(ctx context.Context, key ClusterKey, updateFn func(dc *cassdcapi.CassandraDatacenter)) error {
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, key, dc)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(dc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(dc)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, dc, patch)
}

func (f *Framework) PatchStagateStatus(ctx context.Context, key ClusterKey, updateFn func(sg *stargateapi.Stargate)) error {
	sg := &stargateapi.Stargate{}
	err := f.Get(ctx, key, sg)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(sg.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(sg)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, sg, patch)
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
			if !errors.IsNotFound(err) {
				// We won't log the error if its not found because that is expected and it helps cut
				// down on the verbosity of the test output.
				f.logger.Error(err, "failed to get CassandraDatacenter", "key", key)
			}
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
func (f *Framework) NewWithStargate(ctx context.Context, key ClusterKey) func(func(stargate *stargateapi.Stargate) bool) func() bool {
	return func(condition func(*stargateapi.Stargate) bool) func() bool {
		return f.withStargate(ctx, key, condition)
	}
}

// withStargate Fetches the stargate specified by key and then calls condition.
func (f *Framework) withStargate(ctx context.Context, key ClusterKey, condition func(*stargateapi.Stargate) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			f.logger.Error(f.k8sContextNotFound(key.K8sContext), "cannot lookup Stargate", "key", key)
			return false
		}
		stargate := &stargateapi.Stargate{}
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
	return withStargate(func(dc *stargateapi.Stargate) bool {
		return true
	})
}

type terratestLoggerBridge struct {
	logger logr.Logger
}

func (c *terratestLoggerBridge) Logf(t terratesttesting.TestingT, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	c.logger.Info(msg)
}
