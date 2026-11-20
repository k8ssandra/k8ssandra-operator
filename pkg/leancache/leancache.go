// Package leancache bounds the operator's informer caches, following cass-operator's model:
//
//   - Pods, StatefulSets, Deployments, Services and EndpointSlices are cached only when they
//     were created by cass-operator or k8ssandra-operator. These are hot reconcile reads.
//     ServiceMonitors are scoped the same way when their CRD is installed (see NewCache).
//   - Secrets and ConfigMaps are cached only when labelled with WatchedLabel. The operator adds
//     the label to the objects it creates (MarkWatched) and to user objects it references when
//     it first reads them (Read), so unrelated Secrets never enter the cache.
//   - Rarely read types (legacy Endpoints and CronJob cleanup) are read live (DisableFor).
//
// The same options are applied to the local manager and to every remote cluster.
package leancache

import (
	"context"
	"maps"
	"slices"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	"github.com/k8ssandra/cass-operator/pkg/oplabels"
	kcapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WatchedLabel marks Secrets and ConfigMaps the operator reads, which makes them visible to
// the cache. It is deliberately separate from cass-operator's watched label, which
// cass-operator removes once it no longer watches the object.
const WatchedLabel = "k8ssandra.io/watched"

// MarkWatched adds WatchedLabel to an object the operator is about to create or update.
func MarkWatched(obj client.Object) {
	objLabels := obj.GetLabels()
	if objLabels == nil {
		objLabels = make(map[string]string, 1)
	}
	objLabels[WatchedLabel] = "true"
	obj.SetLabels(objLabels)
}

// managedSelector selects the objects created by cass-operator or k8ssandra-operator.
func managedSelector() labels.Selector {
	createdByOperators, err := labels.NewRequirement(kcapi.NameLabel, selection.In, []string{oplabels.NameLabelValue, kcapi.NameLabelValue})
	if err != nil {
		panic(err) // constant input
	}
	return labels.NewSelector().Add(*createdByOperators)
}

// Options returns the cache options shared by the local manager and remote clusters. They are
// used together with NewCache and NewClient.
func Options() cache.Options {
	managed := managedSelector()
	watched := labels.SelectorFromSet(labels.Set{WatchedLabel: "true"})

	return cache.Options{
		DefaultNamespaces: map[string]cache.Config{},
		DefaultTransform:  StripHeavyMetadata(),
		ByObject: map[client.Object]cache.ByObject{
			&corev1.Pod{}:                {Label: managed},
			&appsv1.StatefulSet{}:        {Label: managed},
			&appsv1.Deployment{}:         {Label: managed},
			&corev1.Service{}:            {Label: managed},
			&discoveryv1.EndpointSlice{}: {Label: managed},
			&corev1.Secret{}:             {Label: watched},
			&corev1.ConfigMap{}:          {Label: watched},
		},
	}
}

// DisableFor lists the types that are only read for legacy cleanup. Those objects carry no
// operator labels, so they are read live instead of through a cluster-wide informer.
func DisableFor() []client.Object {
	return []client.Object{
		&corev1.Endpoints{}, //nolint:staticcheck // Legacy cleanup only.
		&batchv1.CronJob{},
	}
}

// NewCache is a cache.NewCacheFunc that also caches ServiceMonitors, scoped like the other
// objects the operators create, when the Prometheus CRD is installed. ByObject types must be
// resolvable when the cache is built, so this is decided at startup.
func NewCache(cfg *rest.Config, opts cache.Options) (cache.Cache, error) {
	installed, err := serviceMonitorsInstalled(opts.Mapper)
	if err != nil {
		return nil, err
	}
	if installed {
		opts.ByObject = maps.Clone(opts.ByObject)
		opts.ByObject[&promapi.ServiceMonitor{}] = cache.ByObject{Label: managedSelector()}
	}
	return cache.New(cfg, opts)
}

// NewClient is a client.NewClientFunc that returns the regular cache-backed client together
// with a live reader for the same cluster. Read and APIReader use the live reader.
// If the Prometheus CRD was not installed at startup, ServiceMonitors are read live, so that
// installing it later does not create an unscoped informer.
func NewClient(cfg *rest.Config, opts client.Options) (client.Client, error) {
	installed, err := serviceMonitorsInstalled(opts.Mapper)
	if err != nil {
		return nil, err
	}
	if !installed && opts.Cache != nil && opts.Scheme != nil && opts.Scheme.Recognizes(serviceMonitorGVK) {
		cacheOpts := *opts.Cache
		cacheOpts.DisableFor = append(slices.Clone(cacheOpts.DisableFor), &promapi.ServiceMonitor{})
		opts.Cache = &cacheOpts
	}
	c, err := client.New(cfg, opts)
	if err != nil {
		return nil, err
	}
	liveOpts := opts
	liveOpts.Cache = nil
	live, err := client.New(cfg, liveOpts)
	if err != nil {
		return nil, err
	}
	return &cachedClient{Client: c, apiReader: live}, nil
}

var serviceMonitorGVK = promapi.SchemeGroupVersion.WithKind(promapi.ServiceMonitorsKind)

func serviceMonitorsInstalled(mapper meta.RESTMapper) (bool, error) {
	if mapper == nil {
		return false, nil
	}
	_, err := mapper.RESTMapping(serviceMonitorGVK.GroupKind(), serviceMonitorGVK.Version)
	if meta.IsNoMatchError(err) {
		return false, nil
	}
	return err == nil, err
}

type cachedClient struct {
	client.Client
	apiReader client.Reader
}

// APIReader returns the live reader of a client created by NewClient. Other clients (uncached
// or fake ones) are returned as is.
func APIReader(c client.Client) client.Reader {
	if cc, ok := c.(*cachedClient); ok {
		return cc.apiReader
	}
	return c
}

// Read gets a Secret or ConfigMap the operator references. It tries the cache first and on
// NotFound reads the API server, then labels the object so later reads are served from the
// cache. The live object is returned while the informer catches up with the label patch.
// This mirrors cass-operator's secretcache.Read.
func Read(ctx context.Context, c client.Client, key client.ObjectKey, obj client.Object) error {
	err := c.Get(ctx, key, obj)
	cc, ok := c.(*cachedClient)
	if !ok || !apierrors.IsNotFound(err) {
		return err
	}
	if err := cc.apiReader.Get(ctx, key, obj); err != nil {
		return err
	}
	return EnsureWatched(ctx, c, obj)
}

// ReadingClient returns a client whose Secret and ConfigMap Gets go through Read. It is meant
// for library code, such as cass-operator's httphelper, that reads referenced Secrets with a
// plain Get.
func ReadingClient(c client.Client) client.Client {
	if _, ok := c.(*cachedClient); !ok {
		return c
	}
	return &readingClient{Client: c}
}

type readingClient struct {
	client.Client
}

func (r *readingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	switch obj.(type) {
	case *corev1.Secret, *corev1.ConfigMap:
		return Read(ctx, r.Client, key, obj)
	}
	return r.Client.Get(ctx, key, obj, opts...)
}

// EnsureWatched patches WatchedLabel onto an existing object if it is missing.
func EnsureWatched(ctx context.Context, c client.Client, obj client.Object) error {
	if obj.GetLabels()[WatchedLabel] == "true" {
		return nil
	}
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	MarkWatched(obj)
	return c.Patch(ctx, obj, patch)
}

// StripHeavyMetadata drops managedFields from every cached object, and the kubectl
// last-applied-configuration annotation from the objects the operators own. That annotation
// embeds the full object, so it dominates cached-object size. It is kept on user inputs
// (custom resources, Secrets, ConfigMaps), since the operator updates some of them and an
// update from a stripped copy would remove the annotation and break kubectl apply.
func StripHeavyMetadata() toolscache.TransformFunc {
	stripManagedFields := cache.TransformStripManagedFields()
	return func(obj interface{}) (interface{}, error) {
		obj, err := stripManagedFields(obj)
		if err != nil {
			return obj, err
		}
		switch m := obj.(type) {
		case *metav1.PartialObjectMetadata, *corev1.Pod, *corev1.Service, *discoveryv1.EndpointSlice,
			*appsv1.Deployment, *appsv1.StatefulSet, *promapi.ServiceMonitor,
			*cassdcapi.CassandraDatacenter, *cassctlapi.CassandraTask, *medusaapi.MedusaBackup:
			delete(m.(metav1.Object).GetAnnotations(), corev1.LastAppliedConfigAnnotation)
		}
		return obj, nil
	}
}
