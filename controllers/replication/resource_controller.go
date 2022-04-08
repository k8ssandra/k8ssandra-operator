package replication

import (
	"context"
	"fmt"
	"sync"

	api "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
)

// We need rights to update the target cluster's ConfigMaps, not necessarily this cluster
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=configMaps,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=replication.k8ssandra.io,namespace="k8ssandra",resources=replicatedConfigMaps,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=replication.k8ssandra.io,namespace="k8ssandra",resources=replicatedConfigMaps/finalizers,verbs=update
// +kubebuilder:rbac:groups=replication.k8ssandra.io,namespace="k8ssandra",resources=replicatedConfigMaps/status,verbs=get;update;patch

type ConfigMapSyncController struct {
	*config.ReconcilerConfig
	ClientCache *clientcache.ClientCache
	// TODO We need a better structure for empty selectors (match whole kind)
	WatchNamespaces []string
	selectorMutex   sync.RWMutex
	selectors       map[types.NamespacedName]labels.Selector
}

type SelectorCache struct {
	// Mutex
	// Kind + types.NamespacedName => labels.Selector (ObjectMeta?)
}

func (s *ConfigMapSyncController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	localClient := s.ClientCache.GetLocalClient()

	logger.Info("ConfigMapSyncController::Starting reconciliation", "key", req.NamespacedName)

	// TODO Should not be ReplicatedConfigMap, but ReplicatedResource with enough information to get the type
	rsec := &api.ReplicatedConfigMap{}
	if err := localClient.Get(ctx, req.NamespacedName, rsec); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Deletion and finalizer logic
	if rsec.GetDeletionTimestamp() != nil {
		return s.deletionProcess(ctx, rsec)
	}

	if !controllerutil.ContainsFinalizer(rsec, replicatedResourceFinalizer) {
		controllerutil.AddFinalizer(rsec, replicatedResourceFinalizer)
		if err := localClient.Update(ctx, rsec); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Add the new matcher rules also to our cache if not found
	selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Update the selector in cache always (comparing is pointless)
	s.selectorMutex.Lock()
	s.selectors[req.NamespacedName] = selector
	s.selectorMutex.Unlock()

	// TODO This could be a func in the struct to do fetching

	// Fetch all the ConfigMaps that match the ReplicatedConfigMap's rules
	// configMaps, err := s.fetchAllMatchingConfigMaps(ctx, selector)
	// if err != nil {
	// 	return reconcile.Result{}, err
	// }

	// TODO ConfigMapList should come from a function that matches the known type (or the API type)
	items, err := s.fetchAllMatchingObjects(ctx, selector, &corev1.ConfigMapList{})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Verify objects have up-to-date hashes
	for i := range items {
		sec := items[i]
		if err := verifyHashAnnotation(ctx, s.ClientCache.GetLocalClient(), sec.(client.Object)); err != nil {
			return reconcile.Result{}, err
		}
	}

	// For status updates
	patch := client.MergeFrom(rsec.DeepCopy())
	rsec.Status.Conditions = make([]api.ReplicationCondition, 0, len(rsec.Spec.ReplicationTargets))

	for _, target := range rsec.Spec.ReplicationTargets {
		// Even if ReplicationTarget includes local client - remove it (it will cause errors)
		// Only replicate to clusters that are in the ReplicatedConfigMap's context
		var remoteClient client.Client
		if target.K8sContextName == "" {
			remoteClient = localClient
		} else {
			remoteClient, err = s.ClientCache.GetRemoteClient(target.K8sContextName)
			if err != nil {
				logger.Error(err, "Failed to fetch remote client for managed cluster", "ReplicatedConfigMap", req.NamespacedName, "TargetContext", target)
				return ctrl.Result{Requeue: true}, err
			}
		}

		cond := api.ReplicationCondition{
			Cluster: target.K8sContextName,
			Type:    api.ReplicationDone,
		}

	TargetConfigMaps:
		// Iterate all the matching ConfigMaps
		for i := range items {
			sec := items[i].(client.Object)
			namespace := ""
			if target.Namespace == "" {
				namespace = sec.GetNamespace()
			} else {
				namespace = target.Namespace
			}
			// TODO Need to probably cast here unless we can use some other type..
			fetchedConfigMap := &corev1.ConfigMap{}
			if err = remoteClient.Get(ctx, types.NamespacedName{Name: sec.GetName(), Namespace: namespace}, fetchedConfigMap); err != nil {
				if errors.IsNotFound(err) {
					logger.Info("Copying ConfigMap to target cluster", "ConfigMap", sec.GetName(), "TargetContext", target)
					// Create it
					copiedConfigMap := sec.DeepCopyObject().(client.Object)
					copiedConfigMap.SetNamespace(namespace)
					copiedConfigMap.SetResourceVersion("")
					copiedConfigMap.SetOwnerReferences([]metav1.OwnerReference{})
					if err = remoteClient.Create(ctx, copiedConfigMap); err != nil {
						logger.Error(err, "Failed to sync ConfigMap to target cluster", "ConfigMap", copiedConfigMap.GetName(), "TargetContext", target)
						break TargetConfigMaps
					}
					continue
				}
				logger.Error(err, "Failed to fetch ConfigMap from target cluster", "ConfigMap", fetchedConfigMap.Name, "TargetContext", target)
				break TargetConfigMaps
			}

			if fetchedConfigMap.Immutable != nil && *fetchedConfigMap.Immutable {
				err := fmt.Errorf("target ConfigMap is immutable")
				logger.Error(err, "Failed to modify target ConfigMap, ConfigMap is set to immutable", "ConfigMap", fetchedConfigMap.Name, "TargetContext", target)
				break TargetConfigMaps
			}

			if objectRequiresUpdate(sec, fetchedConfigMap) {
				logger.Info("Modifying ConfigMap in target cluster", "ConfigMap", sec.GetName(), "TargetContext", target)
				// TODO Need sync to have a type here ..
				origConfigMap := sec.(*corev1.ConfigMap)
				syncConfigMaps(origConfigMap, fetchedConfigMap)
				if err = remoteClient.Update(ctx, fetchedConfigMap); err != nil {
					logger.Error(err, "Failed to sync target ConfigMap for matching payloads", "ConfigMap", fetchedConfigMap.Name, "TargetContext", target)
					break TargetConfigMaps
				}
			}
		}
		if err != nil {
			cond.Status = corev1.ConditionFalse
		} else {
			cond.Status = corev1.ConditionTrue
		}

		timeNow := metav1.Now()
		cond.LastTransitionTime = &timeNow
		rsec.Status.Conditions = append(rsec.Status.Conditions, cond)
	}

	// Update the ReplicatedConfigMap's Status
	err = localClient.Status().Patch(ctx, rsec, patch)
	if err != nil {
		logger.Error(err, "Failed to update replicated ConfigMap last transition time", "ReplicatedConfigMap", req.NamespacedName)
		return ctrl.Result{}, err
	}

	// If any cluster had failed state, retry
	for _, cond := range rsec.Status.Conditions {
		if cond.Status == corev1.ConditionFalse {
			return ctrl.Result{}, fmt.Errorf("replication failed")
		}
	}
	return ctrl.Result{}, err
}

// TODO Refactor to make it common with SecretController
// Fetcher is the problematic one, are there any others?
func (s *ConfigMapSyncController) deletionProcess(ctx context.Context, rsec *api.ReplicatedConfigMap) (ctrl.Result, error) {
	localClient := s.ClientCache.GetLocalClient()
	logger := log.FromContext(ctx)

	namespacedName := types.NamespacedName{Name: rsec.Name, Namespace: rsec.Namespace}

	if controllerutil.ContainsFinalizer(rsec, replicatedResourceFinalizer) {
		logger.Info("Starting cleanup")

		// Fetch all ConfigMaps from managed cluster.
		// Remove only those ConfigMaps which are not matched by any other ReplicatedConfigMap and do not have the orphan annotation
		if val, found := rsec.GetAnnotations()[secret.OrphanResourceAnnotation]; !found || val != "true" {
			logger.Info("Cleaning up all the replicated resources", "ReplicatedConfigMap", namespacedName)
			selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
			if err != nil {
				logger.Error(err, "Failed to delete the replicated ConfigMap, defined labels are invalid", "ReplicatedConfigMap", namespacedName)
				return reconcile.Result{}, err
			}

			// TODO Needs a common type here for the ConfigMapList (or a separate fetcher?)
			configMaps, err := s.fetchAllMatchingObjects(ctx, selector, &corev1.ConfigMapList{})
			// configMaps, err := s.fetchAllMatchingConfigMaps(ctx, selector)
			if err != nil {
				logger.Error(err, "Failed to fetch the replicated ConfigMaps to cleanup", "ReplicatedConfigMap", namespacedName)
				return reconcile.Result{}, err
			}

			configMapsToDelete := make([]client.Object, 0, len(configMaps))

			s.selectorMutex.RLock()

		ConfigMapsToCheck:
			for _, sec := range configMaps {
				accessor, err := apimeta.Accessor(sec)
				if err != nil {
					return reconcile.Result{}, err
				}
				key := types.NamespacedName{Name: accessor.GetName(), Namespace: accessor.GetNamespace()}
				logger.Info("Checking ConfigMap", "key", key)
				for k, v := range s.selectors {
					if k.Namespace != key.Namespace {
						logger.Info("Skipping ConfigMap", "key", key, "namespace", k.Namespace)
						continue
					}
					if k == namespacedName {
						// This is the ReplicatedConfigMap that will be deleted, we don't want its rules to match
						continue
					}

					if val, found := accessor.GetAnnotations()[secret.OrphanResourceAnnotation]; found && val == "true" {
						// Managed cluster has orphan set to the ConfigMap, do not delete it from target clusters
						continue ConfigMapsToCheck
					}

					if v.Matches(labels.Set(accessor.GetLabels())) {
						// Another Replication rule is matching this ConfigMap, do not delete it
						logger.Info("Another replication rule matches ConfigMap", "key", key)
						continue ConfigMapsToCheck
					}
				}
				logger.Info("Preparing to delete ConfigMap", "key", key)
				configMapsToDelete = append(configMapsToDelete, sec.DeepCopyObject().(client.Object))
			}

			s.selectorMutex.RUnlock()

			for _, target := range rsec.Spec.ReplicationTargets {
				logger.Info("Deleting ConfigMaps for ReplicationTarget", "Target", target)

				// Only replicate to clusters that are in the ReplicatedConfigMap's context
				remoteClient, err := s.ClientCache.GetRemoteClient(target.K8sContextName)
				if err != nil {
					logger.Error(err, "Failed to fetch remote client for managed cluster", "ReplicatedConfigMap", namespacedName, "TargetContext", target)
					return ctrl.Result{}, err
				}
				for _, deleteKey := range configMapsToDelete {
					logger.Info("Deleting ConfigMap", "key", client.ObjectKeyFromObject(deleteKey),
						"Cluster", target.K8sContextName)
					err = remoteClient.Delete(ctx, deleteKey)
					if err != nil && !errors.IsNotFound(err) {
						logger.Error(err, "Failed to remove ConfigMaps from target cluster", "ReplicatedConfigMap", namespacedName, "TargetContext", target)
						return ctrl.Result{}, err
					}
				}
			}
		}
		s.selectorMutex.Lock()
		delete(s.selectors, namespacedName)
		s.selectorMutex.Unlock()
		controllerutil.RemoveFinalizer(rsec, replicatedResourceFinalizer)
		err := localClient.Update(ctx, rsec)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}
	return ctrl.Result{}, nil
}

// TODO Change type to client.Object etc
func syncConfigMaps(src, dest *corev1.ConfigMap) {
	origMeta := dest.ObjectMeta
	src.DeepCopyInto(dest)
	dest.ObjectMeta = origMeta
	dest.OwnerReferences = []metav1.OwnerReference{}

	syncMetadata(&src.ObjectMeta, &dest.ObjectMeta)
}

func (s *ConfigMapSyncController) fetchAllMatchingConfigMaps(ctx context.Context, selector labels.Selector) ([]corev1.ConfigMap, error) {
	configMaps := &corev1.ConfigMapList{}
	listOption := client.ListOptions{
		LabelSelector: selector,
	}
	err := s.ClientCache.GetLocalClient().List(ctx, configMaps, &listOption)
	if err != nil {
		return nil, err
	}

	return configMaps.Items, nil
}

func (s *ConfigMapSyncController) fetchAllMatchingObjects(ctx context.Context, selector labels.Selector, through client.ObjectList) ([]runtime.Object, error) {
	listOption := client.ListOptions{
		LabelSelector: selector,
	}
	err := s.ClientCache.GetLocalClient().List(ctx, through, &listOption)
	if err != nil {
		return nil, err
	}

	allItems, err := apimeta.ExtractList(through)
	return allItems, err
}

func (s *ConfigMapSyncController) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	err := s.initializeCache()
	if err != nil {
		return err
	}

	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.ReplicatedConfigMap{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, handler.EnqueueRequestsFromMapFunc(s.replicaMatcher))

	for _, c := range clusters {
		cb = cb.Watches(
			source.NewKindWithCache(&corev1.ConfigMap{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(s.replicaMatcher))
	}

	return cb.Complete(s)
}

// TODO Duplicate code with SecretController
func (s *ConfigMapSyncController) replicaMatcher(configMap client.Object) []reconcile.Request {
	requests := []reconcile.Request{}
	s.selectorMutex.RLock()
	for k, v := range s.selectors {
		if v.Matches(labels.Set(configMap.GetLabels())) {
			requests = append(requests, reconcile.Request{NamespacedName: k})
		}
	}
	s.selectorMutex.RUnlock()
	return requests
}

// TODO Duplicate code with secretController
func (s *ConfigMapSyncController) initializeCache() error {
	s.selectors = make(map[types.NamespacedName]labels.Selector)
	localClient := s.ClientCache.GetLocalNonCacheClient()

	for _, namespace := range s.WatchNamespaces {
		var err error
		replicatedConfigMaps := api.ReplicatedConfigMapList{}
		opts := make([]client.ListOption, 0, 1)
		if namespace != "" {
			opts = append(opts, client.InNamespace(namespace))
		}
		err = localClient.List(context.Background(), &replicatedConfigMaps, opts...)
		if err != nil {
			return err
		}

		for _, rsec := range replicatedConfigMaps.Items {
			namespacedName := types.NamespacedName{Name: rsec.Name, Namespace: rsec.Namespace}
			// Add the new matcher rules also to our cache if not found
			selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
			if err != nil {
				return err
			}

			s.selectorMutex.Lock()
			s.selectors[namespacedName] = selector
			s.selectorMutex.Unlock()
		}
	}
	return nil
}
