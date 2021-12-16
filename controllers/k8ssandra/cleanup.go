package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
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

		selector := k8ssandralabels.CreatedByK8ssandraControllerLabels(kcKey)
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
