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
	appsv1 "k8s.io/api/apps/v1"
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

		selector := k8ssandralabels.CleanedUpByLabels(kcKey)
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

		if r.deleteDeployments(ctx, kc, dcTemplate, namespace, remoteClient, logger) {
			hasErrors = true
		}

		if r.deleteServices(ctx, kc, dcTemplate, namespace, remoteClient, logger) {
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

	stargate, remoteClient, err := r.findStargateForDeletion(ctx, kcKey, cassDcName, nil)
	if err != nil {
		return result.Error(err)
	}

	if stargate != nil {
		if err = remoteClient.Delete(ctx, stargate); err != nil && !errors.IsNotFound(err) {
			return result.Error(fmt.Errorf("failed to delete Stargate for dc (%s): %v", cassDcName, err))
		}
		logger.Info("Deleted Stargate", "Stargate", utils.GetKey(stargate))
	}

	reaper, remoteClient, err := r.findReaperForDeletion(ctx, kcKey, cassDcName, remoteClient)
	if err != nil {
		return result.Error(err)
	}

	if reaper != nil {
		if err = remoteClient.Delete(ctx, reaper); err != nil && !errors.IsNotFound(err) {
			return result.Error(fmt.Errorf("failed to delete Reaper for dc (%s): %v", cassDcName, err))
		}
		logger.Info("Deleted Reaper", "Reaper", utils.GetKey(reaper))
	}

	dc, remoteClient, err := r.findDcForDeletion(ctx, kcKey, dcName, remoteClient)
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

	delete(kc.Status.Datacenters, dcName)
	logger.Info("DC deletion finished", "DC", dcName)
	return result.Continue()
}

func (r *K8ssandraClusterReconciler) findStargateForDeletion(
	ctx context.Context,
	kcKey client.ObjectKey,
	dcName string,
	remoteClient client.Client) (*stargateapi.Stargate, client.Client, error) {

	selector := k8ssandralabels.CleanedUpByLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	stargateList := &stargateapi.StargateList{}
	stargateName := kcKey.Name + "-" + dcName + "-stargate"

	if remoteClient == nil {
		for _, remoteClient := range r.ClientCache.GetAllClients() {
			err := remoteClient.List(ctx, stargateList, options)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to find Stargate (%s) for DC (%s) deletion: %v", stargateName, dcName, err)
			}
			for _, stargate := range stargateList.Items {
				if stargate.Name == stargateName {
					return &stargate, remoteClient, nil
				}
			}
		}
	} else {
		err := remoteClient.List(ctx, stargateList, options)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find Stargate (%s) for DC (%s) deletion: %v", stargateName, dcName, err)
		}

		for _, stargate := range stargateList.Items {
			if stargate.Name == stargateName {
				return &stargate, remoteClient, nil
			}
		}
	}

	return nil, nil, nil
}

func (r *K8ssandraClusterReconciler) findReaperForDeletion(
	ctx context.Context,
	kcKey client.ObjectKey,
	dcName string,
	remoteClient client.Client) (*reaperapi.Reaper, client.Client, error) {

	selector := k8ssandralabels.CleanedUpByLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	reaperList := &reaperapi.ReaperList{}
	reaperName := kcKey.Name + "-" + dcName + "-reaper"

	if remoteClient == nil {
		for _, remoteClient := range r.ClientCache.GetAllClients() {
			err := remoteClient.List(ctx, reaperList, options)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to find Reaper (%s) for DC (%s) deletion: %v", reaperName, dcName, err)
			}
			for _, reaper := range reaperList.Items {
				if reaper.Name == reaperName {
					return &reaper, remoteClient, nil
				}
			}
		}
	} else {
		err := remoteClient.List(ctx, reaperList, options)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find Reaper (%s) for DC (%s) deletion: %v", reaperName, dcName, err)
		}

		for _, reaper := range reaperList.Items {
			if reaper.Name == reaperName {
				return &reaper, remoteClient, nil
			}
		}
	}

	return nil, nil, nil
}

func (r *K8ssandraClusterReconciler) findDcForDeletion(
	ctx context.Context,
	kcKey client.ObjectKey,
	dcName string,
	remoteClient client.Client) (*cassdcapi.CassandraDatacenter, client.Client, error) {
	selector := k8ssandralabels.CleanedUpByLabels(kcKey)
	options := &client.ListOptions{LabelSelector: labels.SelectorFromSet(selector)}
	dcList := &cassdcapi.CassandraDatacenterList{}

	if remoteClient == nil {
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
	} else {
		err := remoteClient.List(ctx, dcList, options)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find CassandraDatacenter (%s) for deletion: %v", dcName, err)
		}

		for _, dc := range dcList.Items {
			if dc.Name == dcName {
				return &dc, remoteClient, nil
			}
		}
	}

	return nil, nil, nil
}

func (r *K8ssandraClusterReconciler) deleteK8ssandraConfigMaps(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcTemplate k8ssandraapi.CassandraDatacenterTemplate,
	namespace string,
	remoteClient client.Client,
	kcLogger logr.Logger,
) (hasErrors bool) {
	selector := k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.SanitizedName()})
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

func (r *K8ssandraClusterReconciler) deleteServices(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcTemplate k8ssandraapi.CassandraDatacenterTemplate,
	namespace string,
	remoteClient client.Client,
	kcLogger logr.Logger,
) (hasErrors bool) {
	selector := k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	options := client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}
	serviceList := &corev1.ServiceList{}
	if err := remoteClient.List(ctx, serviceList, &options); err != nil {
		kcLogger.Error(err, "Failed to list K8ssandra services", "Context", dcTemplate.K8sContext)
		return true
	}
	for _, rp := range serviceList.Items {
		kcLogger.Info("Deleting service", "Service", utils.GetKey(&rp))
		if err := remoteClient.Delete(ctx, &rp); err != nil {
			key := client.ObjectKey{Namespace: namespace, Name: rp.Name}
			if !errors.IsNotFound(err) {
				kcLogger.Error(err, "Failed to delete Service", "Service", key,
					"Context", dcTemplate.K8sContext)
				hasErrors = true
			}
		}
	}

	return
}

func (r *K8ssandraClusterReconciler) deleteDeployments(
	ctx context.Context,
	kc *k8ssandraapi.K8ssandraCluster,
	dcTemplate k8ssandraapi.CassandraDatacenterTemplate,
	namespace string,
	remoteClient client.Client,
	kcLogger logr.Logger,
) (hasErrors bool) {
	selector := k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	options := client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}
	deploymentList := &appsv1.DeploymentList{}
	if err := remoteClient.List(ctx, deploymentList, &options); err != nil {
		kcLogger.Error(err, "Failed to list K8ssandra deployments", "Context", dcTemplate.K8sContext)
		return true
	}
	for _, item := range deploymentList.Items {
		kcLogger.Info("Deleting deployment", "Deployment", utils.GetKey(&item))
		if err := remoteClient.Delete(ctx, &item); err != nil {
			key := client.ObjectKey{Namespace: namespace, Name: item.Name}
			if !errors.IsNotFound(err) {
				kcLogger.Error(err, "Failed to delete Deployment", "Deployment", key,
					"Context", dcTemplate.K8sContext)
				hasErrors = true
			}
		}
	}

	return
}
