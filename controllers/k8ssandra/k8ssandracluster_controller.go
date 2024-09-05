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
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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
	ManagementApi cassandra.ManagementApiFactory
	Recorder      record.EventRecorder
}

// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters;clientconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.k8ssandra.io,namespace="k8ssandra",resources=clientconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=control.k8ssandra.io,namespace="k8ssandra",resources=cassandratasks,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=stargate.k8ssandra.io,namespace="k8ssandra",resources=stargates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=reaper.k8ssandra.io,namespace="k8ssandra",resources=reapers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=pods;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,namespace="k8ssandra",resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",namespace="k8ssandra",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=batch,namespace="k8ssandra",resources=cronjobs,verbs=get;list;watch;create;update;delete

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
	patch := client.MergeFrom(kc.DeepCopy())
	result, err := r.reconcile(ctx, kc, logger)
	if kc.GetDeletionTimestamp() == nil {
		if err != nil {
			kc.Status.Error = err.Error()
			r.Recorder.Event(kc, corev1.EventTypeWarning, "Reconcile Error", err.Error())
		} else {
			kc.Status.Error = "None"
		}
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

	if medusaSecretResult := r.reconcileMedusaSecrets(ctx, kc, kcLogger); medusaSecretResult.Completed() {
		return medusaSecretResult.Output()
	}

	kcLogger.Info("Reconciling replicated secrets")

	if recResult := r.reconcileReplicatedSecret(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	var actualDcs []*cassdcapi.CassandraDatacenter
	if recResult, dcs := r.reconcileDatacenters(ctx, kc, kcLogger); recResult.Completed() {
		return recResult.Output()
	} else {
		actualDcs = dcs
	}

	kcLogger.Info("All DCs reconciled")

	if recResult := r.afterCassandraReconciled(ctx, kc, actualDcs, kcLogger); recResult.Completed() {
		return recResult.Output()
	}

	if res := updateStatus(ctx, r.Client, kc); res.Completed() {
		return res.Output()
	}

	kcLogger.Info("Finished reconciling the k8ssandracluster")

	return result.Done().Output()
}

func (r *K8ssandraClusterReconciler) afterCassandraReconciled(ctx context.Context, kc *api.K8ssandraCluster, dcs []*cassdcapi.CassandraDatacenter, logger logr.Logger) result.ReconcileResult {
	for i, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		dc := dcs[i]
		dcKey := utils.GetKey(dc)
		logger := logger.WithValues("CassandraDatacenter", dcKey)
		logger.Info("Reconciling Stargate and Reaper for dc " + dc.DatacenterName())
		if remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext); err != nil {
			logger.Error(err, "Failed to get remote client")
			return result.Error(err)
		} else if recResult := r.reconcileCassandraDCTelemetry(ctx, kc, dcTemplate, dc, logger, remoteClient); recResult.Completed() {
			return recResult
		} else if recResult := r.reconcileStargate(ctx, kc, dcTemplate, dc, logger, remoteClient); recResult.Completed() {
			return recResult
		} else if recResult := r.reconcileReaper(ctx, kc, dcTemplate, dc, logger, remoteClient); recResult.Completed() {
			return recResult
		} else if recResult := r.setupVectorCleanup(ctx, kc, dc, remoteClient, logger); recResult.Completed() {
			return recResult
		}
	}
	return result.Continue()
}

func updateStatus(ctx context.Context, r client.Client, kc *api.K8ssandraCluster) result.ReconcileResult {
	if AllowUpdate(kc) {
		if metav1.HasAnnotation(kc.ObjectMeta, api.AutomatedUpdateAnnotation) {
			if kc.Annotations[api.AutomatedUpdateAnnotation] == string(api.AllowUpdateOnce) {
				delete(kc.ObjectMeta.Annotations, api.AutomatedUpdateAnnotation)
				if err := r.Update(ctx, kc); err != nil {
					return result.Error(err)
				}
			}
		}
		kc.Status.SetConditionStatus(api.ClusterRequiresUpdate, corev1.ConditionFalse)
	}

	kc.Status.ObservedGeneration = kc.Generation
	if err := r.Status().Update(ctx, kc); err != nil {
		return result.Error(err)
	}

	return result.Continue()
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraClusterReconciler) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraCluster{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})))

	clusterLabelFilter := func(ctx context.Context, mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)

		kcName := labels.GetLabel(mapObj, api.K8ssandraClusterNameLabel)
		kcNamespace := labels.GetLabel(mapObj, api.K8ssandraClusterNamespaceLabel)

		if kcName != "" && kcNamespace != "" {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: kcNamespace, Name: kcName}})
		}
		return requests
	}

	// Use a more specific filter for Endpoints because we are only interested in one particular service.
	endpointsFilter := func(ctx context.Context, mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)

		kcName := labels.GetLabel(mapObj, api.K8ssandraClusterNameLabel)
		kcNamespace := labels.GetLabel(mapObj, api.K8ssandraClusterNamespaceLabel)

		if kcName != "" && kcNamespace != "" && strings.HasSuffix(mapObj.GetName(), "all-pods-service") {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: kcNamespace, Name: kcName}})
		}
		return requests
	}

	cb = cb.Watches(&cassdcapi.CassandraDatacenter{},
		handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
	cb = cb.Watches(&stargateapi.Stargate{},
		handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
	cb = cb.Watches(&reaperapi.Reaper{},
		handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
	cb = cb.Watches(&v1.ConfigMap{},
		handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
	cb = cb.Watches(&v1.Endpoints{},
		handler.EnqueueRequestsFromMapFunc(endpointsFilter))

	for _, c := range clusters {
		cb = cb.WatchesRawSource(source.Kind(c.GetCache(), &cassdcapi.CassandraDatacenter{}),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
		cb = cb.WatchesRawSource(source.Kind(c.GetCache(), &stargateapi.Stargate{}),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
		cb = cb.WatchesRawSource(source.Kind(c.GetCache(), &reaperapi.Reaper{}),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
		cb = cb.WatchesRawSource(source.Kind(c.GetCache(), &v1.ConfigMap{}),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
		cb = cb.WatchesRawSource(source.Kind(c.GetCache(), &v1.Endpoints{}),
			handler.EnqueueRequestsFromMapFunc(endpointsFilter))
	}

	return cb.Complete(r)
}
