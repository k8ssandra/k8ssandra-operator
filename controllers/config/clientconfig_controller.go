package config

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
)

type ClientConfigReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	ClientCache  *clientcache.ClientCache
	shutdownFunc context.CancelFunc
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
	cb := ctrl.NewControllerManagedBy(mgr).
		For(&configapi.ClientConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	return cb.Complete(r)
}
