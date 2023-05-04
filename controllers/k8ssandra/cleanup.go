package k8ssandra

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/k8ssandra"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

		selector := k8ssandralabels.PartOfLabels(kcKey)
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

		if r.deleteK8ssandraConfigMaps(ctx, kc, dcTemplate, namespace, remoteClient, logger) {
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
	dcName := k8ssandra.GetDatacenterForDecommission(kc)
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
		return r.deleteDc(ctx, kc, dcName, logger)
	}
}

func (r *K8ssandraClusterReconciler) deleteDc(ctx context.Context, kc *api.K8ssandraCluster, dcName string, logger logr.Logger) result.ReconcileResult {

	// delete Stargate
	if result := r.deleteStargate(ctx, kc, dcName, logger); result != nil {
		return result
	}

	// delete Reaper
	if result := r.deleteReaper(ctx, kc, dcName, logger); result != nil {
		return result
	}

	// delete the DC
	if result := r.deleteDatacenter(ctx, kc, dcName, logger); result != nil {
		return result
	}
	delete(kc.Status.Datacenters, dcName)
	logger.Info("DC deletion finished", "DC", dcName)
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) findStargateForDeletion(
	ctx context.Context,
	kcKey client.ObjectKey,
	dcName string,
	remoteClient client.Client) (*stargateapi.Stargate, error) {

	if remoteClient == nil {
		return nil, nil
	}
	stargateName := kcKey.Name + "-" + dcName + "-stargate"
	selector := k8ssandralabels.PartOfLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	stargateList := &stargateapi.StargateList{}

	err := remoteClient.List(ctx, stargateList, options)
	if err != nil {
		return nil, fmt.Errorf("failed to find Stargate (%s) for DC (%s) deletion: %v", stargateName, dcName, err)
	}
	for _, stargate := range stargateList.Items {
		if stargate.Name == stargateName {
			return &stargate, nil
		}
	}
	return nil, nil
}

func (r *K8ssandraClusterReconciler) findReaperForDeletion(
	ctx context.Context,
	kcKey client.ObjectKey,
	dcName string,
	remoteClient client.Client) (*reaperapi.Reaper, error) {

	if remoteClient == nil {
		return nil, nil
	}
	reaperName := kcKey.Name + "-" + dcName + "-reaper"
	selector := k8ssandralabels.PartOfLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	reaperList := &reaperapi.ReaperList{}

	err := remoteClient.List(ctx, reaperList, options)
	if err != nil {
		return nil, fmt.Errorf("failed to find Reaper (%s) for DC (%s) deletion: %v", reaperName, dcName, err)
	}
	for _, reaper := range reaperList.Items {
		if reaper.Name == reaperName {
			return &reaper, nil
		}
	}
	return nil, nil
}

func (r *K8ssandraClusterReconciler) findDcForDeletion(
	ctx context.Context,
	kcKey client.ObjectKey,
	dcName string,
	remoteClient client.Client) (*cassdcapi.CassandraDatacenter, error) {

	if remoteClient == nil {
		return nil, nil
	}
	selector := k8ssandralabels.PartOfLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	dcList := &cassdcapi.CassandraDatacenterList{}

	err := remoteClient.List(ctx, dcList, options)
	if err != nil {
		return nil, fmt.Errorf("failed to find CassandraDatacenter (%s) for deletion: %v", dcName, err)
	}

	for _, dc := range dcList.Items {
		if dc.Name == dcName {
			return &dc, nil
		}
	}
	return nil, nil
}

func (r *K8ssandraClusterReconciler) deleteK8ssandraConfigMaps(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcTemplate k8ssandraapi.CassandraDatacenterTemplate,
	namespace string,
	remoteClient client.Client,
	kcLogger logr.Logger,
) (hasErrors bool) {
	selector := k8ssandralabels.PartOfLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.SanitizedName()})
	configMaps := &corev1.ConfigMapList{}
	options := client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}
	if err := remoteClient.List(ctx, configMaps, &options); err != nil {
		kcLogger.Error(err, "Failed to list ConfigMap objects", "Context", dcTemplate.K8sContext)
		return true
	}
	for _, rp := range configMaps.Items {
		if err := remoteClient.Delete(ctx, &rp); err != nil {
			key := client.ObjectKey{Namespace: namespace, Name: rp.Name}
			if !apierrors.IsNotFound(err) {
				kcLogger.Error(err, "Failed to delete configmap", "ConfigMap", key,
					"Context", dcTemplate.K8sContext)
				hasErrors = true
			}
		}
	}
	return
}

func (r *K8ssandraClusterReconciler) deleteStargate(ctx context.Context, kc *api.K8ssandraCluster, dcName string, logger logr.Logger) result.ReconcileResult {

	kcKey := utils.GetKey(kc)
	remoteClient, err := r.findRemoteClientForStargate(ctx, kcKey, dcName)
	if err != nil {
		return result.Error(err)
	}

	stargate, err := r.findStargateForDeletion(ctx, kcKey, dcName, remoteClient)
	if err != nil {
		return result.Error(err)
	}

	if stargate != nil {
		if err = remoteClient.Delete(ctx, stargate); err != nil && !errors.IsNotFound(err) {
			return result.Error(fmt.Errorf("failed to delete Stargate for dc (%s): %v", dcName, err))
		}
		logger.Info("Deleted Stargate", "Stargate", utils.GetKey(stargate))
	}
	return nil
}

func (r *K8ssandraClusterReconciler) deleteReaper(ctx context.Context, kc *api.K8ssandraCluster, dcName string, logger logr.Logger) result.ReconcileResult {

	kcKey := utils.GetKey(kc)
	remoteClient, err := r.findRemoteClientForReaper(ctx, kcKey, dcName)
	if err != nil {
		return result.Error(err)
	}

	reaper, err := r.findReaperForDeletion(ctx, kcKey, dcName, remoteClient)
	if err != nil {
		return result.Error(err)
	}

	if reaper != nil {
		if err = remoteClient.Delete(ctx, reaper); err != nil && !errors.IsNotFound(err) {
			return result.Error(fmt.Errorf("failed to delete Reaper for dc (%s): %v", dcName, err))
		}
		logger.Info("Deleted Reaper", "Reaper", utils.GetKey(reaper))
	}
	return nil
}

func (r *K8ssandraClusterReconciler) deleteDatacenter(ctx context.Context, kc *api.K8ssandraCluster, dcName string, logger logr.Logger) result.ReconcileResult {

	kcKey := utils.GetKey(kc)
	remoteClient, err := r.findRemoteClientForDc(ctx, kcKey, dcName)
	if err != nil {
		return result.Error(err)
	}

	dc, err := r.findDcForDeletion(ctx, kcKey, dcName, remoteClient)
	if err != nil {
		return result.Error(err)
	}

	if dc != nil {
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
	return nil
}

func (r *K8ssandraClusterReconciler) findRemoteClientForReaper(ctx context.Context, kcKey client.ObjectKey, dcName string) (client.Client, error) {
	reaperName := kcKey.Name + "-" + dcName + "-reaper"
	selector := k8ssandralabels.PartOfLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	reaperList := &reaperapi.ReaperList{}

	for _, remoteClient := range r.ClientCache.GetAllClients() {
		err := remoteClient.List(ctx, reaperList, options)
		if err != nil {
			return nil, fmt.Errorf("failed to find Reaper (%s) for DC (%s) deletion: %v", reaperName, dcName, err)
		}
		for _, reaper := range reaperList.Items {
			if reaper.Name == reaperName {
				return remoteClient, nil
			}
		}
	}
	return nil, nil
}

func (r *K8ssandraClusterReconciler) findRemoteClientForStargate(ctx context.Context, kcKey client.ObjectKey, dcName string) (client.Client, error) {
	stargateName := kcKey.Name + "-" + dcName + "-stargate"
	selector := k8ssandralabels.PartOfLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	stargateList := &stargateapi.StargateList{}

	for _, remoteClient := range r.ClientCache.GetAllClients() {
		err := remoteClient.List(ctx, stargateList, options)
		if err != nil {
			return nil, fmt.Errorf("failed to find Stargate (%s) for DC (%s) deletion: %v", stargateName, dcName, err)
		}
		for _, stargate := range stargateList.Items {
			if stargate.Name == stargateName {
				return remoteClient, nil
			}
		}
	}
	return nil, nil
}

func (r *K8ssandraClusterReconciler) findRemoteClientForDc(ctx context.Context, kcKey client.ObjectKey, dcName string) (client.Client, error) {
	selector := k8ssandralabels.PartOfLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	dcList := &cassdcapi.CassandraDatacenterList{}

	for _, remoteClient := range r.ClientCache.GetAllClients() {
		err := remoteClient.List(ctx, dcList, options)
		if err != nil {
			return nil, fmt.Errorf("failed to find CassandraDatacenter (%s) for deletion: %v", dcName, err)
		}
		for _, reaper := range dcList.Items {
			if reaper.Name == dcName {
				return remoteClient, nil
			}
		}
	}
	return nil, nil
}
