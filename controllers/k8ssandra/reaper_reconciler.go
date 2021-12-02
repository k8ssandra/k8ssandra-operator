/*


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
	"github.com/go-logr/logr"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
)

func (r *K8ssandraClusterReconciler) reconcileReaperSecrets(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	logger logr.Logger,
) error {
	logger.Info("Reconciling Reaper user secrets")
	if kc.Spec.Reaper != nil {
		kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
		cassandraUserSecretRef := kc.Spec.Reaper.CassandraUserSecretRef
		jmxUserSecretRef := kc.Spec.Reaper.JmxUserSecretRef
		if cassandraUserSecretRef == "" {
			cassandraUserSecretRef = reaper.DefaultUserSecretName(kc.Name)
		}
		if jmxUserSecretRef == "" {
			jmxUserSecretRef = reaper.DefaultJmxUserSecretName(kc.Name)
		}
		logger = logger.WithValues(
			"ReaperCassandraUserSecretRef",
			cassandraUserSecretRef,
			"ReaperJmxUserSecretRef",
			jmxUserSecretRef,
		)
		if err := secret.ReconcileSecret(ctx, r.Client, cassandraUserSecretRef, kcKey); err != nil {
			logger.Error(err, "Failed to reconcile Reaper CQL user secret")
			return err
		}
		if err := secret.ReconcileSecret(ctx, r.Client, jmxUserSecretRef, kcKey); err != nil {
			logger.Error(err, "Failed to reconcile Reaper JMX user secret")
			return err
		}
	}
	logger.Info("Reaper user secrets successfully reconciled")
	return nil
}

func (r *K8ssandraClusterReconciler) reconcileReaperSchema(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	actualDc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger,
) error {
	managementApiFacade, err := r.ManagementApi.NewManagementApiFacade(ctx, actualDc, remoteClient, logger)
	if err != nil {
		return err
	}
	keyspace := reaperapi.DefaultKeyspace
	if kc.Spec.Reaper != nil && kc.Spec.Reaper.Keyspace != "" {
		keyspace = kc.Spec.Reaper.Keyspace
	}
	return managementApiFacade.EnsureKeyspaceReplication(
		keyspace,
		cassandra.ComputeReplication(3, kc.Spec.Cassandra.Datacenters...),
	)
}

func (r *K8ssandraClusterReconciler) reconcileReaper(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) (ctrl.Result, error) {

	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	reaperTemplate := reaper.Coalesce(kc.Spec.Reaper.DeepCopy(), dcTemplate.Reaper.DeepCopy())
	reaperKey := types.NamespacedName{
		Namespace: actualDc.Namespace,
		Name:      reaper.ResourceName(kc.Name, actualDc.Name),
	}
	logger = logger.WithValues("Reaper", reaperKey)
	actualReaper := &reaperapi.Reaper{}

	if reaperTemplate != nil {

		logger.Info("Reaper present for DC " + actualDc.Name)

		desiredReaper := reaper.NewReaper(reaperKey, kc, actualDc, reaperTemplate)

		if err := remoteClient.Get(ctx, reaperKey, actualReaper); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating Reaper resource")
				if err := remoteClient.Create(ctx, desiredReaper); err != nil {
					logger.Error(err, "Failed to create Reaper resource")
					return ctrl.Result{}, err
				} else {
					return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
				}
			} else {
				logger.Error(err, "failed to retrieve reaper instance")
				return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
			}
		}

		actualReaper = actualReaper.DeepCopy()

		if err := r.setStatusForReaper(kc, actualReaper, dcTemplate.Meta.Name); err != nil {
			logger.Error(err, "Failed to update status for reaper")
			return ctrl.Result{}, err
		}

		if !utils.CompareAnnotations(actualReaper, desiredReaper, api.ResourceHashAnnotation) {
			logger.Info("Updating Reaper resource")
			resourceVersion := actualReaper.GetResourceVersion()
			desiredReaper.DeepCopyInto(actualReaper)
			actualReaper.SetResourceVersion(resourceVersion)
			if err := remoteClient.Update(ctx, actualReaper); err != nil {
				logger.Error(err, "Failed to update Reaper resource")
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		if !actualReaper.Status.IsReady() {
			logger.Info("Waiting for Reaper to become ready")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		logger.Info("Reaper is ready")
		return ctrl.Result{}, nil

	} else {

		logger.Info("Reaper not present for DC " + actualDc.Name)

		// Test if Reaper was removed
		if err := remoteClient.Get(ctx, reaperKey, actualReaper); err != nil {
			if errors.IsNotFound(err) {
				r.removeReaperStatus(kc, dcTemplate.Meta.Name)
			} else {
				logger.Error(err, "Failed to get Reaper resource")
				return ctrl.Result{}, err
			}
		} else if utils.IsCreatedByK8ssandraController(actualReaper, kcKey) {
			if err = remoteClient.Delete(ctx, actualReaper); err != nil {
				logger.Error(err, "Failed to delete Reaper resource")
				return ctrl.Result{}, err
			} else {
				r.removeReaperStatus(kc, dcTemplate.Meta.Name)
				logger.Info("Reaper deleted")
			}
		} else {
			logger.Info("Not deleting Reaper since it wasn't created by this controller")
		}
		return ctrl.Result{}, nil
	}
}

func (r *K8ssandraClusterReconciler) deleteReapers(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	namespace string,
	remoteClient client.Client,
	kcLogger logr.Logger,
) (hasErrors bool) {
	selector := utils.CreatedByK8ssandraControllerLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	reaperList := &reaperapi.ReaperList{}
	options := client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}
	if err := remoteClient.List(ctx, reaperList, &options); err != nil {
		kcLogger.Error(err, "Failed to list Reaper objects", "Context", dcTemplate.K8sContext)
		return true
	}
	for _, rp := range reaperList.Items {
		if err := remoteClient.Delete(ctx, &rp); err != nil {
			key := client.ObjectKey{Namespace: namespace, Name: rp.Name}
			if !errors.IsNotFound(err) {
				kcLogger.Error(err, "Failed to delete Reaper", "Reaper", key,
					"Context", dcTemplate.K8sContext)
				hasErrors = true
			}
		}
	}
	return
}

func (r *K8ssandraClusterReconciler) setStatusForReaper(kc *api.K8ssandraCluster, reaper *reaperapi.Reaper, dcName string) error {
	if len(kc.Status.Datacenters) == 0 {
		kc.Status.Datacenters = make(map[string]api.K8ssandraStatus)
	}
	kdcStatus, found := kc.Status.Datacenters[dcName]
	if found {
		kdcStatus.Reaper = reaper.Status.DeepCopy()
		kc.Status.Datacenters[dcName] = kdcStatus
	} else {
		kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
			Reaper: reaper.Status.DeepCopy(),
		}
	}
	return nil
}

func (r *K8ssandraClusterReconciler) removeReaperStatus(kc *api.K8ssandraCluster, dcName string) {
	if kdcStatus, found := kc.Status.Datacenters[dcName]; found {
		kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
			Reaper:    nil,
			Cassandra: kdcStatus.Cassandra.DeepCopy(),
			Stargate:  kdcStatus.Stargate.DeepCopy(),
		}
	}
}
