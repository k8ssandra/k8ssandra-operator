package controllers

import (
	"context"
	"fmt"
	"reflect"

	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
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

// TODO Or .. ReplicatedResource? Just set Kind to the resource ..

const (
	replicatedResourceFinalizer = "replicatedresource.k8ssandra.io/finalizer"

	// OrphanResourceAnnotation when set to true prevents the deletion of secret from target clusters even if matching ReplicatedSecret is removed
	OrphanResourceAnnotation = "replicatedresource.k8ssandra.io/orphan"
)

// We need rights to update the target cluster's secrets, not necessarily this cluster
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=secrets,verbs=get;list;watch;update;create
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=replicatedsecrets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=replicatedsecrets/finalizers,verbs=update

type SecretSyncController struct {
	ClientCache *clientcache.ClientCache
	// TODO We need a better structure for empty selectors (match whole kind)
	selectors map[types.NamespacedName]labels.Selector
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
		if controllerutil.ContainsFinalizer(rsec, replicatedResourceFinalizer) {

			// Fetch all secrets from managed cluster.
			// Remove only those secrets which are not matched by any other ReplicatedSecret and do not have the orphan annotation
			if val, found := rsec.GetAnnotations()[OrphanResourceAnnotation]; !found || val != "true" {
				logger.Info("Cleaning up all the replicated resources", "ReplicatedSecret", req.NamespacedName)
				selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
				if err != nil {
					logger.Error(err, "Failed to delete the replicated secret, defined labels are invalid", "ReplicatedSecret", req.NamespacedName)
					return reconcile.Result{}, err
				}

				secrets, err := s.fetchAllMatchingSecrets(selector)
				if err != nil {
					logger.Error(err, "Failed to fetch the replicated secrets to cleanup", "ReplicatedSecret", req.NamespacedName)
					return reconcile.Result{}, err
				}

				secretsToDelete := make([]*corev1.Secret, 0, len(secrets))

			SecretsToCheck:
				for _, sec := range secrets {
					for k, v := range s.selectors {
						if k == req.NamespacedName {
							// This is the ReplicatedSecret that will be deleted, we don't want its rules to match
							continue
						}

						if val, found := sec.GetAnnotations()[OrphanResourceAnnotation]; found && val == "true" {
							// Managed cluster has orphan set to the secret, do not delete it from target clusters
							continue SecretsToCheck
						}

						if v.Matches(labels.Set(sec.GetLabels())) {
							// Another Replication rule is matching this secret, do not delete it
							continue SecretsToCheck
						}
						secretsToDelete = append(secretsToDelete, &sec)
					}
				}

				for _, target := range rsec.Spec.ReplicationTargets {
					// Only replicate to clusters that are in the ReplicatedSecret's context
					remoteClient, err := s.ClientCache.GetRemoteClient(target.K8sContextName)
					if err != nil {
						logger.Error(err, "Failed to fetch remote client for managed cluster", "ReplicatedSecret", req.NamespacedName, "TargetContext", target)
						return ctrl.Result{}, err
					}
					for _, deleteKey := range secretsToDelete {
						err = remoteClient.Delete(ctx, deleteKey)
						if err != nil {
							logger.Error(err, "Failed to remove secrets from target cluster", "ReplicatedSecret", req.NamespacedName, "TargetContext", target)
							return ctrl.Result{}, err
						}
					}
				}
			}
			delete(s.selectors, req.NamespacedName)
			controllerutil.RemoveFinalizer(rsec, replicatedResourceFinalizer)
			err := localClient.Update(ctx, rsec)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(rsec, replicatedResourceFinalizer) {
		controllerutil.AddFinalizer(rsec, replicatedResourceFinalizer)
		err := localClient.Update(ctx, rsec)
		if err != nil {
			logger.Error(err, "Failed to get add finalizer to replicated secret", "ReplicatedSecret", req.NamespacedName)
			return ctrl.Result{}, err
		}
	}

	// Add the new matcher rules also to our cache if not found
	selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
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
	// Verify secrets have up-to-date hashes
	for _, sec := range secrets {
		if err := s.verifyHashAnnotation(ctx, &sec); err != nil {
			logger.Error(err, "Failed to update secret hashes", "ReplicatedSecret", req.NamespacedName, "Secret", sec.Name)
			return reconcile.Result{}, err
		}
	}

	// For status updates
	patch := client.MergeFromWithOptions(rsec.DeepCopy(), client.MergeFromWithOptimisticLock{})

	for _, target := range rsec.Spec.ReplicationTargets {
		// Only replicate to clusters that are in the ReplicatedSecret's context
		remoteClient, err := s.ClientCache.GetRemoteClient(target.K8sContextName)
		if err != nil {
			logger.Error(err, "Failed to fetch remote client for managed cluster", "ReplicatedSecret", req.NamespacedName, "TargetContext", target)
			return ctrl.Result{}, err
		}

		cond := api.ReplicationCondition{
			Cluster: target.K8sContextName,
			Type:    api.ReplicationDone,
		}

	TargetSecrets:
		// Iterate all the matching secrets
		for _, sec := range secrets {
			fetchedSecret := &corev1.Secret{}
			if err = remoteClient.Get(ctx, types.NamespacedName{Name: sec.Name, Namespace: sec.Namespace}, fetchedSecret); err != nil {
				if errors.IsNotFound(err) {
					logger.Info("Copying secret to target cluster", "Secret", sec.Name, "TargetContext", target)
					// Create it
					copiedSecret := sec.DeepCopy()
					copiedSecret.ResourceVersion = ""
					copiedSecret.OwnerReferences = []metav1.OwnerReference{}
					if err := remoteClient.Create(ctx, copiedSecret); err != nil {
						logger.Error(err, "Failed to sync secret to target cluster", "Secret", copiedSecret.Name, "TargetContext", target)
						break TargetSecrets
					}
					continue
				}
				logger.Error(err, "Failed to fetch secret from target cluster", "Secret", fetchedSecret.Name, "TargetContext", target)
				break TargetSecrets
			}

			if fetchedSecret.Immutable != nil && *fetchedSecret.Immutable {
				err := fmt.Errorf("target secret immutable")
				logger.Error(err, "Failed to modify target secret, secret is set to immutable", "Secret", fetchedSecret.Name, "TargetContext", target)
				break TargetSecrets
			}

			logger.Info("Generations for modification check", "Sec", sec.Generation, "FetchedSecret", fetchedSecret.Generation)
			if requiresUpdate(&sec, fetchedSecret) {
				logger.Info("Modifying secret in target cluster", "Secret", sec.Name, "TargetContext", target)
				// TODO These will only work with Secrets, not with ConfigMaps for example
				syncSecrets(&sec, fetchedSecret)
				logger.Info(fmt.Sprintf("Updating to new data: %v", fetchedSecret.Data))
				// TODO What about Patch?
				if err := remoteClient.Update(ctx, fetchedSecret); err != nil {
					logger.Error(err, "Failed to sync target secret for matching payloads", "Secret", fetchedSecret.Name, "TargetContext", target)
					break TargetSecrets
				}
			}
		}
		if err != nil {
			cond.Status = corev1.ConditionFalse
		}

		cond.Status = corev1.ConditionTrue
	}

	// Update the ReplicatedSecret's Status
	err = localClient.Status().Patch(ctx, rsec, patch)
	if err != nil {
		logger.Error(err, "Failed to update replicated secret last transition time", "ReplicatedSecret", req.NamespacedName)
	}

	// TODO If any cluster had failed state, retry
	return ctrl.Result{}, err
}

func requiresUpdate(source, dest client.Object) bool {
	if source.GetGeneration() != 0 {
		// Compare using generation
		return source.GetGeneration() > dest.GetGeneration()
	} else {
		if srcHash, found := source.GetAnnotations()[api.ResourceHashAnnotation]; found {
			destHash := dest.GetAnnotations()[api.ResourceHashAnnotation]
			if srcHash != destHash {
				return true
			}
		}
		// Verify data wasn't manually updated without hash changes..
		if sourceSec, valid := source.(*corev1.Secret); valid {
			if destSec, valid := dest.(*corev1.Secret); valid {
				return !reflect.DeepEqual(sourceSec.Data, destSec.Data)
			}
		}
	}
	return false
}

func syncSecrets(src, dest *corev1.Secret) {
	origMeta := dest.ObjectMeta
	src.DeepCopyInto(dest)
	dest.ObjectMeta = origMeta
	dest.OwnerReferences = []metav1.OwnerReference{}

	// sync annotations, src is more important
	if dest.GetAnnotations() == nil {
		dest.Annotations = make(map[string]string)
	}

	for k, v := range src.Annotations {
		dest.Annotations[k] = v
	}

	// sync labels, src is more important
	if dest.GetLabels() == nil {
		dest.Labels = make(map[string]string)
	}

	for k, v := range src.Labels {
		dest.Labels[k] = v
	}
}

func (s *SecretSyncController) verifyHashAnnotation(ctx context.Context, sec *corev1.Secret) error {
	hash := utils.DeepHashString(sec.Data)
	if sec.GetAnnotations() == nil {
		sec.Annotations = make(map[string]string)
	}
	if existingHash, found := sec.GetAnnotations()[api.ResourceHashAnnotation]; !found || (existingHash != hash) {
		sec.GetAnnotations()[api.ResourceHashAnnotation] = hash
		return s.ClientCache.GetLocalClient().Update(ctx, sec)
	}
	return nil
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
		selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
		if err != nil {
			return err
		}

		s.selectors[namespacedName] = selector
	}

	return nil
}
