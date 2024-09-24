package k8ssandra

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/k8ssandra"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// checkDeletion performs cleanup when a K8ssandraCluster object is deleted. All objects
// that are logically part of the K8ssandraCluster are deleted before removing its
// finalizer.
func (r *K8ssandraClusterReconciler) checkDeletion(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	if kc.DeletionTimestamp == nil {
		return result.Continue()
	}

	if !controllerutil.ContainsFinalizer(kc, k8ssandraClusterFinalizer) {
		return result.Done()
	}

	logger.Info("Starting deletion")

	hasErrors := false

	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		namespace := utils.FirstNonEmptyString(dcTemplate.Meta.Namespace, kc.Namespace)

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

// checkFinalizer ensures that the K8ssandraCluster has a finalizer.
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

func (r *K8ssandraClusterReconciler) checkDcDeletion(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) result.ReconcileResult {
	dcName, dcNameOverride := k8ssandra.GetDatacenterForDecommission(kc)
	if dcName == "" {
		return result.Continue()
	}

	status := kc.Status.Datacenters[dcName]

	switch kc.Status.Datacenters[dcName].DecommissionProgress {
	case api.DecommNone:
		logger.Info("Preparing to updating replication for DC decommission", "DC", dcName)
		status.DecommissionProgress = api.DecommUpdatingReplication
		kc.Status.Datacenters[dcName] = status
		return result.Continue()
	case api.DecommUpdatingReplication:
		logger.Info("Waiting for replication updates for DC decommission to complete", "DC", dcName)
		return result.Continue()
	default:
		logger.Info("Proceeding with DC deletion", "DC", dcName)

		cassDcName := dcName
		if dcNameOverride != "" {
			cassDcName = dcNameOverride
		}
		return r.deleteDc(ctx, kc, dcName, cassDcName, logger)
	}
}

func (r *K8ssandraClusterReconciler) deleteDc(ctx context.Context, kc *api.K8ssandraCluster, dcName string, cassDcName string, logger logr.Logger) result.ReconcileResult {
	kcKey := utils.GetKey(kc)

	dc, remoteClient, err := r.findDcForDeletion(ctx, kcKey, dcName)
	if err != nil {
		return result.Error(err)
	}

	if dc != nil {
		if err := r.deleteContactPointsService(ctx, kc, dc, logger); err != nil {
			return result.Error(err)
		}

		if dc.GetConditionStatus(cassdcapi.DatacenterDecommission) == corev1.ConditionTrue {
			logger.Info("CassandraDatacenter decommissioning in progress", "CassandraDatacenter", utils.GetKey(dc))
			// There is no need to requeue here. Reconciliation will be trigger by updates made by cass-operator.
			return result.Done()
		}

		if !annotations.HasAnnotationWithValue(dc, cassdcapi.DecommissionOnDeleteAnnotation, "true") {
			patch := client.MergeFrom(dc.DeepCopy())
			annotations.AddAnnotation(dc, cassdcapi.DecommissionOnDeleteAnnotation, "true")
			if err = remoteClient.Patch(ctx, dc, patch); err != nil {
				return result.Error(fmt.Errorf("failed to add %s annotation to dc: %v", cassdcapi.DecommissionOnDeleteAnnotation, err))
			}
		}

		if err = remoteClient.Delete(ctx, dc); err != nil && !errors.IsNotFound(err) {
			return result.Error(fmt.Errorf("failed to delete CassandraDatacenter (%s): %v", dcName, err))
		}
		logger.Info("Deleted CassandraDatacenter", "CassandraDatacenter", utils.GetKey(dc))
		// There is no need to requeue here. Reconciliation will be trigger by updates made by cass-operator.
		return result.Done()
	}

	delete(kc.Status.Datacenters, dcName)
	logger.Info("DC deletion finished", "DC", dcName)
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) findDcForDeletion(
	ctx context.Context,
	kcKey client.ObjectKey,
	dcName string,
) (*cassdcapi.CassandraDatacenter, client.Client, error) {
	selector := k8ssandralabels.CleanedUpByLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	dcList := &cassdcapi.CassandraDatacenterList{}

	for _, remoteClient := range r.ClientCache.GetAllClients() {
		err := remoteClient.List(ctx, dcList, options)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to CassandraDatacenter (%s) for DC (%s) deletion: %v", dcName, dcName, err)
		}
		for _, dc := range dcList.Items {
			if dc.Name == dcName {
				return &dc, remoteClient, nil
			}
		}
	}

	return nil, nil, nil
}

// setDcOwnership loads the remote resource identified by controlledKey, sets dc as its owner, and writes it back. If
// the remote resource does not exist, this is a no-op.
func setDcOwnership[T client.Object](
	ctx context.Context,
	dc *cassdcapi.CassandraDatacenter,
	controlledKey client.ObjectKey,
	controlled T,
	remoteClient client.Client,
	scheme *runtime.Scheme,
	logger logr.Logger,
) result.ReconcileResult {
	if err := remoteClient.Get(ctx, controlledKey, controlled); err != nil {
		if errors.IsNotFound(err) {
			return result.Continue()
		}
		logger.Error(err, "Failed to get controlled resource", "key", controlledKey)
		return result.Error(err)
	}
	if controllerutil.HasControllerReference(controlled) {
		// Assume this is us from a previous reconcile loop
		return result.Continue()
	}
	if err := controllerutil.SetControllerReference(dc, controlled, scheme); err != nil {
		logger.Error(err, "Failed to set DC owner reference", "key", controlledKey)
		return result.Error(err)
	}
	if err := remoteClient.Update(ctx, controlled); err != nil {
		logger.Error(err, "Failed to update controlled resource", "key", controlledKey)
		return result.Error(err)
	}
	return result.Continue()
}
