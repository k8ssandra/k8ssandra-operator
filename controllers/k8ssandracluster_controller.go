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
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/util/hash"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
)

const (
	resourceHashAnnotation = "k8ssandra.io/resource-hash"
)

// K8ssandraClusterReconciler reconciles a K8ssandraCluster object
type K8ssandraClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8ssandra.io,namespace="k8ssandra",resources=k8ssandraclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch;create;update;delete;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the K8ssandraCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
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
		for _, template := range k8ssandra.Spec.Cassandra.Datacenters {
			key := types.NamespacedName{Namespace: template.Meta.Namespace, Name: template.Meta.Name}
			desired := &cassdcapi.CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					// TODO Check to see if Meta.Namespace is set
					Namespace:   req.Namespace,
					Name:        template.Meta.Name,
					Annotations: map[string]string{},
				},
				Spec: template.Spec,
			}

			desiredHash := deepHashString(desired)
			desired.Annotations[resourceHashAnnotation] = deepHashString(desiredHash)

			actual := &cassdcapi.CassandraDatacenter{}

			// TODO set controller reference

			if err = r.Get(ctx, key, actual); err == nil {
				if actualHash, found := actual.Annotations[resourceHashAnnotation]; !(found && actualHash == desiredHash) {
					logger.Info("Updating datacenter", "CassandraDatacenter", key)
					actual = actual.DeepCopy()
					desired.DeepCopyInto(actual)

					if err = r.Update(ctx, actual); err != nil {
						logger.Error(err, "Failed to update datacenter", "CassandraDatacenter", key)
						return ctrl.Result{RequeueAfter: 10 * time.Second}, err
					}
				}
			} else {
				if errors.IsNotFound(err) {
					logger.Info("Creating datacenter", "CassandraDatacenter", key)
					if err = r.Create(ctx, desired); err != nil {
						logger.Error(err, "Failed to create datacenter", "CassandraDatacenter", key)
						return ctrl.Result{RequeueAfter: 10 * time.Second}, err
					}
				} else {
					logger.Error(err, "Failed to get datacenter", "CassandraDatacenter", key)
					return ctrl.Result{RequeueAfter: 10 * time.Second}, err
				}
			}
		}
	}
	logger.Info("Finished reconciling datacenters")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *K8ssandraClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.K8ssandraCluster{}).
		Complete(r)
}

func deepHashString(obj interface{}) string {
	hasher := sha256.New()
	hash.DeepHashObject(hasher, obj)
	hashBytes := hasher.Sum([]byte{})
	b64Hash := base64.StdEncoding.EncodeToString(hashBytes)
	return b64Hash
}
