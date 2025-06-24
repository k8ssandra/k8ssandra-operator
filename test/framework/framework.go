package framework

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"

	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	logrusr "github.com/bombsimon/logrusr/v2"
	"github.com/go-logr/logr"
	terratestlogger "github.com/gruntwork-io/terratest/modules/logger"
	terratesttesting "github.com/gruntwork-io/terratest/modules/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	casstaskapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	controlapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
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

	err = reaperapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for reaper")

	err = configapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator configs")

	err = replicationapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator replication")

	err = cassdcapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for cass-operator")

	err = casstaskapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for cass-operator tasks")

	err = promapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for prometheus")

	err = medusaapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for medusa")

	err = controlapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for control")

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
	// data plane clients, if the control plane is being deployed in data plane.
	Client client.Client

	// The control plane, that is, the Kubernetes context in which the K8ssandraCluster controller
	// is running.
	ControlPlaneContext string

	// The data planes, that is, the Kubernetes contexts where K8ssandraCluster controller is going
	// to deploy datacenters. There must be at least one data plane defined.
	DataPlaneContexts []string

	// RemoteClients is a mapping of Kubernetes context names to clients. It includes a client for
	// the control plane context as well as clients for all data plane contexts, if any.
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

func NewClusterKey(context, namespace, name string) ClusterKey {
	return ClusterKey{
		K8sContext: context,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func NewFramework(client client.Client, controlPlaneContext string, dataPlaneContexts []string, remoteClients map[string]client.Client) *Framework {
	log := logrusr.New(logrus.New())
	terratestlogger.Default = terratestlogger.New(&terratestLoggerBridge{logger: log})
	return &Framework{
		Client:              client,
		ControlPlaneContext: controlPlaneContext,
		DataPlaneContexts:   dataPlaneContexts,
		remoteClients:       remoteClients,
		logger:              log,
	}
}

func (f *Framework) getRemoteClient(k8sContext string) (client.Client, error) {
	if len(k8sContext) == 0 {
		return nil, fmt.Errorf("k8sContext must be specified")
	}
	remoteClient, found := f.remoteClients[k8sContext]
	if !found {
		return nil, f.k8sContextNotFound(k8sContext)
	}
	return remoteClient, nil
}

// Get fetches the object specified by key from the cluster specified by key. An error is
// returned is ClusterKey.K8sContext is not set or if there is no corresponding client.
func (f *Framework) Get(ctx context.Context, key ClusterKey, obj client.Object) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	return remoteClient.Get(ctx, key.NamespacedName, obj)
}

func (f *Framework) List(ctx context.Context, key ClusterKey, obj client.ObjectList, opts ...client.ListOption) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	opts = append(opts, client.InNamespace(key.Namespace))
	return remoteClient.List(ctx, obj, opts...)
}

func (f *Framework) Update(ctx context.Context, key ClusterKey, obj client.Object) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	return remoteClient.Update(ctx, obj)
}

func (f *Framework) Delete(ctx context.Context, key ClusterKey, obj client.Object) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	return remoteClient.Delete(ctx, obj)
}

func (f *Framework) DeleteAllOf(ctx context.Context, k8sContext string, obj client.Object, opts ...client.DeleteAllOfOption) error {
	remoteClient, err := f.getRemoteClient(k8sContext)
	if err != nil {
		return err
	}
	return remoteClient.DeleteAllOf(ctx, obj, opts...)
}

func (f *Framework) UpdateStatus(ctx context.Context, key ClusterKey, obj client.Object) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	return remoteClient.Status().Update(ctx, obj)
}

func (f *Framework) Patch(ctx context.Context, obj client.Object, patch client.Patch, key ClusterKey, opts ...client.PatchOption) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	return remoteClient.Patch(ctx, obj, patch, opts...)
}

func (f *Framework) PatchStatus(ctx context.Context, obj client.Object, patch client.Patch, key ClusterKey, opts ...client.SubResourcePatchOption) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	return remoteClient.Status().Patch(ctx, obj, patch, opts...)
}

func (f *Framework) Create(ctx context.Context, key ClusterKey, obj client.Object) error {
	remoteClient, err := f.getRemoteClient(key.K8sContext)
	if err != nil {
		return err
	}
	return remoteClient.Create(ctx, obj)
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
			if !errors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	return nil
}

const dns1035LabelFmt string = "[a-z]([-a-z0-9]*[a-z0-9])?"

func CleanupForKubernetes(input string) string {
	if len(validation.IsDNS1035Label(input)) > 0 {
		r := regexp.MustCompile(dns1035LabelFmt)

		// Invalid domain name, Kubernetes will reject this. Try to modify it to a suitable string
		input = strings.ToLower(input)
		input = strings.ReplaceAll(input, "_", "-")
		validParts := r.FindAllString(input, -1)
		return strings.Join(validParts, "")
	}

	return input
}

func (f *Framework) k8sContextNotFound(k8sContext string) error {
	return fmt.Errorf("context %s not found", k8sContext)
}

// PatchK8ssandraCluster fetches the K8ssandraCluster specified by key in the control plane,
// applies changes via updateFn, and then performs a patch operation.
func (f *Framework) PatchK8ssandraCluster(ctx context.Context, key client.ObjectKey, updateFn func(kc *api.K8ssandraCluster)) error {
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, key, kc)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(kc.DeepCopy())
	updateFn(kc)
	return f.Client.Patch(ctx, kc, patch)
}

// SetDatacenterStatusReady fetches the CassandraDatacenter specified by key and persists
// a status update to make the CassandraDatacenter ready. It sets the DatacenterReady and
// DatacenterInitialized conditions to true.
func (f *Framework) SetDatacenterStatusReady(ctx context.Context, key ClusterKey) error {
	now := metav1.Now()

	return f.PatchDatacenterStatus(ctx, key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
		dc.Status.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterInitialized,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
		dc.Status.ObservedGeneration = dc.Generation
	})
}

// SetDeploymentReplicas sets the replicas field of the given deployment to the given value.
func (f *Framework) SetMedusaDeplAvailable(ctx context.Context, key ClusterKey) error {
	return f.PatchDeploymentStatus(ctx, key, func(depl *appsv1.Deployment) {
		// Add a condition to the deployment to indicate that it is available
		now := metav1.Now()
		depl.Status.Conditions = append(depl.Status.Conditions, appsv1.DeploymentCondition{
			Type:               appsv1.DeploymentAvailable,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
		depl.Status.ReadyReplicas = 1
		depl.Status.Replicas = 1
	})
}

func (f *Framework) PatchDeploymentStatus(ctx context.Context, key ClusterKey, updateFn func(depl *appsv1.Deployment)) error {
	depl := &appsv1.Deployment{}
	err := f.Get(ctx, key, depl)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(depl.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(depl)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, depl, patch)
}

// UpdateDatacenterGeneration fetches the CassandraDatacenter specified by key and persists
// a status update to make the ObservedGeneration match the Generation.
// It will wait for .meta.Generation to be different from .status.ObservedGeneration.
// This is done to simulate the behavior of the CassandraDatacenter controller.
func (f *Framework) UpdateDatacenterGeneration(ctx context.Context, t *testing.T, key ClusterKey) bool {
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, key, dc)
	require.NoError(t, err, "failed to get CassandraDatacenter %s/%s", key.Namespace, key.Name)
	if dc.Generation == dc.Status.ObservedGeneration {
		// Expected generation change hasn't happened yet
		return false
	}
	// Generation has changed, update the status accoringly
	return f.PatchDatacenterStatus(ctx, key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.ObservedGeneration = dc.Generation
	}) == nil
}

// SetDatacenterStatusStopped fetches the CassandraDatacenter specified by key and persists
// a status update to make the CassandraDatacenter stopped. It sets the DatacenterStopped and
// DatacenterInitialized conditions to true.
func (f *Framework) SetDatacenterStatusStopped(ctx context.Context, key ClusterKey) error {
	now := metav1.Now()
	return f.PatchDatacenterStatus(ctx, key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterStopped,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
		dc.Status.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterInitialized,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
		dc.Status.ObservedGeneration = dc.Generation
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

func (f *Framework) SetReaperStatusReady(ctx context.Context, key ClusterKey) error {
	return f.PatchReaperStatus(ctx, key, func(r *reaperapi.Reaper) {
		r.Status.Progress = reaperapi.ReaperProgressRunning
		r.Status.SetReady()
	})
}

func (f *Framework) PatchReaperStatus(ctx context.Context, key ClusterKey, updateFn func(r *reaperapi.Reaper)) error {
	r := &reaperapi.Reaper{}
	err := f.Get(ctx, key, r)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(r.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(r)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, r, patch)
}

func (f *Framework) PatchCassandraTaskStatus(ctx context.Context, key ClusterKey, updateFn func(sg *casstaskapi.CassandraTask)) error {
	task := &casstaskapi.CassandraTask{}
	err := f.Get(ctx, key, task)

	if err != nil {
		return err
	}

	patch := client.MergeFromWithOptions(task.DeepCopy(), client.MergeFromWithOptimisticLock{})
	updateFn(task)

	remoteClient := f.remoteClients[key.K8sContext]
	return remoteClient.Status().Patch(ctx, task, patch)
}

// WaitForDeploymentToBeReady Blocks until the Deployment is ready. If
// ClusterKey.K8sContext is empty, this method blocks until the deployment is ready in all
// remote clusters.
func (f *Framework) WaitForDeploymentToBeReady(key ClusterKey, timeout, interval time.Duration) error {
	if len(key.K8sContext) == 0 {
		for k8sContext := range f.remoteClients {
			opts := kubectl.Options{Namespace: key.Namespace, Context: k8sContext}
			err := wait.PollUntilContextTimeout(context.Background(), interval, timeout, true, func(ctx context.Context) (bool, error) {
				if err := kubectl.RolloutStatus(ctx, opts, "Deployment", key.Name); err != nil {
					return false, err
				}
				return true, nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		if _, found := f.remoteClients[key.K8sContext]; !found {
			return f.k8sContextNotFound(key.K8sContext)
		}
		opts := kubectl.Options{Namespace: key.Namespace, Context: key.K8sContext}
		return wait.PollUntilContextTimeout(context.Background(), interval, timeout, true, func(ctx context.Context) (bool, error) {
			if err := kubectl.RolloutStatus(ctx, opts, "Deployment", key.Name); err != nil {
				return false, err
			}
			return true, nil
		})
	}
}

func (f *Framework) DeleteK8ssandraCluster(ctx context.Context, key client.ObjectKey, timeout time.Duration, interval time.Duration) error {
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, key, kc)
	if err != nil {
		return err
	}
	err = f.Client.Delete(ctx, kc)
	if err != nil {
		return err
	}

	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, key, kc)
		return err != nil && errors.IsNotFound(err), nil
	})
}

func (f *Framework) DeleteK8ssandraClusters(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("Deleting K8ssandraClusters", "Namespace", namespace)
	k8ssandra := &api.K8ssandraCluster{}

	// We set the context here so that it correctly fails the DeleteAllOf operation if the context has timed out
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := f.Client.DeleteAllOf(ctx, k8ssandra, client.InNamespace(namespace)); err != nil {
		f.logger.Error(err, "Failed to delete K8ssandraClusters")
		return err
	}

	return wait.PollUntilContextCancel(ctx, interval, true, func(ctx context.Context) (bool, error) {
		list := &api.K8ssandraClusterList{}
		if err := context.Cause(ctx); err != nil {
			f.logger.Error(err, "Failed to delete K8ssandraClusters since context is cancelled in loop")
		}
		if err := f.Client.List(ctx, list, client.InNamespace(namespace)); err != nil {
			f.logger.Info("Waiting for k8ssandracluster deletion", "error", err)
			return false, nil
		}
		f.logger.Info("Waiting for K8ssandraClusters to be deleted", "Namespace", namespace, "Count", len(list.Items))
		return len(list.Items) == 0, nil
	})
}

func (f *Framework) DeleteCassandraDatacenters(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("Deleting CassandraDatacenters", "Namespace", namespace)
	dc := &cassdcapi.CassandraDatacenter{}

	ctx := context.Background()

	if err := f.Client.DeleteAllOf(ctx, dc, client.InNamespace(namespace)); err != nil {
		f.logger.Error(err, "Failed to delete CassandraDatacenters")
	}

	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		list := &cassdcapi.CassandraDatacenterList{}
		err := f.Client.List(ctx, list, client.InNamespace(namespace))
		if err != nil {
			f.logger.Info("Waiting for CassandraDatacenter deletion", "error", err)
			return false, nil
		}
		f.logger.Info("Waiting for CassandraDatacenters to be deleted", "Namespace", namespace, "Count", len(list.Items))
		return len(list.Items) == 0, nil
	})
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

func (f *Framework) MedusaConfigExists(ctx context.Context, k8sContext string, medusaConfigKey ClusterKey) func() bool {
	remoteClient, found := f.remoteClients[k8sContext]
	if !found {
		f.logger.Error(f.k8sContextNotFound(k8sContext), "cannot lookup CassandraDatacenter", "context", k8sContext)
		return func() bool { return false }
	}
	medusaConfig := &medusaapi.MedusaConfiguration{}
	if err := remoteClient.Get(ctx, medusaConfigKey.NamespacedName, medusaConfig); err != nil {
		f.logger.Error(err, "failed to get MedusaConfiguration", "key", medusaConfigKey)
		return func() bool { return false }
	} else {
		return func() bool { return true }
	}
}

// withCassTask Fetches the CassandraTask specified by key and then calls condition.
// func (f *Framework) CassTaskExists(ctx context.Context, key ClusterKey, condition func(task *casstaskapi.CassandraTask) bool) func() bool {
func (f *Framework) CassTaskExists(ctx context.Context, key ClusterKey) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			f.logger.Error(f.k8sContextNotFound(key.K8sContext), "cannot lookup CassandraDatacenter", "key", key)
			return false
		}

		dc := &casstaskapi.CassandraTask{}
		if err := remoteClient.Get(ctx, key.NamespacedName, dc); err == nil {
			f.logger.Info("CassandraTask was found", "key", key)
			return true
		} else {
			if !errors.IsNotFound(err) {
				f.logger.Error(err, "failed to get CassandraTask", "key", key)
			}
			f.logger.Info("CassandraTask not found", "key", key)
			return false
		}
	}
}

// func (f *Framework) CassTaskExists(ctx context.Context, key ClusterKey) func() bool {
// 	withCassTask := f.NewWithCassTask(ctx, key)
// 	return withCassTask(func(dc *casstaskapi.CassandraTask) bool {
// 		return true
// 	})
// }

// NewWithK8ssandraTask is a function generator for withCassandraTask that is bound to ctx, and key.
func (f *Framework) NewWithK8ssandraTask(ctx context.Context, key ClusterKey) func(func(*controlapi.K8ssandraTask) bool) func() bool {
	return func(condition func(dc *controlapi.K8ssandraTask) bool) func() bool {
		return f.withK8ssandraTask(ctx, key, condition)
	}
}

// withK8ssandraTask Fetches the CassandraTask specified by key and then calls condition.
func (f *Framework) withK8ssandraTask(ctx context.Context, key ClusterKey, condition func(task *controlapi.K8ssandraTask) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			f.logger.Error(f.k8sContextNotFound(key.K8sContext), "cannot lookup CassandraDatacenter", "key", key)
			return false
		}

		dc := &controlapi.K8ssandraTask{}
		if err := remoteClient.Get(ctx, key.NamespacedName, dc); err == nil {
			return condition(dc)
		} else {
			if !errors.IsNotFound(err) {
				f.logger.Error(err, "failed to get CassandraTask", "key", key)
			}
			return false
		}
	}
}

func (f *Framework) K8ssandraTaskExists(ctx context.Context, key ClusterKey) func() bool {
	withK8ssandraTask := f.NewWithK8ssandraTask(ctx, key)
	return withK8ssandraTask(func(dc *controlapi.K8ssandraTask) bool {
		return true
	})
}

// // NewWithStargate is a function generator for withStargate that is bound to ctx, and key.
// func (f *Framework) NewWithStargate(ctx context.Context, key ClusterKey) func(func(stargate *stargateapi.Stargate) bool) func() bool {
// 	return func(condition func(*stargateapi.Stargate) bool) func() bool {
// 		return f.withStargate(ctx, key, condition)
// 	}
// }

// withStargate Fetches the stargate specified by key and then calls condition.
// func (f *Framework) withStargate(ctx context.Context, key ClusterKey, condition func(*stargateapi.Stargate) bool) func() bool {
// 	return func() bool {
// 		remoteClient, found := f.remoteClients[key.K8sContext]
// 		if !found {
// 			f.logger.Error(f.k8sContextNotFound(key.K8sContext), "cannot lookup Stargate", "key", key)
// 			return false
// 		}
// 		stargate := &stargateapi.Stargate{}
// 		if err := remoteClient.Get(ctx, key.NamespacedName, stargate); err == nil {
// 			return condition(stargate)
// 		} else {
// 			f.logger.Error(err, "failed to get Stargate", "key", key)
// 			return false
// 		}
// 	}
// }

// func (f *Framework) StargateExists(ctx context.Context, key ClusterKey) func() bool {
// 	withStargate := f.NewWithStargate(ctx, key)
// 	return withStargate(func(s *stargateapi.Stargate) bool {
// 		return true
// 	})
// }

// NewWithReaper is a function generator for withReaper that is bound to ctx, and key.
func (f *Framework) NewWithReaper(ctx context.Context, key ClusterKey) func(func(reaper *reaperapi.Reaper) bool) func() bool {
	return func(condition func(*reaperapi.Reaper) bool) func() bool {
		return f.withReaper(ctx, key, condition)
	}
}

// withReaper Fetches the reaper specified by key and then calls condition.
func (f *Framework) withReaper(ctx context.Context, key ClusterKey, condition func(*reaperapi.Reaper) bool) func() bool {
	return func() bool {
		remoteClient, found := f.remoteClients[key.K8sContext]
		if !found {
			f.logger.Error(f.k8sContextNotFound(key.K8sContext), "cannot lookup Reaper", "key", key)
			return false
		}
		reaper := &reaperapi.Reaper{}
		if err := remoteClient.Get(ctx, key.NamespacedName, reaper); err == nil {
			return condition(reaper)
		} else {
			return false
		}
	}
}

func (f *Framework) ReaperExists(ctx context.Context, key ClusterKey) func() bool {
	withReaper := f.NewWithReaper(ctx, key)
	return withReaper(func(r *reaperapi.Reaper) bool {
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

func (f *Framework) ContainerHasVolumeMount(container corev1.Container, volumeName, volumePath string) bool {
	for _, volume := range container.VolumeMounts {
		if volume.Name == volumeName && volume.MountPath == volumePath {
			return true
		}
	}
	return false
}

func (f *Framework) ContainerHasEnvVar(container corev1.Container, envVarName, envVarValue string) bool {
	for _, envVar := range container.Env {
		if envVar.Name == envVarName && (envVar.Value == envVarValue || envVarValue == "") {
			return true
		}
	}
	return false
}

func (f *Framework) AssertObjectDoesNotExist(ctx context.Context, t *testing.T, key ClusterKey, obj client.Object, timeout, interval time.Duration) {
	assert.Eventually(t, func() bool {
		err := f.Get(ctx, key, obj)
		return err != nil && errors.IsNotFound(err)
	}, timeout, interval, fmt.Sprintf("failed to verify object (%+v) does not exist", key))
}

// NewAllPodsEndpoints simulates the *-all-pods-service Endpoints that lists the nodes of a DC (in real life, this is
// done by the CassandraDatacenter's StatefulSet).
func (f *Framework) NewAllPodsEndpoints(
	kcKey ClusterKey, kc *api.K8ssandraCluster, dcKey ClusterKey, podIp string,
) *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dcKey.Namespace,
			Name:      fmt.Sprintf("%s-%s-all-pods-service", kc.SanitizedName(), dcKey.Name),
			Labels: map[string]string{
				api.K8ssandraClusterNamespaceLabel: kcKey.Namespace,
				api.K8ssandraClusterNameLabel:      kcKey.Name,
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{{IP: podIp}},
				Ports:     []corev1.EndpointPort{{Name: "mock-port", Port: 9042, Protocol: corev1.ProtocolTCP}},
			},
		},
	}
}

func (f *Framework) GetContactPointsService(
	ctx context.Context, kcKey ClusterKey, kc *api.K8ssandraCluster, dcKey ClusterKey,
) (*corev1.Service, *corev1.Endpoints, error) {
	serviceKey := types.NamespacedName{
		Namespace: kcKey.Namespace,
		Name:      fmt.Sprintf("%s-%s-contact-points-service", kc.SanitizedName(), dcKey.Name),
	}
	service := &corev1.Service{}
	err := f.Client.Get(ctx, serviceKey, service)
	if err != nil {
		return nil, nil, err
	}
	endpoints := &corev1.Endpoints{}
	err = f.Client.Get(ctx, serviceKey, endpoints)
	if err != nil {
		return nil, nil, err
	}
	return service, endpoints, nil
}
