package config

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

const (
	ClientConfigHashAnnotation = k8ssandraapi.ResourceHashAnnotation
	KubeSecretHashAnnotation   = "k8ssandra.io/secret-hash"
)

type ClientConfigReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	ClientCache  *clientcache.ClientCache
	shutdownFunc context.CancelFunc

	// filterMutex  sync.RWMutex
	secretFilter map[types.NamespacedName]types.NamespacedName
}

/*
	What about situations:
		- Remove clientconfig when the cluster is still added?
			* Cluster properties get updated?
				* Do we accept or not?
			* Could just be a cert-update etc, short-term, long-term ones
		- New ClientConfig doesn't work?
		- Add cluster points to a clientconfig that doesn't exist?
		- What if the ClientConfig's kubeConfig is changed in the secret?
		- What if certs expire? What if client connection fails - how do we detect when we need to do something to clientConfig?
		- Do we load clientConfigs from all watch namespaces (or empty - cluster wide) or just from where operator is installed?
			- Security /  user rights? If a namespace is allowed to add a K8ssandraCluster, but not new Kubernetes clusters to connect to?
*/

/*
	TODO Add a channel that other controllers listen? So that they can add new Watches?
	TODO Expose new cluster loading watchers in each controller?
*/

func (r *ClientConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// SetupWithManager will only set this controller to listen in control plane cluster
func (r *ClientConfigReconciler) SetupWithManager(mgr ctrl.Manager, cancelFunc context.CancelFunc) error {
	r.shutdownFunc = cancelFunc

	// We should only reconcile objects that match the rules
	toMatchingClientConfig := func(secret client.Object) []reconcile.Request {
		requests := []reconcile.Request{}
		if clientConfigName, found := r.secretFilter[types.NamespacedName{Name: secret.GetName(), Namespace: secret.GetNamespace()}]; found {
			requests = append(requests, reconcile.Request{NamespacedName: clientConfigName})
		}
		return requests
	}

	cb := ctrl.NewControllerManagedBy(mgr).
		For(&configapi.ClientConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(toMatchingClientConfig))

	return cb.Complete(r)
}

// InitClientConfigs will fetch clientConfigs from the current cluster (control plane cluster) and create all the required Cluster objects for
// other controllers to use. Not called from SetupWithManager since other controllers need the []cluster.Cluster array
func (r *ClientConfigReconciler) InitClientConfigs(ctx context.Context, mgr ctrl.Manager, watchNamespace string) ([]cluster.Cluster, error) {
	uncachedClient := r.ClientCache.GetLocalNonCacheClient()
	clientConfigs := make([]configapi.ClientConfig, 0)
	namespaces := strings.Split(watchNamespace, ",")

	secretMapper := make(map[types.NamespacedName]types.NamespacedName)

	for _, ns := range namespaces {
		cConfigs := configapi.ClientConfigList{}
		err := uncachedClient.List(ctx, &cConfigs, client.InNamespace(ns))
		if err != nil {
			return nil, err
		}
		clientConfigs = append(clientConfigs, cConfigs.Items...)
	}

	additionalClusters := make([]cluster.Cluster, 0, len(clientConfigs))

	for _, cCfg := range clientConfigs {
		// Calculate hashes
		cCfgName := types.NamespacedName{Name: cCfg.Name, Namespace: cCfg.Namespace}
		secretName := types.NamespacedName{Name: cCfg.Spec.KubeConfigSecret.Name, Namespace: cCfg.Namespace}

		cCfgHash, secretHash, err := calculateHashes(ctx, uncachedClient, cCfg)
		if err != nil {
			return nil, err
		}

		metav1.SetMetaDataAnnotation(&cCfg.ObjectMeta, ClientConfigHashAnnotation, cCfgHash)
		metav1.SetMetaDataAnnotation(&cCfg.ObjectMeta, KubeSecretHashAnnotation, secretHash)

		if err := uncachedClient.Update(ctx, &cCfg); err != nil {
			return nil, err
		}

		// Add the Secret to the cache
		secretMapper[secretName] = cCfgName

		// Create clients and add them to the client cache
		cfg, err := r.ClientCache.GetRestConfig(&cCfg)
		if err != nil {
			return nil, err
		}

		// Add cluster to the manager
		var c cluster.Cluster
		if strings.Contains(watchNamespace, ",") {
			c, err = cluster.New(cfg, func(o *cluster.Options) {
				o.Scheme = r.Scheme
				o.Namespace = ""
				o.NewCache = cache.MultiNamespacedCacheBuilder(namespaces)
			})
		} else {
			c, err = cluster.New(cfg, func(o *cluster.Options) {
				o.Scheme = r.Scheme
				o.Namespace = watchNamespace
			})
		}
		if err != nil {
			return nil, err
		}

		r.ClientCache.AddClient(cCfg.GetContextName(), c.GetClient())

		err = mgr.Add(c)
		if err != nil {
			return nil, err
		}

		additionalClusters = append(additionalClusters, c)
	}

	r.secretFilter = secretMapper
	return additionalClusters, nil
}

func calculateHashes(ctx context.Context, anyClient client.Client, clientCfg configapi.ClientConfig) (string, string, error) {
	secret := &corev1.Secret{}
	secretName := types.NamespacedName{Name: clientCfg.Spec.KubeConfigSecret.Name, Namespace: clientCfg.Namespace}

	if err := anyClient.Get(ctx, secretName, secret); err != nil {
		return "", "", err
	}

	cfgHash := utils.DeepHashString(clientCfg)
	secretHash := utils.DeepHashString(secret)

	return cfgHash, secretHash, nil
}
