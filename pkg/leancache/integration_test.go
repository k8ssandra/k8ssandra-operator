package leancache_test

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	kcapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/leancache"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// apiCalls counts the GET requests the operator's client sends to the API server, excluding
// the informers' list and watch requests.
type apiCalls struct {
	gets         atomic.Int32
	clusterLists atomic.Int32
}

func (a *apiCalls) wrap(rt http.RoundTripper) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Query().Get("watch") != "true" {
			parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
			// /api/v1/namespaces/<ns>/<resource>/<name> or /apis/<group>/<version>/namespaces/<ns>/<resource>/<name>
			if (parts[0] == "api" && len(parts) == 6) || (parts[0] == "apis" && len(parts) == 7) {
				a.gets.Add(1)
			}
			if strings.HasSuffix(req.URL.Path, "/secrets") && !strings.Contains(req.URL.Path, "/namespaces/") {
				a.clusterLists.Add(1)
			}
		}
		return rt.RoundTrip(req)
	})
}

func TestLeanCacheIntegration(t *testing.T) {
	env := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")}, ErrorIfCRDPathMissing: true}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, env.Stop()) })

	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	admin, err := client.New(cfg, client.Options{Scheme: s})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	for _, ns := range []string{"watched", "outside"} {
		require.NoError(t, admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}))
	}

	managedMeta := func(name, nameLabel string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: "watched", Labels: map[string]string{kcapi.NameLabel: nameLabel}}
	}
	podSpec := corev1.PodSpec{Containers: []corev1.Container{{Name: "cassandra", Image: "test"}}}
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}}
	template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: selector.MatchLabels}, Spec: podSpec}
	managed := []client.Object{
		&corev1.Pod{ObjectMeta: managedMeta("cassandra-pod", "cassandra"), Spec: podSpec},
		&appsv1.StatefulSet{ObjectMeta: managedMeta("cassandra-sts", "cassandra"), Spec: appsv1.StatefulSetSpec{Selector: selector, Template: template}},
		&corev1.Service{ObjectMeta: managedMeta("contact-points", kcapi.NameLabelValue), Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 9042}}}},
		&appsv1.Deployment{ObjectMeta: managedMeta("reaper", kcapi.NameLabelValue), Spec: appsv1.DeploymentSpec{Selector: selector, Template: template}},
		&discoveryv1.EndpointSlice{ObjectMeta: managedMeta("all-pods", "cassandra"), AddressType: discoveryv1.AddressTypeIPv4},
	}
	foreignService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "watched"}, Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}}}
	referenced := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "superuser", Namespace: "watched",
		Annotations: map[string]string{corev1.LastAppliedConfigAnnotation: "keep"}}, Data: map[string][]byte{"password": []byte("original")}}
	unrelated := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "watched"}}
	for _, obj := range append(managed, foreignService, referenced, unrelated) {
		require.NoError(t, admin.Create(ctx, obj))
	}

	cli, calls := startCluster(t, ctx, cfg, s)

	// Objects created by the operators are served from the cache, others are not cached at all
	for _, obj := range managed {
		require.Eventually(t, func() bool {
			return cli.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object)) == nil
		}, 10*time.Second, 20*time.Millisecond, "%T", obj)
	}
	require.True(t, apierrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(foreignService), &corev1.Service{})))
	require.Zero(t, calls.gets.Load(), "managed objects must be read from the cache")

	// A missing optional ConfigMap is answered by the cache
	require.True(t, apierrors.IsNotFound(cli.Get(ctx, client.ObjectKey{Namespace: "watched", Name: "optional"}, &corev1.ConfigMap{})))
	require.Zero(t, calls.gets.Load())

	// Unlabelled Secrets are not cached; the first Read goes live and labels the Secret
	require.True(t, apierrors.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(unrelated), &corev1.Secret{})))
	got := &corev1.Secret{}
	require.NoError(t, leancache.Read(ctx, cli, client.ObjectKeyFromObject(referenced), got))
	require.Equal(t, "original", string(got.Data["password"]))
	require.Equal(t, int32(1), calls.gets.Load())
	stored := &corev1.Secret{}
	require.NoError(t, admin.Get(ctx, client.ObjectKeyFromObject(referenced), stored))
	require.Equal(t, "true", stored.Labels[leancache.WatchedLabel])
	require.Equal(t, "keep", stored.Annotations[corev1.LastAppliedConfigAnnotation])

	// Once the informer has it, reads are served from the cache and see updates
	require.Eventually(t, func() bool {
		return cli.Get(ctx, client.ObjectKeyFromObject(referenced), &corev1.Secret{}) == nil
	}, 10*time.Second, 20*time.Millisecond)
	stored.Data["password"] = []byte("rotated")
	require.NoError(t, admin.Update(ctx, stored))
	require.Eventually(t, func() bool {
		return cli.Get(ctx, client.ObjectKeyFromObject(referenced), got) == nil && string(got.Data["password"]) == "rotated"
	}, 10*time.Second, 20*time.Millisecond)
	calls.gets.Store(0)
	for range 3 {
		require.NoError(t, leancache.Read(ctx, cli, client.ObjectKeyFromObject(referenced), &corev1.Secret{}))
	}
	require.Zero(t, calls.gets.Load(), "warm reads must use the cache")

	// Secrets outside the cached namespaces are an error, not a reason to read live
	require.Error(t, leancache.Read(ctx, cli, client.ObjectKey{Namespace: "outside", Name: "superuser"}, &corev1.Secret{}))
	require.Zero(t, calls.gets.Load())

	require.Zero(t, calls.clusterLists.Load(), "namespace-scoped caches must never list Secrets cluster-wide")
}

// startCluster starts a cluster with the operator's cache configuration, caching the "watched"
// namespace, and counts the API calls its client makes.
func startCluster(t *testing.T, ctx context.Context, cfg *rest.Config, s *runtime.Scheme) (client.Client, *apiCalls) {
	calls := &apiCalls{}
	countedCfg := rest.CopyConfig(cfg)
	countedCfg.WrapTransport = calls.wrap
	c, err := cluster.New(countedCfg, func(o *cluster.Options) {
		o.Scheme = s
		o.Cache = leancache.Options()
		o.Cache.DefaultNamespaces = map[string]cache.Config{"watched": {}}
		o.NewCache = leancache.NewCache
		o.NewClient = leancache.NewClient
		o.Client.Cache = &client.CacheOptions{DisableFor: leancache.DisableFor()}
	})
	require.NoError(t, err)
	cacheCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- c.Start(cacheCtx) }()
	t.Cleanup(func() { stop(); require.NoError(t, <-done) })
	require.True(t, c.GetCache().WaitForCacheSync(ctx))
	return c.GetClient(), calls
}

func TestServiceMonitorScoping(t *testing.T) {
	env := &envtest.Environment{}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, env.Stop()) })

	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, promapi.AddToScheme(s))
	admin, err := client.New(cfg, client.Options{Scheme: s})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	require.NoError(t, admin.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "watched"}}))

	// Started before Prometheus is installed: ServiceMonitors must never get an informer
	cliBefore, callsBefore := startCluster(t, ctx, cfg, s)

	_, err = envtest.InstallCRDs(cfg, envtest.CRDInstallOptions{CRDs: []*apiextensionsv1.CustomResourceDefinition{serviceMonitorCRD()}})
	require.NoError(t, err)

	ours := &promapi.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "ours", Namespace: "watched",
		Labels: map[string]string{kcapi.NameLabel: kcapi.NameLabelValue}}}
	foreign := &promapi.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "watched"}}
	for _, obj := range []client.Object{ours, foreign} {
		require.NoError(t, admin.Create(ctx, obj))
	}

	require.Eventually(t, func() bool {
		return cliBefore.Get(ctx, client.ObjectKeyFromObject(ours), &promapi.ServiceMonitor{}) == nil
	}, 10*time.Second, 50*time.Millisecond)
	gets := callsBefore.gets.Load()
	require.NotZero(t, gets)
	require.NoError(t, cliBefore.Get(ctx, client.ObjectKeyFromObject(ours), &promapi.ServiceMonitor{}))
	require.Equal(t, gets+1, callsBefore.gets.Load(), "ServiceMonitors are read live when the CRD was missing at startup")

	// Started with Prometheus installed: our ServiceMonitors are cached, others are not
	cliAfter, callsAfter := startCluster(t, ctx, cfg, s)
	require.Eventually(t, func() bool {
		return cliAfter.Get(ctx, client.ObjectKeyFromObject(ours), &promapi.ServiceMonitor{}) == nil
	}, 10*time.Second, 50*time.Millisecond)
	require.True(t, apierrors.IsNotFound(cliAfter.Get(ctx, client.ObjectKeyFromObject(foreign), &promapi.ServiceMonitor{})))
	require.Zero(t, callsAfter.gets.Load(), "ServiceMonitors must be read from the cache")
}

// serviceMonitorCRD is a minimal stand-in for the Prometheus operator's ServiceMonitor CRD.
func serviceMonitorCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "servicemonitors.monitoring.coreos.com"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: promapi.SchemeGroupVersion.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: "servicemonitors", Singular: "servicemonitor", Kind: promapi.ServiceMonitorsKind, ListKind: "ServiceMonitorList",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name: promapi.SchemeGroupVersion.Version, Served: true, Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
					Type: "object", XPreserveUnknownFields: ptr.To(true),
				}},
			}},
		},
	}
}
