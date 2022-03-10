package framework

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/bombsimon/logrusr"
	"github.com/go-logr/logr"
	terratestlogger "github.com/gruntwork-io/terratest/modules/logger"
	terratesttesting "github.com/gruntwork-io/terratest/modules/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
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

	err = promapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for prometheus")

	err = medusaapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for medusa")

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

	// The Kubernetes context in which the K8ssandraCluster controller is running.
	ControlPlaneContext string

	// Extra Kubernetes contexts where K8ssandraCluster controller is not running but datacenters can be deployed. May
	// be empty.
	DataPlaneContexts []string

	// RemoteClients is mapping of Kubernetes context names to clients. It includes a client for the control plane
	// context as well as clients for all data plane contexts, if any.
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
	var log logr.Logger
	log = logrusr.NewLogger(logrus.New())
	terratestlogger.Default = terratestlogger.New(&terratestLoggerBridge{logger: log})
	return &Framework{
		Client:              client,
		ControlPlaneContext: controlPlaneContext,
		DataPlaneContexts:   dataPlaneContexts,
		remoteClients:       remoteClients,
		logger:              log,
	}
}

// AllK8sContexts returns all contexts, including the control plane and all data planes, if any. The control plane is
// always the first element in the returned slice.
func (f *Framework) AllK8sContexts() []string {
	contexts := make([]string, 0, len(f.remoteClients))
	contexts = append(contexts, f.ControlPlaneContext)
	contexts = append(contexts, f.DataPlaneContexts...)
	return contexts
}

// K8sContext returns the context name associated with the given zero-based index. When i = 0, the control plane is
// returned; when i > 0, the corresponding data plane is returned, based on the order in which data planes were
// specified. Note that this way of mapping indices to context names is also honored in e2e tests e.g. by Ingress
// routes, especially when mapping ports to services in different contexts, and so it is important that this mapping is
// not modified accidentally. Also note: this method panics when an invalid index is passed; this is by design since it
// denotes a flaw in test code.
func (f *Framework) K8sContext(k8sContextIdx int) string {
	if k8sContextIdx == 0 {
		return f.ControlPlaneContext
	}
	if k8sContextIdx > len(f.DataPlaneContexts) {
		panic(f.k8sContextNotFound(strconv.Itoa(k8sContextIdx)))
	}
	return f.DataPlaneContexts[k8sContextIdx-1]
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

func (f *Framework) List(ctx context.Context, key ClusterKey, obj client.ObjectList, opts ...client.ListOption) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}
	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
	}
	return remoteClient.List(ctx, obj, opts...)
}

func (f *Framework) Update(ctx context.Context, key ClusterKey, obj client.Object) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}

	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
	}

	return remoteClient.Update(ctx, obj)
}

func (f *Framework) Delete(ctx context.Context, key ClusterKey, obj client.Object) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}

	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
	}

	return remoteClient.Delete(ctx, obj)
}

func (f *Framework) UpdateStatus(ctx context.Context, key ClusterKey, obj client.Object) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}

	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
	}

	return remoteClient.Status().Update(ctx, obj)
}

func (f *Framework) Patch(ctx context.Context, obj client.Object, patch client.Patch, key ClusterKey, opts ...client.PatchOption) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}

	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
	}
	return remoteClient.Patch(ctx, obj, patch, opts...)
}

func (f *Framework) Create(ctx context.Context, key ClusterKey, obj client.Object) error {
	if len(key.K8sContext) == 0 {
		return fmt.Errorf("the K8sContext must be specified for key %s", key)
	}

	remoteClient, found := f.remoteClients[key.K8sContext]
	if !found {
		return fmt.Errorf("no remote client found for context %s", key.K8sContext)
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
			return err
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
	})
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

func (f *Framework) SetStargateStatusReady(ctx context.Context, key ClusterKey) error {
	return f.PatchStargateStatus(ctx, key, func(sg *stargateapi.Stargate) {
		now := metav1.Now()
		sg.Status.Progress = stargateapi.StargateProgressRunning
		sg.Status.AvailableReplicas = 1
		sg.Status.Replicas = 1
		sg.Status.ReadyReplicas = 1
		sg.Status.UpdatedReplicas = 1
		sg.Status.SetCondition(stargateapi.StargateCondition{
			Type:               stargateapi.StargateReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
	})
}

func (f *Framework) PatchStargateStatus(ctx context.Context, key ClusterKey, updateFn func(sg *stargateapi.Stargate)) error {
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

func (f *Framework) DeleteK8ssandraCluster(ctx context.Context, key client.ObjectKey) error {
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, key, kc)
	if err != nil {
		return err
	}
	return f.Client.Delete(ctx, kc)
}

func (f *Framework) DeleteK8ssandraClusters(namespace string, interval, timeout time.Duration) error {
	f.logger.Info("Deleting K8ssandraClusters", "Namespace", namespace)
	k8ssandra := &api.K8ssandraCluster{}

	if err := f.Client.DeleteAllOf(context.TODO(), k8ssandra, client.InNamespace(namespace)); err != nil {
		f.logger.Error(err, "Failed to delete K8ssandraClusters")
		return err
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &api.K8ssandraClusterList{}
		err := f.Client.List(context.Background(), list, client.InNamespace(namespace))
		if err != nil {
			f.logger.Info("Waiting for k8ssandracluster deletion", "error", err)
			return false, nil
		}
		return len(list.Items) == 0, nil
	})
}

func (f *Framework) DeleteCassandraDatacenters(namespace string, interval, timeout time.Duration) error {
	f.logger.Info("Deleting CassandraDatacenters", "Namespace", namespace)
	dc := &cassdcapi.CassandraDatacenter{}

	if err := f.Client.DeleteAllOf(context.Background(), dc, client.InNamespace(namespace)); err != nil {
		f.logger.Error(err, "Failed to delete CassandraDatacenters")
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &cassdcapi.CassandraDatacenterList{}
		err := f.Client.List(context.Background(), list, client.InNamespace(namespace))
		if err != nil {
			f.logger.Info("Waiting for CassandraDatacenter deletion", "error", err)
			return false, nil
		}
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
	return withStargate(func(s *stargateapi.Stargate) bool {
		return true
	})
}

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
