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

package controllers

import (
	"context"
	"fmt"
	stargateutil "github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"math"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/cluster"

	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// K8ssandraClusterReconciler reconciles a K8ssandraCluster object
type K8ssandraClusterReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ClientCache   *clientcache.ClientCache
	SeedsResolver cassandra.RemoteSeedsResolver
}

//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters;clientconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch;create;update;delete;patch
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=stargates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=secrets,verbs=get;list;watch

func (r *K8ssandraClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	kc := &api.K8ssandraCluster{}
	err := r.Get(ctx, req.NamespacedName, kc)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	kc = kc.DeepCopy()

	if kc.Spec.Cassandra != nil {
		var seeds []string
		systemDistributedRF := getSystemDistributedRF(kc)
		dcNames := make([]string, 0, len(kc.Spec.Cassandra.Datacenters))

		for _, dc := range kc.Spec.Cassandra.Datacenters {
			dcNames = append(dcNames, dc.Meta.Name)
		}

		for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
			desiredDc, err := newDatacenter(req.NamespacedName, kc.Spec.Cassandra.Cluster, dcNames, dcTemplate, seeds, systemDistributedRF)
			if err != nil {
				logger.Error(err, "Failed to create new CassandraDatacenter")
				return ctrl.Result{}, err
			}
			dcKey := types.NamespacedName{Namespace: desiredDc.Namespace, Name: desiredDc.Name}

			desiredDcHash := utils.DeepHashString(desiredDc)
			desiredDc.Annotations[api.ResourceHashAnnotation] = desiredDcHash

			remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
			if err != nil {
				logger.Error(err, "Failed to get remote client for datacenter", "CassandraDatacenter", dcKey)
				return ctrl.Result{}, err
			}

			if remoteClient == nil {
				logger.Info("remoteClient cannot be nil")
				return ctrl.Result{}, fmt.Errorf("remoteClient cannot be nil")
			}

			actualDc := &cassdcapi.CassandraDatacenter{}

			if err = remoteClient.Get(ctx, dcKey, actualDc); err == nil {
				if err = r.setStatusForDatacenter(ctx, kc, actualDc); err != nil {
					logger.Error(err, "Failed to update status for datacenter", "CassandraDatacenter", dcKey)
					return ctrl.Result{}, err
				}

				if actualHash, found := actualDc.Annotations[api.ResourceHashAnnotation]; !(found && actualHash == desiredDcHash) {
					logger.Info("Updating datacenter", "CassandraDatacenter", dcKey)
					actualDc = actualDc.DeepCopy()
					resourceVersion := actualDc.GetResourceVersion()
					desiredDc.DeepCopyInto(actualDc)
					actualDc.SetResourceVersion(resourceVersion)
					if err = remoteClient.Update(ctx, actualDc); err != nil {
						logger.Error(err, "Failed to update datacenter", "CassandraDatacenter", dcKey)
						return ctrl.Result{}, err
					}
				}

				if !cassandra.DatacenterReady(actualDc) {
					logger.Info("Waiting for datacenter to become ready", "CassandraDatacenter", dcKey)
					return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
				}

				logger.Info("The datacenter is ready", "CassandraDatacenter", dcKey)

				endpoints, err := r.SeedsResolver.ResolveSeedEndpoints(ctx, actualDc, remoteClient)
				if err != nil {
					logger.Error(err, "Failed to resolve seed endpoints", "CassandraDatacenter", dcKey)
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
				//logger.Info("Updating seeds", "Seeds", seeds)
				//if err = r.updateAdditionalSeeds(ctx, kc, seeds, 0, i); err != nil {
				//	logger.Error(err, "Failed to update seeds")
				//	return ctrl.Result{}, err
				//}
			} else {
				if errors.IsNotFound(err) {
					if err = remoteClient.Create(ctx, desiredDc); err != nil {
						logger.Error(err, "Failed to create datacenter", "CassandraDatacenter", dcKey)
						return ctrl.Result{}, err
					}
					return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
				} else {
					logger.Error(err, "Failed to get datacenter", "CassandraDatacenter", dcKey)
					return ctrl.Result{}, err
				}
			}

			if dcTemplate.Stargate != nil {

				stargateKey := types.NamespacedName{
					Namespace: actualDc.Namespace,
					Name:      kc.Name + "-" + actualDc.Name + "-stargate",
				}

				desiredStargate := &api.Stargate{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   stargateKey.Namespace,
						Name:        stargateKey.Name,
						Annotations: map[string]string{},
						Labels: map[string]string{
							api.PartOfLabel:           api.PartOfLabelValue,
							api.K8ssandraClusterLabel: req.Name,
						},
					},
					Spec: api.StargateSpec{
						StargateTemplate: *dcTemplate.Stargate,
						DatacenterRef:    corev1.LocalObjectReference{Name: actualDc.Name},
					},
				}
				desiredStargateHash := utils.DeepHashString(desiredStargate)
				desiredStargate.Annotations[api.ResourceHashAnnotation] = desiredStargateHash

				actualStargate := &api.Stargate{}

				if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
					if errors.IsNotFound(err) {
						logger.Info("Creating Stargate resource", "Stargate", stargateKey)

						if err := remoteClient.Create(ctx, desiredStargate); err != nil {
							logger.Error(err, "Failed to create Stargate resource", "Stargate", stargateKey)
							return ctrl.Result{}, err
						} else {
							return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
						}
					} else {
						logger.Error(err, "Failed to get Stargate resource", "Stargate", stargateKey)
						return ctrl.Result{}, err
					}
				} else {
					if err = r.setStatusForStargate(ctx, kc, actualStargate, dcTemplate.Meta.Name); err != nil {
						logger.Error(err, "Failed to update status for stargate", "Stargate", stargateKey)
						return ctrl.Result{}, err
					}

					if actualStargateHash, found := actualStargate.Annotations[api.ResourceHashAnnotation]; !found || actualStargateHash != desiredStargateHash {
						logger.Info("Updating Stargate resource", "Stargate", stargateKey)
						resourceVersion := actualStargate.GetResourceVersion()
						desiredStargate.DeepCopyInto(actualStargate)
						actualStargate.SetResourceVersion(resourceVersion)

						if err = remoteClient.Update(ctx, actualStargate); err == nil {
							return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
						} else {
							logger.Error(err, "Failed to update Stargate resource", "Stargate", stargateKey)
							return ctrl.Result{}, err
						}
					}

					if !stargateutil.IsReady(actualStargate) {
						logger.Info("Waiting for Stargate to become ready", "Stargate", stargateKey)
						return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
					}
					logger.Info("Stargate is ready", "Stargate", stargateKey)
				}
			}
		}
	}
	logger.Info("Finished reconciling the k8ssandracluster")
	return ctrl.Result{}, nil
}

func newDatacenter(k8ssandraKey types.NamespacedName, cluster string, dcNames []string, template api.CassandraDatacenterTemplateSpec, additionalSeeds []string, systemDistributedRF int) (*cassdcapi.CassandraDatacenter, error) {
	namespace := template.Meta.Namespace
	if len(namespace) == 0 {
		namespace = k8ssandraKey.Namespace
	}

	config, err := cassandra.GetMergedConfig(template.Config, dcNames, systemDistributedRF, template.ServerVersion)
	if err != nil {
		return nil, err
	}

	return &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        template.Meta.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.PartOfLabel:           api.PartOfLabelValue,
				api.K8ssandraClusterLabel: k8ssandraKey.Name,
			},
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:     cluster,
			ServerImage:     template.ServerImage,
			Size:            template.Size,
			ServerType:      "cassandra",
			ServerVersion:   template.ServerVersion,
			Resources:       template.Resources,
			Config:          config,
			Racks:           template.Racks,
			StorageConfig:   template.StorageConfig,
			AdditionalSeeds: additionalSeeds,
			Networking: &cassdcapi.NetworkingConfig{
				HostNetwork: true,
			},
		},
	}, nil
}

func getSystemDistributedRF(kc *api.K8ssandraCluster) int {
	size := 1.0
	for _, dc := range kc.Spec.Cassandra.Datacenters {
		size = math.Min(size, float64(dc.Size))
	}
	replicationFactor := math.Min(size, 3.0)

	return int(replicationFactor)
}

func (r *K8ssandraClusterReconciler) resolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error) {
	podList := &corev1.PodList{}
	labels := client.MatchingLabels{cassdcapi.DatacenterLabel: dc.Name}

	err := remoteClient.List(ctx, podList, labels)
	if err != nil {
		return nil, err
	}

	endpoints := make([]string, 0, 3)

	for _, pod := range podList.Items {
		endpoints = append(endpoints, pod.Status.PodIP)
		if len(endpoints) > 2 {
			break
		}
	}

	return endpoints, nil
}

func (r *K8ssandraClusterReconciler) updateAdditionalSeeds(ctx context.Context, kc *api.K8ssandraCluster, seeds []string, start, end int) error {
	for i := start; i < end; i++ {
		dc, remoteClient, err := r.getDatacenterForTemplate(ctx, kc, i)
		if err != nil {
			return err
		}

		if err = r.updateAdditionalSeedsForDatacenter(ctx, dc, seeds, remoteClient); err != nil {
			return err
		}
	}

	return nil
}

// addSeedEndpoints returns a new slice with endpoints added to seeds and duplicates
// removed.
func addSeedEndpoints(seeds []string, endpoints ...string) []string {
	updatedSeeds := make([]string, 0, len(seeds))
	updatedSeeds = append(updatedSeeds, seeds...)

	for _, endpoint := range endpoints {
		if !contains(updatedSeeds, endpoint) {
			updatedSeeds = append(updatedSeeds, endpoint)
		}
	}

	return updatedSeeds
}

func contains(slice []string, s string) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}

func (r *K8ssandraClusterReconciler) updateAdditionalSeedsForDatacenter(ctx context.Context, dc *cassdcapi.CassandraDatacenter, seeds []string, remoteClient client.Client) error {
	patch := client.MergeFromWithOptions(dc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	if dc.Spec.AdditionalSeeds == nil {
		dc.Spec.AdditionalSeeds = make([]string, 0, len(seeds))
	}
	dc.Spec.AdditionalSeeds = seeds
	return remoteClient.Patch(ctx, dc, patch)
}

func (r *K8ssandraClusterReconciler) getDatacenterForTemplate(ctx context.Context, kc *api.K8ssandraCluster, idx int) (*cassdcapi.CassandraDatacenter, client.Client, error) {
	dcTemplate := kc.Spec.Cassandra.Datacenters[idx]
	k8ssandraKey := types.NamespacedName{Namespace: kc.Namespace, Name: kc.Name}
	remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)

	if err != nil {
		return nil, nil, err
	}

	dc := &cassdcapi.CassandraDatacenter{}
	dcKey := getDatacenterKey(dcTemplate, k8ssandraKey)
	err = remoteClient.Get(ctx, dcKey, dc)

	return dc, remoteClient, err
}

func getDatacenterKey(dcTemplate api.CassandraDatacenterTemplateSpec, kcKey types.NamespacedName) types.NamespacedName {
	if len(dcTemplate.Meta.Namespace) == 0 {
		return types.NamespacedName{Namespace: kcKey.Namespace, Name: dcTemplate.Meta.Name}
	}
	return types.NamespacedName{Namespace: dcTemplate.Meta.Namespace, Name: dcTemplate.Meta.Name}
}

func (r *K8ssandraClusterReconciler) setStatusForDatacenter(ctx context.Context, kc *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter) error {
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
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

	return r.Status().Patch(ctx, kc, patch)
}

func (r *K8ssandraClusterReconciler) setStatusForStargate(ctx context.Context, kc *api.K8ssandraCluster, stargate *api.Stargate, dcName string) error {
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	if len(kc.Status.Datacenters) == 0 {
		kc.Status.Datacenters = make(map[string]api.K8ssandraStatus, 0)
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

	return r.Status().Patch(ctx, kc, patch)
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraClusterReconciler) SetupWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	cb := ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraCluster{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})) // No generation changed predicate here?

	clusterLabelFilter := func(mapObj client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)
		labels := mapObj.GetLabels()
		cluster, found := labels[api.K8ssandraClusterLabel]
		if found {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mapObj.GetNamespace(), Name: cluster}})
		}
		return requests
	}

	for _, c := range clusters {
		cb = cb.Watches(source.NewKindWithCache(&cassdcapi.CassandraDatacenter{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
		cb = cb.Watches(source.NewKindWithCache(&api.Stargate{}, c.GetCache()),
			handler.EnqueueRequestsFromMapFunc(clusterLabelFilter))
	}

	return cb.Complete(r)
}
