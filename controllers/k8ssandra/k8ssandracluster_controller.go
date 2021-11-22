/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8ssandra

import (
	"context"
	"fmt"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"k8s.io/apimachinery/pkg/labels"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sort"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	k8ssandraClusterFinalizer = "k8ssandracluster.k8ssandra.io/finalizer"
)

// K8ssandraClusterReconciler reconciles a K8ssandraCluster object
type K8ssandraClusterReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme        *runtime.Scheme
	ClientCache   *clientcache.ClientCache
	SeedsResolver cassandra.RemoteSeedsResolver
	ManagementApi cassandra.ManagementApiFactory
}

// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters;clientconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.k8ssandra.io,namespace="k8ssandra",resources=clientconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=stargate.k8ssandra.io,namespace="k8ssandra",resources=stargates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=reaper.k8ssandra.io,namespace="k8ssandra",resources=reapers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=pods;secrets,verbs=get;list;watch

func (r *K8ssandraClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("K8ssandraCluster", req.NamespacedName)

	kc := &api.K8ssandraCluster{}
	err := r.Get(ctx, req.NamespacedName, kc)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, err
	}

	kc = kc.DeepCopy()
	patch := client.MergeFromWithOptions(kc.DeepCopy())
	result, err := r.reconcile(ctx, kc, logger)
	if kc.GetDeletionTimestamp() == nil {
		if patchErr := r.Status().Patch(ctx, kc, patch); patchErr != nil {
			logger.Error(patchErr, "failed to update k8ssandracluster status")
		} else {
			logger.Info("updated k8ssandracluster status")
		}
	}
	return result, err
}

func (r *K8ssandraClusterReconciler) reconcile(ctx context.Context, kc *api.K8ssandraCluster, kcLogger logr.Logger) (ctrl.Result, error) {
	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}

	if kc.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(kc, k8ssandraClusterFinalizer) {
			return ctrl.Result{}, nil
		}

		kcLogger.Info("Starting deletion")

		hasErrors := false

		for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
			namespace := dcTemplate.Meta.Namespace
			if namespace == "" {
				namespace = kc.Namespace
			}
			dcKey := client.ObjectKey{Namespace: namespace, Name: dcTemplate.Meta.Name}

			remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
			if err != nil {
				kcLogger.Error(err, "Failed to get remote client", "Context", dcTemplate.K8sContext)
				hasErrors = true
				continue
			}

			dc := &cassdcapi.CassandraDatacenter{}
			err = remoteClient.Get(ctx, dcKey, dc)
			if err != nil {
				if !errors.IsNotFound(err) {
					kcLogger.Error(err, "Failed to get CassandraDatacenter for deletion",
						"CassandraDatacenter", dcKey, "Context", dcTemplate.K8sContext)
					hasErrors = true
				}
			} else if err = remoteClient.Delete(ctx, dc); err != nil {
				kcLogger.Error(err, "Failed to delete CassandraDatacenter", "CassandraDatacenter", dcKey, "Context", dcTemplate.K8sContext)
				hasErrors = true
			}

			selector := utils.CreatedByK8ssandraControllerLabels(kc.Name)
			stargateList := &stargateapi.StargateList{}
			options := client.ListOptions{
				Namespace:     namespace,
				LabelSelector: labels.SelectorFromSet(selector),
			}

			err = remoteClient.List(ctx, stargateList, &options)
			if err != nil {
				kcLogger.Error(err, "Failed to list Stargate objects", "Context", dcTemplate.K8sContext)
				hasErrors = true
				continue
			}

			for _, sg := range stargateList.Items {
				if err = remoteClient.Delete(ctx, &sg); err != nil {
					key := client.ObjectKey{Namespace: namespace, Name: sg.Name}
					if !errors.IsNotFound(err) {
						kcLogger.Error(err, "Failed to delete Stargate", "Stargate", key,
							"Context", dcTemplate.K8sContext)
						hasErrors = true
					}
				}
			}

			if r.deleteReapers(ctx, kc, dcTemplate, namespace, remoteClient, kcLogger) {
				hasErrors = true
			}
		}

		if hasErrors {
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		patch := client.MergeFrom(kc.DeepCopy())
		controllerutil.RemoveFinalizer(kc, k8ssandraClusterFinalizer)
		if err := r.Client.Patch(ctx, kc, patch); err != nil {
			kcLogger.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(kc, k8ssandraClusterFinalizer) {
		patch := client.MergeFrom(kc.DeepCopy())
		controllerutil.AddFinalizer(kc, k8ssandraClusterFinalizer)
		if err := r.Client.Patch(ctx, kc, patch); err != nil {
			kcLogger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	if kc.Spec.Cassandra == nil {
		// TODO handle the scenario of CassandraClusterTemplate being set to nil after having a non-nil value
		return ctrl.Result{}, nil
	}

	var seeds []string

	// Reconcile the ReplicatedSecret and superuserSecret first (otherwise CassandraDatacenter will not start)
	if kc.Spec.Cassandra.SuperuserSecretName == "" {
		// Note that we do not persist this change because doing so would prevent us from
		// differentiating between a default secret by the operator vs one provided by the
		// client that happens to have the same name as the default name.
		kc.Spec.Cassandra.SuperuserSecretName = secret.DefaultSuperuserSecretName(kc.Spec.Cassandra.Cluster)
		kcLogger.Info("Setting default superuser secret", "SuperuserSecretName", kc.Spec.Cassandra.SuperuserSecretName)
	}

	if err := secret.ReconcileSecret(ctx, r.Client, kc.Spec.Cassandra.SuperuserSecretName, kc.Name, kc.Namespace); err != nil {
		kcLogger.Error(err, "Failed to verify existence of superuserSecret")
		return ctrl.Result{}, err
	}

	if err := r.reconcileReaperSecrets(ctx, kc, kcLogger); err != nil {
		return ctrl.Result{}, err
	}

	if err := secret.ReconcileReplicatedSecret(ctx, r.Client, r.Scheme, kc, kcLogger); err != nil {
		kcLogger.Error(err, "Failed to reconcile ReplicatedSecret")
		return ctrl.Result{}, err
	}

	systemReplication := cassandra.ComputeSystemReplication(kc)

	actualDcs := make([]*cassdcapi.CassandraDatacenter, 0, len(kc.Spec.Cassandra.Datacenters))
	// Reconcile CassandraDatacenter objects only
	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {

		if !secret.HasReplicatedSecrets(ctx, r.Client, kc.Name, kc.Namespace, dcTemplate.K8sContext) {
			// ReplicatedSecret has not replicated yet, wait until it has
			kcLogger.Info("Waiting for secret replication")
			return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
		}

		// Note that it is necessary to use a copy of the CassandraClusterTemplate because
		// its fields are pointers, and without the copy we could end of with shared
		// references that would lead to unexpected and incorrect values.
		dcConfig := cassandra.Coalesce(kc.Spec.Cassandra.DeepCopy(), dcTemplate.DeepCopy())
		cassandra.ApplySystemReplication(dcConfig, systemReplication)
		if !cassandra.IsCassandra3(dcConfig.ServerVersion) && kc.HasStargates() {
			// if we're not running Cassandra 3.11 and have Stargate pods, we need to allow alter RF during range movements
			cassandra.AllowAlterRfDuringRangeMovement(dcConfig)
		}
		dcConfig.AdditionalSeeds = seeds
		reaperTemplate := reaper.Coalesce(kc.Spec.Reaper.DeepCopy(), dcTemplate.Reaper.DeepCopy())
		if reaperTemplate != nil {
			reaper.AddReaperSettingsToDcConfig(reaperTemplate, dcConfig)
		}
		desiredDc, err := cassandra.NewDatacenter(kcKey, dcConfig)
		dcKey := types.NamespacedName{Namespace: desiredDc.Namespace, Name: desiredDc.Name}
		logger := kcLogger.WithValues("CassandraDatacenter", dcKey)

		if err != nil {
			logger.Error(err, "Failed to create new CassandraDatacenter")
			return ctrl.Result{}, err
		}

		desiredDcHash := utils.DeepHashString(desiredDc)
		desiredDc.Annotations[api.ResourceHashAnnotation] = desiredDcHash

		actualDc := &cassdcapi.CassandraDatacenter{}

		remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err = remoteClient.Get(ctx, dcKey, actualDc); err == nil {
			// cassdc already exists, we'll update it
			if err = r.setStatusForDatacenter(kc, actualDc); err != nil {
				logger.Error(err, "Failed to update status for datacenter")
				return ctrl.Result{}, err
			}

			if actualHash, found := actualDc.Annotations[api.ResourceHashAnnotation]; !(found && actualHash == desiredDcHash) {
				logger.Info("Updating datacenter")

				if actualDc.Spec.SuperuserSecretName != desiredDc.Spec.SuperuserSecretName {
					// If actualDc is created with SuperuserSecretName, it can't be changed anymore. We should reject all changes coming from K8ssandraCluster
					desiredDc.Spec.SuperuserSecretName = actualDc.Spec.SuperuserSecretName
					err = fmt.Errorf("tried to update superuserSecretName in K8ssandraCluster")
					logger.Error(err, "SuperuserSecretName is immutable, reverting to existing value in CassandraDatacenter")
				}

				actualDc = actualDc.DeepCopy()
				resourceVersion := actualDc.GetResourceVersion()
				desiredDc.DeepCopyInto(actualDc)
				actualDc.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualDc); err != nil {
					logger.Error(err, "Failed to update datacenter")
					return ctrl.Result{}, err
				}
			}

			if !cassandra.DatacenterReady(actualDc) {
				logger.Info("Waiting for datacenter to become ready")
				return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
			}

			logger.Info("The datacenter is ready")

			endpoints, err := r.SeedsResolver.ResolveSeedEndpoints(ctx, actualDc, remoteClient)
			if err != nil {
				logger.Error(err, "Failed to resolve seed endpoints")
				return ctrl.Result{}, err
			}

			// The following code will update the AdditionalSeeds property for the
			// datacenters. We will wind up having endpoints from every DC listed in
			// the AdditionalSeeds property. We really want to exclude the seeds from
			// the current DC. It is not a major concern right now as this is a short-term
			// solution for handling seed addresses.
			seeds = addSeedEndpoints(seeds, endpoints...)

			// Temporarily do not update seeds on existing dcs. SEE
			// https://github.com/k8ssandra/k8ssandra-operator/issues/67.
			// logger.Info("Updating seeds", "Seeds", seeds)
			// if err = r.updateAdditionalSeeds(ctx, kc, seeds, 0, i); err != nil {
			//	logger.Error(err, "Failed to update seeds")
			//	return ctrl.Result{}, err
			// }
			actualDcs = append(actualDcs, actualDc)
		} else {
			if errors.IsNotFound(err) {
				// cassdc doesn't exist, we'll create it
				if err = remoteClient.Create(ctx, desiredDc); err != nil {
					logger.Error(err, "Failed to create datacenter")
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
			} else {
				logger.Error(err, "Failed to get datacenter")
				return ctrl.Result{}, err
			}
		}
	}

	kcLogger.Info("All dcs reconciled")

	if kc.HasStargates() {
		kcLogger.Info("Reconciling Stargate auth schema")
		dcTemplate := kc.Spec.Cassandra.Datacenters[0]
		if remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext); err != nil {
			return ctrl.Result{}, err
		} else if err := r.reconcileStargateAuthSchema(ctx, kc, actualDcs[0], remoteClient, kcLogger); err != nil {
			return ctrl.Result{RequeueAfter: r.ReconcilerConfig.LongDelay}, err
		}
	}

	if kc.HasReapers() {
		kcLogger.Info("Reconciling Reaper schema")
		dcTemplate := kc.Spec.Cassandra.Datacenters[0]
		if remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext); err != nil {
			return ctrl.Result{}, err
		} else if err := r.reconcileReaperSchema(ctx, kc, actualDcs[0], remoteClient, kcLogger); err != nil {
			return ctrl.Result{RequeueAfter: r.ReconcilerConfig.LongDelay}, err
		}
	}

	// Reconcile Stargate and Reaper across all datacenters
	for i, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		actualDc := actualDcs[i]
		dcKey := types.NamespacedName{Namespace: actualDc.Namespace, Name: actualDc.Name}
		logger := kcLogger.WithValues("CassandraDatacenter", dcKey)
		logger.Info("Reconciling Stargate and Reaper for dc " + actualDc.Name)
		if remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext); err != nil {
			return ctrl.Result{}, err
		} else if result, err := r.reconcileStargate(ctx, kc, dcTemplate, actualDc, logger, remoteClient); !result.IsZero() || err != nil {
			return result, err
		} else if result, err := r.reconcileReaper(ctx, kc, dcTemplate, actualDc, logger, remoteClient); !result.IsZero() || err != nil {
			return result, err
		}
	}

	kcLogger.Info("Finished reconciling the k8ssandracluster")
	return ctrl.Result{}, nil
}

func (r *K8ssandraClusterReconciler) reconcileStargate(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) (ctrl.Result, error) {

	stargateTemplate := dcTemplate.Stargate.Coalesce(kc.Spec.Stargate)
	stargateKey := types.NamespacedName{
		Namespace: actualDc.Namespace,
		Name:      stargate.ResourceName(kc, actualDc),
	}
	actualStargate := &stargateapi.Stargate{}
	logger = logger.WithValues("Stargate", stargateKey)

	if stargateTemplate != nil {

		desiredStargate := r.newStargate(stargateKey, kc, stargateTemplate, actualDc)
		desiredStargateHash := utils.DeepHashString(desiredStargate)
		desiredStargate.Annotations[api.ResourceHashAnnotation] = desiredStargateHash

		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating Stargate resource")
				if err := remoteClient.Create(ctx, desiredStargate); err != nil {
					logger.Error(err, "Failed to create Stargate resource")
					return ctrl.Result{}, err
				} else {
					return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
				}
			} else {
				logger.Error(err, "Failed to get Stargate resource")
				return ctrl.Result{}, err
			}
		} else {
			if err = r.setStatusForStargate(kc, actualStargate, dcTemplate.Meta.Name); err != nil {
				logger.Error(err, "Failed to update status for stargate")
				return ctrl.Result{}, err
			}
			if actualStargateHash, found := actualStargate.Annotations[api.ResourceHashAnnotation]; !found || actualStargateHash != desiredStargateHash {
				logger.Info("Updating Stargate resource")
				resourceVersion := actualStargate.GetResourceVersion()
				desiredStargate.DeepCopyInto(actualStargate)
				actualStargate.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualStargate); err == nil {
					return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
				} else {
					logger.Error(err, "Failed to update Stargate resource")
					return ctrl.Result{}, err
				}
			}
			if !actualStargate.Status.IsReady() {
				logger.Info("Waiting for Stargate to become ready")
				return ctrl.Result{RequeueAfter: r.ReconcilerConfig.DefaultDelay}, nil
			}
			logger.Info("Stargate is ready")
		}
	} else {
		// Test if Stargate was removed
		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				// OK
			} else {
				logger.Error(err, "Failed to get Stargate resource", "Stargate", stargateKey)
				return ctrl.Result{}, err
			}
		} else if utils.IsCreatedByK8ssandraController(actualStargate, kc.Name) {
			if err := remoteClient.Delete(ctx, actualStargate); err != nil {
				logger.Error(err, "Failed to delete Stargate resource", "Stargate", stargateKey)
				return ctrl.Result{}, err
			} else {
				r.removeStargateStatus(kc, dcTemplate.Meta.Name)
				logger.Info("Stargate deleted", "Stargate", stargateKey)
			}
		} else {
			logger.Info("Not deleting Stargate since it wasn't created by this controller", "Stargate", stargateKey)
		}
	}
	return ctrl.Result{}, nil
}

func (r *K8ssandraClusterReconciler) newStargate(stargateKey types.NamespacedName, kc *api.K8ssandraCluster, stargateTemplate *stargateapi.StargateDatacenterTemplate, actualDc *cassdcapi.CassandraDatacenter) *stargateapi.Stargate {
	desiredStargate := &stargateapi.Stargate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   stargateKey.Namespace,
			Name:        stargateKey.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.NameLabel:             api.NameLabelValue,
				api.PartOfLabel:           api.PartOfLabelValue,
				api.ComponentLabel:        api.ComponentLabelValueStargate,
				api.CreatedByLabel:        api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterLabel: kc.Name,
			},
		},
		Spec: stargateapi.StargateSpec{
			StargateDatacenterTemplate: *stargateTemplate,
			DatacenterRef:              corev1.LocalObjectReference{Name: actualDc.Name},
		},
	}
	return desiredStargate
}

// addSeedEndpoints returns a new slice with endpoints added to seeds and duplicates
// removed.
func addSeedEndpoints(seeds []string, endpoints ...string) []string {
	updatedSeeds := make([]string, 0, len(seeds))
	updatedSeeds = append(updatedSeeds, seeds...)

	for _, endpoint := range endpoints {
		if !utils.SliceContains(updatedSeeds, endpoint) {
			updatedSeeds = append(updatedSeeds, endpoint)
		}
	}

	// We must sort the results here to ensure consistent ordering. See
	// https://github.com/k8ssandra/k8ssandra-operator/issues/80 for details.
	sort.Strings(updatedSeeds)

	return updatedSeeds
}

func (r *K8ssandraClusterReconciler) setStatusForDatacenter(kc *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter) error {
	if len(kc.Status.Datacenters) == 0 {
		kc.Status.Datacenters = make(map[string]api.K8ssandraStatus, 0)
	}

	kdcStatus, found := kc.Status.Datacenters[dc.Name]

	if found {
		dc.Status.DeepCopyInto(kdcStatus.Cassandra)
	} else {
		kc.Status.Datacenters[dc.Name] = api.K8ssandraStatus{
			Cassandra: dc.Status.DeepCopy(),
		}
	}

	return nil
}

func (r *K8ssandraClusterReconciler) setStatusForStargate(kc *api.K8ssandraCluster, stargate *stargateapi.Stargate, dcName string) error {
	if len(kc.Status.Datacenters) == 0 {
		kc.Status.Datacenters = make(map[string]api.K8ssandraStatus)
	}

	kdcStatus, found := kc.Status.Datacenters[dcName]

	if found {
		if kdcStatus.Stargate == nil {
			kdcStatus.Stargate = stargate.Status.DeepCopy()
			kc.Status.Datacenters[dcName] = kdcStatus
		} else {
			stargate.Status.DeepCopyInto(kdcStatus.Stargate)
		}
	} else {
		kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
			Stargate: stargate.Status.DeepCopy(),
		}
	}

	if kc.Status.Datacenters[dcName].Stargate.Progress == "" {
		kc.Status.Datacenters[dcName].Stargate.Progress = stargateapi.StargateProgressPending
	}
	return nil
}

func (r *K8ssandraClusterReconciler) reconcileStargateAuthSchema(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger,
) error {
	managementApi, err := r.ManagementApi.NewManagementApiFacade(ctx, dc, remoteClient, logger)
	if err == nil {
		replication := cassandra.ComputeReplication(3, kc.Spec.Cassandra.Datacenters...)
		if err = managementApi.EnsureKeyspaceReplication(stargate.AuthKeyspace, replication); err == nil {
			err = stargate.ReconcileAuthTable(managementApi, logger)
		}
	}
	return err
}

func (r *K8ssandraClusterReconciler) removeStargateStatus(kc *api.K8ssandraCluster, dcName string) {
	if kdcStatus, found := kc.Status.Datacenters[dcName]; found {
		kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
			Stargate:  nil,
			Cassandra: kdcStatus.Cassandra.DeepCopy(),
			Reaper:    kdcStatus.Reaper.DeepCopy(),
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraClusterReconciler) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraCluster{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})) // No generation changed predicate here?

	clusterLabelFilter := func(mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)
		k8cName := utils.GetLabel(mapObj, api.K8ssandraClusterLabel)
		if k8cName != "" {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mapObj.GetNamespace(), Name: k8cName}})
		}
		return requests
	}

	for _, c := range clusters {
		cb = cb.Watches(source.NewKindWithCache(&cassdcapi.CassandraDatacenter{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
		cb = cb.Watches(source.NewKindWithCache(&stargateapi.Stargate{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
		cb = cb.Watches(source.NewKindWithCache(&reaperapi.Reaper{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
	}

	return cb.Complete(r)
}
