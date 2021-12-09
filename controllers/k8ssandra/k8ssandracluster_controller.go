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

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
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
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,namespace="k8ssandra",resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete

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
	if recResult := r.checkDeletion(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := r.checkFinalizer(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	if kc.Spec.Cassandra == nil {
		// TODO handle the scenario of CassandraClusterTemplate being set to nil after having a non-nil value
		return ctrl.Result{}, nil
	}

	// Reconcile the ReplicatedSecret and superuserSecret first (otherwise CassandraDatacenter will not start)

	if recResult := r.reconcileSuperuserSecret(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := r.reconcileReaperSecrets(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := r.reconcileReplicatedSecret(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	var actualDcs []*cassdcapi.CassandraDatacenter
	if recResult, dcs := r.reconcileDatacenters(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	} else {
		actualDcs = dcs
	}

	kcLogger.Info("All dcs reconciled")

	if recResult := r.reconcileStargateAuthSchema(ctx, kc, actualDcs, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := r.reconcileReaperSchema(ctx, kc, actualDcs, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := r.reconcileStargateAndReaper(ctx, kc, actualDcs, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	kcLogger.Info("Finished reconciling the k8ssandracluster")

	return result.Done().Output()
}

func (r *K8ssandraClusterReconciler) reconcileStargateAndReaper(ctx context.Context, kc *api.K8ssandraCluster, dcs []*cassdcapi.CassandraDatacenter, logger logr.Logger) result.ReconcileResult {
	for i, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		dc := dcs[i]
		dcKey := utils.GetKey(dc)
		logger := logger.WithValues("CassandraDatacenter", dcKey)
		logger.Info("Reconciling Stargate and Reaper for dc " + dc.Name)
		if remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext); err != nil {
			logger.Error(err, "Failed to get remote client")
			return result.Error(err)
		} else if recResult := r.reconcileStargate(ctx, kc, dcTemplate, dc, logger, remoteClient); recResult.Completed() {
			return recResult
		} else if recResult := r.reconcileReaper(ctx, kc, dcTemplate, dc, logger, remoteClient); recResult.Completed() {
			return recResult
		} else if result, err := r.reconcileCassandraDCTelemetry(ctx, kc, dcTemplate, dc, logger, remoteClient); !result.IsZero() || err != nil {
			return reconcile.Result.Requeue
		}
	}
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileStargate(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) result.ReconcileResult {

	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	stargateTemplate := dcTemplate.Stargate.Coalesce(kc.Spec.Stargate)
	stargateKey := types.NamespacedName{
		Namespace: actualDc.Namespace,
		Name:      stargate.ResourceName(kc, actualDc),
	}
	actualStargate := &stargateapi.Stargate{}
	logger = logger.WithValues("Stargate", stargateKey)

	if stargateTemplate != nil {
		logger.Info("Reconcile Stargate")

		desiredStargate := r.newStargate(stargateKey, kc, stargateTemplate, actualDc)
		utils.AddHashAnnotation(desiredStargate, api.ResourceHashAnnotation)

		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating Stargate resource")
				if err := remoteClient.Create(ctx, desiredStargate); err != nil {
					logger.Error(err, "Failed to create Stargate resource")
					return result.Error(err)
				} else {
					return result.RequeueSoon(r.DefaultDelay)
				}
			} else {
				logger.Error(err, "Failed to get Stargate resource")
				return result.Error(err)
			}
		} else {
			if err = r.setStatusForStargate(kc, actualStargate, dcTemplate.Meta.Name); err != nil {
				logger.Error(err, "Failed to update status for stargate")
				return result.Error(err)
			}
			if !utils.CompareAnnotations(desiredStargate, actualStargate, api.ResourceHashAnnotation) {
				logger.Info("Updating Stargate")
				resourceVersion := actualStargate.GetResourceVersion()
				desiredStargate.DeepCopyInto(actualStargate)
				actualStargate.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualStargate); err == nil {
					return result.RequeueSoon(r.DefaultDelay)
				} else {
					logger.Error(err, "Failed to update Stargate")
					return result.Error(err)
				}
			}
			if !actualStargate.Status.IsReady() {
				logger.Info("Waiting for Stargate to become ready")
				return result.RequeueSoon(r.DefaultDelay)
			}
			logger.Info("Stargate is ready")
		}
	} else {
		logger.Info("Stargate not present")

		// Test if Stargate was removed
		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				// OK
			} else {
				logger.Error(err, "Failed to get Stargate")
				return result.Error(err)
			}
		} else if utils.IsCreatedByK8ssandraController(actualStargate, kcKey) {
			if err := remoteClient.Delete(ctx, actualStargate); err != nil {
				logger.Error(err, "Failed to delete Stargate")
				return result.Error(err)
			} else {
				r.removeStargateStatus(kc, dcTemplate.Meta.Name)
				logger.Info("Stargate deleted")
			}
		} else {
			logger.Info("Not deleting Stargate since it wasn't created by this controller")
		}
	}
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) newStargate(stargateKey types.NamespacedName, kc *api.K8ssandraCluster, stargateTemplate *stargateapi.StargateDatacenterTemplate, actualDc *cassdcapi.CassandraDatacenter) *stargateapi.Stargate {
	desiredStargate := &stargateapi.Stargate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   stargateKey.Namespace,
			Name:        stargateKey.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.NameLabel:                      api.NameLabelValue,
				api.PartOfLabel:                    api.PartOfLabelValue,
				api.ComponentLabel:                 api.ComponentLabelValueStargate,
				api.CreatedByLabel:                 api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterNameLabel:      kc.Name,
				api.K8ssandraClusterNamespaceLabel: kc.Namespace,
			},
		},
		Spec: stargateapi.StargateSpec{
			StargateDatacenterTemplate: *stargateTemplate,
			DatacenterRef:              corev1.LocalObjectReference{Name: actualDc.Name},
		},
	}
	return desiredStargate
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

func (r *K8ssandraClusterReconciler) checkDeletion(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if kc.DeletionTimestamp == nil {
		return result.Continue()
	}

	if !controllerutil.ContainsFinalizer(kc, k8ssandraClusterFinalizer) {
		return result.Done()
	}

	logger.Info("Starting deletion")

	kcKey := utils.GetKey(kc)
	hasErrors := false

	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		namespace := dcTemplate.Meta.Namespace
		if namespace == "" {
			namespace = kc.Namespace
		}

		dcKey := client.ObjectKey{Namespace: namespace, Name: dcTemplate.Meta.Name}

		remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
		if err != nil {
			logger.Error(err, "Failed to get remote client", "Context", dcTemplate.K8sContext)
			hasErrors = true
			continue
		}

		dc := &cassdcapi.CassandraDatacenter{}
		err = remoteClient.Get(ctx, dcKey, dc)
		if err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "Failed to get CassandraDatacenter for deletion",
					"CassandraDatacenter", dcKey, "Context", dcTemplate.K8sContext)
				hasErrors = true
			}
		} else if err = remoteClient.Delete(ctx, dc); err != nil {
			logger.Error(err, "Failed to delete CassandraDatacenter", "CassandraDatacenter", dcKey, "Context", dcTemplate.K8sContext)
			hasErrors = true
		}

		selector := utils.CreatedByK8ssandraControllerLabels(kcKey)
		stargateList := &stargateapi.StargateList{}
		options := client.ListOptions{
			Namespace:     namespace,
			LabelSelector: labels.SelectorFromSet(selector),
		}

		err = remoteClient.List(ctx, stargateList, &options)
		if err != nil {
			logger.Error(err, "Failed to list Stargate objects", "Context", dcTemplate.K8sContext)
			hasErrors = true
			continue
		}

		for _, sg := range stargateList.Items {
			if err = remoteClient.Delete(ctx, &sg); err != nil {
				key := client.ObjectKey{Namespace: namespace, Name: sg.Name}
				if !errors.IsNotFound(err) {
					logger.Error(err, "Failed to delete Stargate", "Stargate", key,
						"Context", dcTemplate.K8sContext)
					hasErrors = true
				}
			}
		}

		if r.deleteReapers(ctx, kc, dcTemplate, namespace, remoteClient, logger) {
			hasErrors = true
		}
	}

	if hasErrors {
		return result.RequeueSoon(r.DefaultDelay)
	}

	patch := client.MergeFrom(kc.DeepCopy())
	controllerutil.RemoveFinalizer(kc, k8ssandraClusterFinalizer)
	if err := r.Client.Patch(ctx, kc, patch); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return result.Error(err)
	}

	return result.Done()
}

func (r *K8ssandraClusterReconciler) checkFinalizer(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if controllerutil.ContainsFinalizer(kc, k8ssandraClusterFinalizer) {
		return result.Continue()
	}

	patch := client.MergeFrom(kc.DeepCopy())
	controllerutil.AddFinalizer(kc, k8ssandraClusterFinalizer)
	if err := r.Client.Patch(ctx, kc, patch); err != nil {
		logger.Error(err, "Failed to add finalizer")
		return result.Error(err)
	}

	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileSuperuserSecret(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if kc.Spec.Cassandra.SuperuserSecretName == "" {
		// Note that we do not persist this change because doing so would prevent us from
		// differentiating between a default secret by the operator vs one provided by the
		// client that happens to have the same name as the default name.
		kc.Spec.Cassandra.SuperuserSecretName = secret.DefaultSuperuserSecretName(kc.Spec.Cassandra.Cluster)
		logger.Info("Setting default superuser secret", "SuperuserSecretName", kc.Spec.Cassandra.SuperuserSecretName)
	}

	if err := secret.ReconcileSecret(ctx, r.Client, kc.Spec.Cassandra.SuperuserSecretName, utils.GetKey(kc)); err != nil {
		logger.Error(err, "Failed to verify existence of superuserSecret")
		return result.Error(err)
	}

	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileReplicatedSecret(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if err := secret.ReconcileReplicatedSecret(ctx, r.Client, r.Scheme, kc, logger); err != nil {
		logger.Error(err, "Failed to reconcile ReplicatedSecret")
		return result.Error(err)
	}
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileDatacenters(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) (result.ReconcileResult, []*cassdcapi.CassandraDatacenter) {
	kcKey := utils.GetKey(kc)
	systemReplication := cassandra.ComputeSystemReplication(kc)

	actualDcs := make([]*cassdcapi.CassandraDatacenter, 0, len(kc.Spec.Cassandra.Datacenters))

	seeds, err := r.findSeeds(ctx, kc, logger)
	if err != nil {
		logger.Error(err, "Failed to find seed nodes")
		return result.Error(err), actualDcs
	}

	// Reconcile CassandraDatacenter objects only
	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {

		if !secret.HasReplicatedSecrets(ctx, r.Client, kcKey, dcTemplate.K8sContext) {
			// ReplicatedSecret has not replicated yet, wait until it has
			logger.Info("Waiting for replication to complete")
			return result.RequeueSoon(r.DefaultDelay), actualDcs
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
		reaperTemplate := reaper.Coalesce(kc.Spec.Reaper.DeepCopy(), dcTemplate.Reaper.DeepCopy())
		if reaperTemplate != nil {
			reaper.AddReaperSettingsToDcConfig(reaperTemplate, dcConfig)
		}
		desiredDc, err := cassandra.NewDatacenter(kcKey, dcConfig)
		dcKey := types.NamespacedName{Namespace: desiredDc.Namespace, Name: desiredDc.Name}
		logger := logger.WithValues("CassandraDatacenter", dcKey, "K8SContext", dcTemplate.K8sContext)

		if err != nil {
			logger.Error(err, "Failed to create new CassandraDatacenter")
			return result.Error(err), actualDcs
		}

		utils.AddHashAnnotation(desiredDc, api.ResourceHashAnnotation)

		actualDc := &cassdcapi.CassandraDatacenter{}

		remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
		if err != nil {
			logger.Error(err, "Failed to get remote client")
			return result.Error(err), actualDcs
		}

		if recResult := r.reconcileSeedsEndpoints(ctx, desiredDc, seeds, remoteClient, logger); recResult.Completed() {
			return recResult, actualDcs
		}

		if err = remoteClient.Get(ctx, dcKey, actualDc); err == nil {
			// cassdc already exists, we'll update it
			if err = r.setStatusForDatacenter(kc, actualDc); err != nil {
				logger.Error(err, "Failed to update status for datacenter")
				return result.Error(err), actualDcs
			}

			if !utils.CompareAnnotations(actualDc, desiredDc, api.ResourceHashAnnotation) {
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
					return result.Error(err), actualDcs
				}
			}

			if !cassandra.DatacenterReady(actualDc) {
				logger.Info("Waiting for datacenter to become ready")
				return result.RequeueSoon(r.DefaultDelay), actualDcs
			}

			logger.Info("The datacenter is ready")

			actualDcs = append(actualDcs, actualDc)
		} else {
			if errors.IsNotFound(err) {
				// cassdc doesn't exist, we'll create it
				if err = remoteClient.Create(ctx, desiredDc); err != nil {
					logger.Error(err, "Failed to create datacenter")
					return result.Error(err), actualDcs
				}
				return result.RequeueSoon(r.DefaultDelay), actualDcs
			} else {
				logger.Error(err, "Failed to get datacenter")
				return result.Error(err), actualDcs
			}
		}
	}

	// If we reach this point all CassandraDatacenters are ready. We only set the
	// CassandraInitialized condition if it is unset, i.e., only once. This allows us to
	// distinguish whether we are deploying a CassandraDatacenter as part of a new cluster
	// or as part of an existing cluster.
	if kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionUnknown {
		now := metav1.Now()
		(&kc.Status).SetCondition(api.K8ssandraClusterCondition{
			Type:               api.CassandraInitialized,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
	}

	return result.Continue(), actualDcs
}

func (r *K8ssandraClusterReconciler) reconcileStargateAuthSchema(ctx context.Context, kc *api.K8ssandraCluster, dcs []*cassdcapi.CassandraDatacenter, logger logr.Logger) result.ReconcileResult {
	if !kc.HasStargates() {
		return result.Continue()
	}

	logger.Info("Reconciling Stargate auth schema")
	dcTemplate := kc.Spec.Cassandra.Datacenters[0]

	if remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext); err != nil {
		logger.Error(err, "Failed to get remote client")
		return result.Error(err)
	} else {
		dc := dcs[0]
		managementApi, err := r.ManagementApi.NewManagementApiFacade(ctx, dc, remoteClient, logger)
		if err != nil {
			logger.Error(err, "Failed to create ManagementApiFacade")
			return result.Error(err)
		}

		replication := cassandra.ComputeReplication(3, kc.Spec.Cassandra.Datacenters...)
		if err = managementApi.EnsureKeyspaceReplication(stargate.AuthKeyspace, replication); err != nil {
			logger.Error(err, "Failed to ensure keyspace replication")
			return result.Error(err)
		}

		if err = stargate.ReconcileAuthTable(managementApi, logger); err != nil {
			logger.Error(err, "Failed to reconcile Stargate auth table")
			return result.Error(err)
		}

		return result.Continue()
	}

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

		kcName := utils.GetLabel(mapObj, api.K8ssandraClusterNameLabel)
		kcNamespace := utils.GetLabel(mapObj, api.K8ssandraClusterNamespaceLabel)

		if kcName != "" && kcNamespace != "" {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: kcNamespace, Name: kcName}})
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
