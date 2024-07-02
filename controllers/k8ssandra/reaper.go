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
	"fmt"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	kerrors "github.com/k8ssandra/k8ssandra-operator/pkg/errors"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileReaperSchema(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	mgmtApi cassandra.ManagementApiFacade,
	logger logr.Logger) result.ReconcileResult {

	if kc.Spec.Reaper == nil {
		return result.Continue()
	}

	logger.Info("Reconciling Reaper schema")

	if recResult := r.versionCheck(ctx, kc); recResult.Completed() {
		return recResult
	}

	keyspace := getReaperKeyspace(kc)

	datacenters := kc.GetInitializedDatacenters()
	err := mgmtApi.EnsureKeyspaceReplication(
		keyspace,
		cassandra.ComputeReplicationFromDatacenters(3, kc.Spec.ExternalDatacenters, datacenters...),
	)
	if err != nil {
		if kerrors.IsSchemaDisagreement(err) {
			return result.RequeueSoon(r.DefaultDelay)
		}
		logger.Error(err, "Failed to ensure keyspace replication")
		return result.Error(err)
	}

	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileReaper(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) result.ReconcileResult {

	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	reaperKey := types.NamespacedName{
		Namespace: actualDc.Namespace,
		Name:      reaper.DefaultResourceName(actualDc),
	}
	logger = logger.WithValues("Reaper", reaperKey)

	reaperTemplate := kc.Spec.Reaper.DeepCopy()
	if reaperTemplate != nil {
		if reaperTemplate.DeploymentMode == reaperapi.DeploymentModeSingle && getSingleReaperDcName(kc) != actualDc.Name {
			logger.Info("DC is not Reaper DC: skipping Reaper deployment")
			reaperTemplate = nil
		}
		if actualDc.Spec.Stopped {
			logger.Info("DC is stopped: skipping Reaper deployment")
			reaperTemplate = nil
		}
	}

	// we might have nil-ed the template because a DC got stopped, so we need to re-check
	if reaperTemplate != nil {
		if reaperTemplate.HasReaperRef() {
			logger.Info("ReaperRef present, registering with referenced Reaper instead of creating a new one")
			return r.addClusterToExternalReaper(ctx, kc, actualDc, logger)
		}
	}

	if reaperTemplate != nil && reaperTemplate.StorageType == reaperapi.StorageTypeLocal && reaperTemplate.DeploymentMode == reaperapi.DeploymentModePerDc {
		err := fmt.Errorf("reaper with 'local' storage cannot have multiple instances needed for 'per-dc' deployment mode")
		return result.Error(err)
	}

	actualReaper := &reaperapi.Reaper{}

	if reaperTemplate != nil {
		reaperTemplate.SecretsProvider = kc.Spec.SecretsProvider

		logger.Info("Reaper present for DC " + actualDc.DatacenterName())

		desiredReaper, err := reaper.NewReaper(reaperKey, kc, actualDc, reaperTemplate, logger)

		if err != nil {
			logger.Error(err, "failed to create Reaper API object")
			return result.Error(err)
		}

		if err := remoteClient.Get(ctx, reaperKey, actualReaper); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating Reaper resource")
				if err := remoteClient.Create(ctx, desiredReaper); err != nil {
					logger.Error(err, "Failed to create Reaper resource")
					return result.Error(err)
				} else {
					return result.RequeueSoon(r.DefaultDelay)
				}
			} else {
				logger.Error(err, "failed to retrieve reaper instance")
				return result.Error(err)
			}
		}

		actualReaper = actualReaper.DeepCopy()

		if err := r.setStatusForReaper(kc, actualReaper, dcTemplate.Meta.Name); err != nil {
			logger.Error(err, "Failed to update status for reaper")
			return result.Error(err)
		}

		if !annotations.CompareHashAnnotations(actualReaper, desiredReaper) {
			logger.Info("Updating Reaper resource")
			resourceVersion := actualReaper.GetResourceVersion()
			desiredReaper.DeepCopyInto(actualReaper)
			actualReaper.SetResourceVersion(resourceVersion)
			if err := remoteClient.Update(ctx, actualReaper); err != nil {
				logger.Error(err, "Failed to update Reaper resource")
				return result.Error(err)
			}
			return result.RequeueSoon(r.DefaultDelay)
		}

		if !actualReaper.Status.IsReady() {
			logger.Info("Waiting for Reaper to become ready")
			return result.RequeueSoon(r.DefaultDelay)
		}

		logger.Info("Reaper is ready")
		return result.Continue()

	} else {

		logger.Info("Reaper not present for DC " + actualDc.DatacenterName())

		// Test if Reaper was removed
		if err := remoteClient.Get(ctx, reaperKey, actualReaper); err != nil {
			if errors.IsNotFound(err) {
				r.removeReaperStatus(kc, dcTemplate.Meta.Name)
			} else {
				logger.Error(err, "Failed to get Reaper resource")
				return result.Error(err)
			}
		} else if k8ssandralabels.IsCleanedUpBy(actualReaper, kcKey) {
			if err = remoteClient.Delete(ctx, actualReaper); err != nil {
				logger.Error(err, "Failed to delete Reaper resource")
				return result.Error(err)
			} else {
				r.removeReaperStatus(kc, dcTemplate.Meta.Name)
				logger.Info("Reaper deleted")
			}
		} else {
			logger.Info("Not deleting Reaper since it wasn't created by this controller")
		}
		return result.Continue()
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
	selector := k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
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

func getReaperKeyspace(kc *api.K8ssandraCluster) string {
	keyspace := reaperapi.DefaultKeyspace
	if kc.Spec.Reaper != nil && kc.Spec.Reaper.Keyspace != "" {
		keyspace = kc.Spec.Reaper.Keyspace
	}
	return keyspace
}

func getSingleReaperDcName(kc *api.K8ssandraCluster) string {
	for _, dc := range kc.Spec.Cassandra.Datacenters {
		if !dc.Stopped {
			return dc.Meta.Name
		}
	}
	return ""
}

func (r *K8ssandraClusterReconciler) addClusterToExternalReaper(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
) result.ReconcileResult {
	manager := reaper.NewManager()
	manager.SetK8sClient(r)
	if username, password, err := manager.GetUiCredentials(ctx, kc.Spec.Reaper.UiUserSecretRef, kc.Namespace); err != nil {
		logger.Error(err, "Failed to get Reaper UI user secret")
		return result.Error(err)
	} else {
		if err = manager.ConnectWithReaperRef(ctx, kc, username, password); err != nil {
			logger.Error(err, "Failed to connect to external Reaper")
			return result.Error(err)
		}
		if err = manager.AddClusterToReaper(ctx, actualDc); err != nil {
			logger.Error(err, "Failed to add cluster to external Reaper")
			return result.Error(err)
		}
	}
	return result.Continue()
}
