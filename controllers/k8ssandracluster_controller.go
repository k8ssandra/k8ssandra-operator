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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/hash"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	resourceHashAnnotation = "k8ssandra.io/resource-hash"
)

// K8ssandraClusterReconciler reconciles a K8ssandraCluster object
type K8ssandraClusterReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	ClientCache *clientcache.ClientCache
}

//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch;create;update;delete;patch
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=stargates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,namespace="k8ssandra",resources=secrets,verbs=get;list;watch

func (r *K8ssandraClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	k8ssandra := &api.K8ssandraCluster{}
	err := r.Get(ctx, req.NamespacedName, k8ssandra)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	k8ssandra = k8ssandra.DeepCopy()

	if k8ssandra.Spec.Cassandra != nil {
		var seeds []string
		systemDistributedRF := getSystemDistributedRF(k8ssandra)
		dcNames := make([]string, 0, len(k8ssandra.Spec.Cassandra.Datacenters))

		for i, dcTemplate := range k8ssandra.Spec.Cassandra.Datacenters {
			desiredDc := newDatacenter(req.Namespace, k8ssandra.Spec.Cassandra.Cluster, dcTemplate, seeds)
			dcKey := types.NamespacedName{Namespace: desiredDc.Namespace, Name: desiredDc.Name}

		for i, template := range k8ssandra.Spec.Cassandra.Datacenters {
			desired, err := newDatacenter(req.Namespace, k8ssandra.Spec.Cassandra.Cluster, dcNames, template, seeds, systemDistributedRF)
			if err != nil {
				logger.Error(err, "Failed to CassandraDatacenter")
				return ctrl.Result{}, err
			}
			dcKey := types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}

			//if err := controllerutil.SetControllerReference(k8ssandra, desired, r.Scheme); err != nil {
			//	logger.Error(err, "failed to set owner reference", "CassandraDatacenter", key)
			//	return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			//}

			desiredDcHash := deepHashString(desiredDc)
			desiredDc.Annotations[resourceHashAnnotation] = desiredDcHash

			remoteClient, err := r.ClientCache.GetClient(req.NamespacedName, k8ssandra.Spec.K8sContextsSecret, dcTemplate.K8sContext)
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
				if actualHash, found := actualDc.Annotations[resourceHashAnnotation]; !(found && actualHash == desiredDcHash) {
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

				endpoints, err := r.resolveSeedEndpoints(ctx, actualDc, remoteClient)
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

				if err = r.updateAdditionalSeeds(ctx, k8ssandra, seeds, 0, i); err != nil {
					logger.Error(err, "Failed to update seeds")
					return ctrl.Result{}, err
				}
			} else {
				if errors.IsNotFound(err) {
					if err = remoteClient.Create(ctx, &desiredDc); err != nil {
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
					Name:      k8ssandra.Name + "-" + actualDc.Name + "-stargate",
				}

				desiredStargate := &api.Stargate{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   stargateKey.Namespace,
						Name:        stargateKey.Name,
						Annotations: map[string]string{},
					},
					Spec: api.StargateSpec{
						StargateTemplate: *dcTemplate.Stargate,
						DatacenterRef:    actualDc.Name,
					},
				}
				desiredStargateHash := deepHashString(desiredStargate)
				desiredStargate.Annotations[resourceHashAnnotation] = desiredStargateHash

				actualStargate := &api.Stargate{}

				if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
					if errors.IsNotFound(err) {
						logger.Info("Creating Stargate resource", "Stargate", stargateKey)
						if err := controllerutil.SetControllerReference(k8ssandra, desiredStargate, r.Scheme); err != nil {
							logger.Error(err, "Failed to set owner reference on Stargate resource", "Stargate", stargateKey)
							return ctrl.Result{RequeueAfter: 10 * time.Second}, err
						} else if err := remoteClient.Create(ctx, desiredStargate); err != nil {
							logger.Error(err, "Failed to create Stargate resource", "Stargate", stargateKey)
							return ctrl.Result{}, err
						} else {
							return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
						}
					} else {
						logger.Error(err, "Failed to get Stargate resource", "Stargate", stargateKey)
						return ctrl.Result{}, err
					}
				} else if actualStargateHash, found := actualStargate.Annotations[resourceHashAnnotation]; !found || actualStargateHash != desiredStargateHash {
					logger.Info("Updating Stargate resource", "Stargate", stargateKey)
					resourceVersion := actualStargate.GetResourceVersion()
					desiredStargate.DeepCopyInto(actualStargate)
					actualStargate.SetResourceVersion(resourceVersion)
					if err := controllerutil.SetControllerReference(k8ssandra, actualStargate, r.Scheme); err != nil {
						logger.Error(err, "Failed to set owner reference on Stargate resource", "Stargate", stargateKey)
						return ctrl.Result{RequeueAfter: 10 * time.Second}, err
					} else if err = remoteClient.Update(ctx, actualStargate); err != nil {
						logger.Error(err, "Failed to update Stargate resource", "Stargate", stargateKey)
						return ctrl.Result{}, err
					}
				}
			}
		}
	}
	logger.Info("Finished reconciling the k8ssandracluster")
	return ctrl.Result{}, nil
}

func newDatacenter(k8ssandraNamespace, cluster string, dcNames []string, template api.CassandraDatacenterTemplateSpec, additionalSeeds []string, systemDistributedRF int) (*cassdcapi.CassandraDatacenter, error) {
	namespace := template.Meta.Namespace
	if len(namespace) == 0 {
		namespace = k8ssandraNamespace
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
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:     cluster,
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
	}
}

func getSystemDistributedRF(k8ssandra *api.K8ssandraCluster) int {
	size := 1.0
	for _, dc := range k8ssandra.Spec.Cassandra.Datacenters {
		size := math.Min(size, float64(dc.Size))
	}
	replicationFactor := math.Min(size, float64(3.0))

	return int(replicationFactor)
}

func deepHashString(obj interface{}) string {
	hasher := sha256.New()
	hash.DeepHashObject(hasher, obj)
	hashBytes := hasher.Sum([]byte{})
	b64Hash := base64.StdEncoding.EncodeToString(hashBytes)
	return b64Hash
}

func (r *K8ssandraClusterReconciler) resolveSeedEndpoints(ctx context.Context, dc *cassdcapi.CassandraDatacenter, remoteClient client.Client) ([]string, error) {
	//ips, err := net.LookupIP(dc.GetSeedServiceName())
	//if err != nil {
	//	return nil, err
	//}

	//endpoints := make([]string, len(ips))
	//
	//for _, ip := range ips {
	//	if ip.To4() == nil {
	//		return nil, fmt.Errorf("failed to get IPv4 address for ip %s from seed service %s", ip, dc.GetSeedServiceName())
	//	}
	//	endpoints = append(endpoints, ip.String())
	//}
	//
	//return endpoints, nil

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

func (r *K8ssandraClusterReconciler) updateAdditionalSeeds(ctx context.Context, k8ssandra *api.K8ssandraCluster, seeds []string, start, end int) error {
	for i := start; i < end; i++ {
		dc, remoteClient, err := r.getDatacenterForTemplate(ctx, k8ssandra, i)
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
	// TODO make sure we do not have duplicates
	dc.Spec.AdditionalSeeds = append(dc.Spec.AdditionalSeeds, seeds...)

	return remoteClient.Patch(ctx, dc, patch)
}

func (r *K8ssandraClusterReconciler) getDatacenterForTemplate(ctx context.Context, k8ssandra *api.K8ssandraCluster, idx int) (*cassdcapi.CassandraDatacenter, client.Client, error) {
	dcTemplate := k8ssandra.Spec.Cassandra.Datacenters[idx]
	k8ssandraKey := types.NamespacedName{Namespace: k8ssandra.Namespace, Name: k8ssandra.Name}
	remoteClient, err := r.ClientCache.GetClient(k8ssandraKey, k8ssandra.Spec.K8sContextsSecret, dcTemplate.K8sContext)

	if err != nil {
		return nil, nil, err
	}

	dc := &cassdcapi.CassandraDatacenter{}
	dcKey := getDatacenterKey(dcTemplate, k8ssandraKey)
	err = remoteClient.Get(ctx, dcKey, dc)

	return dc, remoteClient, err
}

func getDatacenterKey(dcTemplate api.CassandraDatacenterTemplateSpec, k8ssandraKey types.NamespacedName) types.NamespacedName {
	if len(dcTemplate.Meta.Namespace) == 0 {
		return types.NamespacedName{Namespace: k8ssandraKey.Namespace, Name: dcTemplate.Meta.Name}
	}
	return types.NamespacedName{Namespace: dcTemplate.Meta.Namespace, Name: dcTemplate.Meta.Name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraCluster{}).
		Owns(&cassdcapi.CassandraDatacenter{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&api.Stargate{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func (r *K8ssandraClusterReconciler) SetupMultiClusterWithManager(mgr ctrl.Manager, clusters []cluster.Cluster) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraCluster{})

	for _, c := range clusters {
		builder = builder.Watches(source.NewKindWithCache(&api.K8ssandraCluster{}, c.GetCache()),
			&handler.EnqueueRequestForObject{})
	}

	return builder.Complete(r)
}
