package controllers

import (
	"context"
	"fmt"
	"reflect"

	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
)

const (
	replicatedSecretFinalizer = "replicatedsecret.k8ssandra.io/finalizer"
)

// We need rights to update the target cluster's secrets, not necessarily this cluster
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=secrets,verbs=get;list;watch;update;create
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=replicatedsecrets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=replicatedsecrets/finalizers,verbs=update

type SecretSyncController struct {
	ClientCache *clientcache.ClientCache
	selectors   map[types.NamespacedName]labels.Selector
}

func (s *SecretSyncController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	localClient := s.ClientCache.GetLocalClient()

	rsec := &api.ReplicatedSecret{}
	if err := localClient.Get(ctx, req.NamespacedName, rsec); err != nil {
		logger.Error(err, "Failed to get replicated secret", "ReplicatedSecret", req.NamespacedName)
		return reconcile.Result{}, err
	}

	// Deletion and finalizer logic
	if rsec.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(rsec, replicatedSecretFinalizer) {
			delete(s.selectors, req.NamespacedName)
			controllerutil.RemoveFinalizer(rsec, replicatedSecretFinalizer)
			err := localClient.Update(ctx, rsec)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(rsec, replicatedSecretFinalizer) {
		controllerutil.AddFinalizer(rsec, replicatedSecretFinalizer)
		err := localClient.Update(ctx, rsec)
		if err != nil {
			logger.Error(err, "Failed to get add finalizer to replicated secret", "ReplicatedSecret", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	// Add the new matcher rules also to our cache if not found
	selector, err := metav1.LabelSelectorAsSelector(&rsec.Spec.Selector)
	if err != nil {
		logger.Error(err, "Failed to transform to label selector", "ReplicatedSecret", req.NamespacedName)
		return reconcile.Result{}, err
	}

	// Update the selector in cache always (comparing is pointless)
	s.selectors[req.NamespacedName] = selector

	// Fetch all the secrets that match the ReplicatedSecret's rules
	secrets, err := s.fetchAllMatchingSecrets(selector)
	if err != nil {
		logger.Error(err, "Failed to fetch linked secrets", "ReplicatedSecret", req.NamespacedName)
		return reconcile.Result{}, err
	}

	for _, targetCtx := range rsec.Spec.TargetContexts {
		// Only replicate to clusters that are in the ReplicatedSecret's context
		remoteClient, err := s.ClientCache.GetRemoteClient(targetCtx)
		if err != nil {
			logger.Error(err, "Failed to fetch remote client for managed cluster", "ReplicatedSecret", req.NamespacedName, "TargetContext", targetCtx)
			return ctrl.Result{}, err
		}

		// Iterate all the matching secrets
		for _, sec := range secrets {
			fetchedSecret := &corev1.Secret{}
			if err := remoteClient.Get(ctx, types.NamespacedName{Name: sec.Name, Namespace: sec.Namespace}, fetchedSecret); err != nil {
				if errors.IsNotFound(err) {
					logger.Info("Copying secret to target cluster", "Secret", sec.Name, "TargetContext", targetCtx)
					// Create it
					copiedSecret := sec.DeepCopy()
					copiedSecret.ResourceVersion = ""
					copiedSecret.OwnerReferences = []metav1.OwnerReference{}
					if err := remoteClient.Create(ctx, copiedSecret); err != nil {
						logger.Error(err, "Failed to sync secret to target cluster", "Secret", copiedSecret.Name, "TargetContext", targetCtx)
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
				logger.Error(err, "Failed to fetch secret from managed cluster", "Secret", fetchedSecret.Name, "TargetContext", targetCtx)
				return ctrl.Result{}, err
			}

			if fetchedSecret.Immutable != nil && *fetchedSecret.Immutable {
				err := fmt.Errorf("target secret immutable")
				logger.Error(err, "Failed to modify target secret, secret is set to immutable", "Secret", fetchedSecret.Name, "TargetContext", targetCtx)
				return ctrl.Result{}, err
			}

			// If Data section does not match, update the target
			if !reflect.DeepEqual(sec.Data, fetchedSecret.Data) {
				logger.Info("Modifying secret in target cluster", "Secret", sec.Name, "TargetContext", targetCtx)
				tmpSec := sec.DeepCopy()
				fetchedSecret.Data = tmpSec.Data
				if err := remoteClient.Update(ctx, fetchedSecret); err != nil {
					logger.Error(err, "Failed to sync target secret for matching payloads", "Secret", fetchedSecret.Name, "TargetContext", targetCtx)
					return ctrl.Result{}, err
				}
			}
		}
	}

	// Update the ReplicatedSecret's LastReplicationTime
	patch := client.MergeFromWithOptions(rsec.DeepCopy(), client.MergeFromWithOptimisticLock{})
	rsec.Status.LastReplicationTime = metav1.Now()
	err = localClient.Status().Patch(ctx, rsec, patch)
	if err != nil {
		logger.Error(err, "Failed to update replicated secret last transition time", "ReplicatedSecret", req.NamespacedName)
	}

	return ctrl.Result{}, err
}

func (s *SecretSyncController) fetchAllMatchingSecrets(selector labels.Selector) ([]corev1.Secret, error) {
	secrets := &corev1.SecretList{}
	listOption := client.ListOptions{
		LabelSelector: selector,
	}
	err := s.ClientCache.GetLocalClient().List(context.TODO(), secrets, &listOption)
	if err != nil {
		return nil, err
	}

	return secrets.Items, nil
}

func (s *SecretSyncController) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	err := s.initializeCache()
	if err != nil {
		return err
	}

	// We should only reconcile objects that match the rules
	toMatchingReplicates := func(secret client.Object) []reconcile.Request {
		requests := []reconcile.Request{}
		for k, v := range s.selectors {
			if v.Matches(labels.Set(secret.GetLabels())) {
				requests = append(requests, reconcile.Request{NamespacedName: k})
			}
		}
		return requests
	}

	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.ReplicatedSecret{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(toMatchingReplicates))

	for _, c := range clusters {
		cb = cb.Watches(
			source.NewKindWithCache(&corev1.Secret{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(toMatchingReplicates))
	}

	return cb.Complete(s)
}

func (s *SecretSyncController) initializeCache() error {
	s.selectors = make(map[types.NamespacedName]labels.Selector)
	localClient := s.ClientCache.GetLocalNonCacheClient()

	replicatedSecrets := api.ReplicatedSecretList{}
	err := localClient.List(context.Background(), &replicatedSecrets)
	if err != nil {
		return err
	}

	for _, rsec := range replicatedSecrets.Items {
		namespacedName := types.NamespacedName{Name: rsec.Name, Namespace: rsec.Namespace}
		// Add the new matcher rules also to our cache if not found
		selector, err := metav1.LabelSelectorAsSelector(&rsec.Spec.Selector)
		if err != nil {
			return err
		}

		s.selectors[namespacedName] = selector
	}

	return nil
}
