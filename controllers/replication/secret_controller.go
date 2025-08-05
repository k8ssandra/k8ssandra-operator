package replication

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"

	coreapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
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

// TODO Move these to apis?
const (
	replicatedResourceFinalizer = "replicatedresource.k8ssandra.io/finalizer"
)

// We need rights to update the target cluster's secrets, not necessarily this cluster
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=secrets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=replication.k8ssandra.io,namespace="k8ssandra",resources=replicatedsecrets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=replication.k8ssandra.io,namespace="k8ssandra",resources=replicatedsecrets/finalizers,verbs=update
// +kubebuilder:rbac:groups=replication.k8ssandra.io,namespace="k8ssandra",resources=replicatedsecrets/status,verbs=get;update;patch

type SecretSyncController struct {
	*config.ReconcilerConfig
	ClientCache *clientcache.ClientCache
	// TODO We need a better structure for empty selectors (match whole kind)
	WatchNamespaces []string
	selectorMutex   sync.RWMutex
	selectors       map[types.NamespacedName]labels.Selector
}

func (s *SecretSyncController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	localClient := s.ClientCache.GetLocalClient()

	logger.Info("Starting reconciliation", "key", req.NamespacedName)

	rsec := &api.ReplicatedSecret{}
	if err := localClient.Get(ctx, req.NamespacedName, rsec); err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "Failed to get replicated secret, it may have been deleted", "ReplicatedSecret", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get replicated secret", "ReplicatedSecret", req.NamespacedName)
		return reconcile.Result{}, err
	}
	// Deletion and finalizer logic
	if rsec.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(rsec, replicatedResourceFinalizer) {
			logger.Info("Starting cleanup")

			// Fetch all secrets from managed cluster.
			// Remove only those secrets which are not matched by any other ReplicatedSecret and do not have the orphan annotation
			if val, found := rsec.GetAnnotations()[secret.OrphanResourceAnnotation]; !found || val != "true" {
				logger.Info("Cleaning up all the replicated resources", "ReplicatedSecret", req.NamespacedName)
				selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
				if err != nil {
					logger.Error(err, "Failed to delete the replicated secret, defined labels are invalid", "ReplicatedSecret", req.NamespacedName)
					return reconcile.Result{}, err
				}

				secrets, err := s.fetchAllMatchingSecrets(ctx, selector, rsec.Namespace)
				if err != nil {
					logger.Error(err, "Failed to fetch the replicated secrets to cleanup", "ReplicatedSecret", req.NamespacedName, "Namespace", rsec.Namespace)
					return reconcile.Result{}, err
				}

				sourceSecretsToMapToTargets := make([]*corev1.Secret, 0, len(secrets))

				s.selectorMutex.RLock()

			SecretsToCheck:
				for _, sec := range secrets {
					key := client.ObjectKey{Namespace: sec.Namespace, Name: sec.Name}
					logger.Info("Checking secret", "key", key)
					for k, v := range s.selectors {
						if k.Namespace != sec.Namespace {
							logger.Info("Skipping secret", "key", key, "namespace", k.Namespace)
							continue
						}
						if k == req.NamespacedName {
							// This is the ReplicatedSecret that will be deleted, we don't want its rules to match
							continue
						}

						if val, found := sec.GetAnnotations()[secret.OrphanResourceAnnotation]; found && val == "true" {
							// Managed cluster has orphan set to the secret, do not delete it from target clusters
							continue SecretsToCheck
						}

						if v.Matches(labels.Set(sec.GetLabels())) {
							// Another Replication rule is matching this secret, do not delete it
							logger.Info("Another replication rule matches secret", "key", key)
							continue SecretsToCheck
						}
					}
					logger.Info("Preparing to delete secrets downstream from", "key", key)
					sourceSecretsToMapToTargets = append(sourceSecretsToMapToTargets, &sec)
				}

				s.selectorMutex.RUnlock()

				// Convert []corev1.Secret to []*corev1.Secret for syncSecretsToTarget
				secretsToSync := make([]corev1.Secret, len(sourceSecretsToMapToTargets))
				for i, s := range sourceSecretsToMapToTargets {
					secretsToSync[i] = *s
				}

				for _, target := range rsec.Spec.ReplicationTargets {
					logger.Info("Deleting secrets for ReplicationTarget", "Target", target)
					// Only replicate to clusters that are in the ReplicatedSecret's context
					remoteClient, err := s.ClientCache.GetRemoteClient(target.K8sContextName)
					if err != nil {
						logger.Error(err, "Failed to fetch remote client for managed cluster", "ReplicatedSecret", req.NamespacedName, "TargetContext", target)
						return ctrl.Result{}, err
					}

					if err := s.cleanupSecretsInTarget(ctx, secretsToSync, rsec, target, remoteClient, logger); err != nil {
						return ctrl.Result{}, err
					}
				}
			}
			s.selectorMutex.Lock()
			delete(s.selectors, req.NamespacedName)
			s.selectorMutex.Unlock()
			controllerutil.RemoveFinalizer(rsec, replicatedResourceFinalizer)
			if err := localClient.Update(ctx, rsec); err != nil {
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
		return ctrl.Result{Requeue: true}, err
	}

	// Add the new matcher rules also to our cache if not found
	selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
	if err != nil {
		logger.Error(err, "Failed to transform to label selector", "ReplicatedSecret", req.NamespacedName)
		return reconcile.Result{}, err
	}

	// Update the selector in cache always (comparing is pointless)
	s.selectorMutex.Lock()
	s.selectors[req.NamespacedName] = selector
	s.selectorMutex.Unlock()

	// Fetch all the secrets that match the ReplicatedSecret's rules
	secrets, err := s.fetchAllMatchingSecrets(ctx, selector, req.Namespace)
	if err != nil {
		logger.Error(err, "Failed to fetch linked secrets", "ReplicatedSecret", req.NamespacedName, "namespace", req.Namespace)
		return reconcile.Result{}, err
	}
	// Verify secrets have up-to-date hashes
	for i := range secrets {
		sec := &secrets[i]
		if err := s.verifyHashAnnotation(ctx, sec); err != nil {
			logger.Error(err, "Failed to update secret hashes", "ReplicatedSecret", req.NamespacedName, "Secret", sec.Name)
			return reconcile.Result{}, err
		}
	}

	// For status updates
	patch := client.MergeFrom(rsec.DeepCopy())
	// patch := client.MergeFromWithOptions(rsec.DeepCopy(), client.MergeFromWithOptimisticLock{})
	rsec.Status.Conditions = make([]api.ReplicationCondition, 0, len(rsec.Spec.ReplicationTargets))

	for _, target := range rsec.Spec.ReplicationTargets {
		// Even if ReplicationTarget includes local client - remove it (it will cause errors)
		// Only replicate to clusters that are in the ReplicatedSecret's context
		var remoteClient client.Client
		if target.K8sContextName == "" {
			remoteClient = localClient
		} else {
			remoteClient, err = s.ClientCache.GetRemoteClient(target.K8sContextName)
			if err != nil {
				logger.Error(err, "Failed to fetch remote client for managed cluster", "ReplicatedSecret", req.NamespacedName, "TargetContext", target)
				return ctrl.Result{}, err
			}
		}

		cond := api.ReplicationCondition{
			Cluster: target.K8sContextName,
			Type:    api.ReplicationDone,
		}

		err = s.syncSecretsToTarget(ctx, secrets, rsec, target, remoteClient, logger)
		if err != nil {
			cond.Status = corev1.ConditionFalse
		} else {
			cond.Status = corev1.ConditionTrue
		}

		timeNow := metav1.Now()
		cond.LastTransitionTime = &timeNow
		rsec.Status.Conditions = append(rsec.Status.Conditions, cond)
	}

	// Update the ReplicatedSecret's Status
	err = localClient.Status().Patch(ctx, rsec, patch)
	if err != nil {
		logger.Error(err, "Failed to update replicated secret last transition time", "ReplicatedSecret", req.NamespacedName)
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

// syncSecretsToTarget syncs a list of secrets to a specific replication target
func (s *SecretSyncController) syncSecretsToTarget(ctx context.Context, secrets []corev1.Secret, rsec *api.ReplicatedSecret, target api.ReplicationTarget, remoteClient client.Client, logger logr.Logger) error {
	for i := range secrets {
		sec := &secrets[i]
		// If the secret would be created in the same target namespace with the same labels, skip and warn.
		isInf, err := wouldBeInfinite(*sec, *rsec, target)
		if err != nil {
			logger.Error(err, "failed to check infinite replication", "Secret", "TargetContext", target)
			return err
		}
		if isInf {
			logger.Info("warning: secret would be infinite, bailing", "Secret", sec.Name, "TargetContext", target)
			continue
		}
		namespace := ""
		if target.Namespace == "" {
			namespace = sec.Namespace
		} else {
			namespace = target.Namespace
		}
		fetchedSecret := &corev1.Secret{}
		if err = remoteClient.Get(ctx, types.NamespacedName{Name: getPrefixedSecretName(target.TargetPrefix, sec.Name), Namespace: namespace}, fetchedSecret); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Copying secret to target cluster", "Secret", sec.Name, "TargetContext", target)
				// Create it
				copiedSecret := sec.DeepCopy()
				copiedSecret.Namespace = namespace
				copiedSecret.ResourceVersion = ""
				copiedSecret.OwnerReferences = []metav1.OwnerReference{}
				copiedSecret.Name = getPrefixedSecretName(target.TargetPrefix, sec.Name)
				copiedSecret.Labels = calculateTargetLabels(copiedSecret.Labels, target)
				if err = remoteClient.Create(ctx, copiedSecret); err != nil {
					logger.Error(err, "Failed to sync secret to target cluster", "Secret", copiedSecret.Name, "TargetContext", target)
					return err
				}
				continue
			}
			logger.Error(err, "Failed to fetch secret from target cluster", "Secret", fetchedSecret.Name, "TargetContext", target)
			return err
		}

		if fetchedSecret.Immutable != nil && *fetchedSecret.Immutable {
			err := fmt.Errorf("target secret is immutable")
			logger.Error(err, "Failed to modify target secret, secret is set to immutable", "Secret", fetchedSecret.Name, "TargetContext", target)
			return err
		}

		if requiresUpdate(sec, fetchedSecret) {
			logger.Info("Modifying secret in target cluster", "Secret", sec.Name, "TargetContext", target)
			syncSecrets(sec, fetchedSecret, target)
			copiedSecret := fetchedSecret.DeepCopy()
			copiedSecret.Name = getPrefixedSecretName(target.TargetPrefix, sec.Name)
			if err = remoteClient.Update(ctx, copiedSecret); err != nil {
				logger.Error(err, "Failed to sync target secret for matching payloads", "Secret", fetchedSecret.Name, "TargetContext", target)
				return err
			}
		}
	}
	return nil
}

// cleanupSecretsInTarget deletes replicated secrets from a target
func (s *SecretSyncController) cleanupSecretsInTarget(ctx context.Context, secrets []corev1.Secret, rsec *api.ReplicatedSecret, target api.ReplicationTarget, remoteClient client.Client, logger logr.Logger) error {
	for i := range secrets {
		origSecret := &secrets[i]
		targetNamespace := utils.FirstNonEmptyString(target.Namespace, origSecret.Namespace)
		deleteObject := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: getPrefixedSecretName(target.TargetPrefix, origSecret.Name), Namespace: targetNamespace}}
		if origSecret.Namespace == target.Namespace && origSecret.Name == deleteObject.Name {
			// Target is the same secret as the original - bail.
			// TODO: Note that this will cause secrets to not be cleaned up if they are in a remote cluster.
			continue
		}
		logger.Info("Deleting secrets for", "objectMeta", deleteObject.ObjectMeta,
			"Cluster", target.K8sContextName)
		err := remoteClient.Delete(ctx, deleteObject)
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to remove secrets from target cluster", "ReplicatedSecret", rsec.Name, "TargetContext", target, "targetSecret", deleteObject.ObjectMeta)
			return err
		}
	}
	return nil
}

func requiresUpdate(source, dest client.Object) bool {
	// In case we target the same cluster
	if source.GetUID() == dest.GetUID() {
		return false
	}

	if srcHash, found := source.GetAnnotations()[coreapi.ResourceHashAnnotation]; found {
		// Get dest hash value
		destHash, destFound := dest.GetAnnotations()[coreapi.ResourceHashAnnotation]
		if !destFound {
			return true
		}

		if destSec, valid := dest.(*corev1.Secret); valid {
			hash := utils.DeepHashString(destSec.Data)
			if destHash != hash {
				// Destination data did not match destination hash
				return true
			}
		}

		return srcHash != destHash
	}
	return false
}

func syncSecrets(src, dest *corev1.Secret, target api.ReplicationTarget) {
	origMeta := dest.ObjectMeta
	src.DeepCopyInto(dest)
	dest.ObjectMeta = origMeta
	dest.OwnerReferences = []metav1.OwnerReference{}

	// sync annotations, src is more important
	if dest.GetAnnotations() == nil {
		dest.Annotations = make(map[string]string)
	}

	for k, v := range src.Annotations {
		if !filterValue(k) {
			dest.Annotations[k] = v
		}
	}

	// sync labels, src is more important
	if dest.GetLabels() == nil {
		dest.Labels = make(map[string]string)
	}

	for k, v := range src.Labels {
		if !filterValue(k) {
			dest.Labels[k] = v
		}
	}
	// TODO: it would be nice at some point in future to remove the DC specific hardcoded stuff and
	// filter DC specific stuff by setting dropLabels in the replicatedsecret resource.
	dest.Labels = calculateTargetLabels(dest.Labels, target)
}

// filterValue verifies the annotation is not something datacenter specific
// TODO: it would be nice at some point in future to remove the DC specific hardcoded stuff and
// filter DC specific stuff by setting dropLabels in the replicatedsecret resource.
func filterValue(key string) bool {
	return strings.HasPrefix(key, "cassandra.datastax.com/")
}

func (s *SecretSyncController) verifyHashAnnotation(ctx context.Context, sec *corev1.Secret) error {
	hash := utils.DeepHashString(sec.Data)
	if sec.GetAnnotations() == nil {
		sec.Annotations = make(map[string]string)
	}
	if existingHash, found := sec.GetAnnotations()[coreapi.ResourceHashAnnotation]; !found || (existingHash != hash) {
		sec.GetAnnotations()[coreapi.ResourceHashAnnotation] = hash
		return s.ClientCache.GetLocalClient().Update(ctx, sec)
	}
	return nil
}

func (s *SecretSyncController) fetchAllMatchingSecrets(ctx context.Context, selector labels.Selector, namespace string) ([]corev1.Secret, error) {
	secrets := &corev1.SecretList{}
	listOption := client.ListOptions{
		LabelSelector: selector,
		Namespace:     namespace,
	}
	err := s.ClientCache.GetLocalClient().List(ctx, secrets, &listOption)
	if err != nil {
		return nil, err
	}

	return secrets.Items, nil
}

func (s *SecretSyncController) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster, logger logr.Logger) error {
	err := s.initializeCache()
	if err != nil {
		logger.Info("Failed to initialize cache, error", err)
		return err
	}

	// We should only reconcile objects that match the rules
	toMatchingReplicates := func(ctx context.Context, secret client.Object) []reconcile.Request {
		requests := []reconcile.Request{}
		s.selectorMutex.RLock()
		for k, v := range s.selectors {
			if v.Matches(labels.Set(secret.GetLabels())) {
				requests = append(requests, reconcile.Request{NamespacedName: k})
			}
		}
		s.selectorMutex.RUnlock()
		return requests
	}

	toMatchingReplicatesTyped := func(ctx context.Context, secret *corev1.Secret) []reconcile.Request {
		return toMatchingReplicates(ctx, secret)
	}

	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.ReplicatedSecret{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(toMatchingReplicates))

	for _, c := range clusters {
		cb = cb.WatchesRawSource(
			source.Kind(c.GetCache(), &corev1.Secret{},
				handler.TypedEnqueueRequestsFromMapFunc(toMatchingReplicatesTyped)))
	}

	return cb.Complete(s)
}

func (s *SecretSyncController) initializeCache() error {
	s.selectors = make(map[types.NamespacedName]labels.Selector)
	localClient := s.ClientCache.GetLocalNonCacheClient()

	for _, namespace := range s.WatchNamespaces {
		var err error
		replicatedSecrets := api.ReplicatedSecretList{}
		opts := make([]client.ListOption, 0, 1)
		if namespace != "" {
			opts = append(opts, client.InNamespace(namespace))
		}
		err = localClient.List(context.Background(), &replicatedSecrets, opts...)
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

			s.selectorMutex.Lock()
			s.selectors[namespacedName] = selector
			s.selectorMutex.Unlock()
		}
	}
	return nil
}

func getPrefixedSecretName(prefix string, secretName string) string {
	return fmt.Sprintf("%s%s", prefix, secretName)
}

func calculateTargetLabels(originalLabels map[string]string, target api.ReplicationTarget) map[string]string {
	out := make(map[string]string)
	for k, v := range originalLabels {
		out[k] = v
	}
	for k, v := range target.AddLabels {
		out[k] = v
	}
	for _, key := range target.DropLabels {
		delete(out, key)
	}
	return out
}

func wouldBeInfinite(origin corev1.Secret, rsec api.ReplicatedSecret, target api.ReplicationTarget) (bool, error) {
	computedLabels := labels.Set(calculateTargetLabels(origin.Labels, target))
	selector, err := metav1.LabelSelectorAsSelector(rsec.Spec.Selector)
	if err != nil {
		return true, err
	}
	if selector.Matches(computedLabels) {
		if (origin.Namespace == target.Namespace || target.Namespace == "") && target.K8sContextName == "" && target.TargetPrefix != "" { // TargetPrefix is included here because the initial .Get in the main reconciliation body will return the original secret if no target prefix is specified, thereby bailing from the secret creation process.
			// This will still be infinite if the target has a non-empty k8scontext which points back to the origin cluster.
			return true, nil
		}
	}
	return false, nil
}
