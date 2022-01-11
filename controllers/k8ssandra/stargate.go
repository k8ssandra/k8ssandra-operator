package k8ssandra

import (
	"context"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *K8ssandraClusterReconciler) reconcileStargate(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dcTemplate api.CassandraDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	logger logr.Logger,
	remoteClient client.Client,
) result.ReconcileResult {

	kcKey := client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}
	stargateTemplate := dcTemplate.Stargate.Coalesce(kc.Spec.Stargate)
	stargateKey := types.NamespacedName{
		Namespace: actualDc.Namespace,
		Name:      stargate.ResourceName(actualDc),
	}
	actualStargate := &stargateapi.Stargate{}
	logger = logger.WithValues("Stargate", stargateKey)

	if stargateTemplate != nil {
		logger.Info("Reconcile Stargate")

		desiredStargate := r.newStargate(stargateKey, kc, stargateTemplate, actualDc)
		annotations.AddHashAnnotation(desiredStargate)

		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating Stargate resource")
				if err := remoteClient.Create(ctx, desiredStargate); err != nil {
					logger.Error(err, "Failed to create Stargate resource")
					return result.Error(err)
				} else {
					return result.RequeueSoon(r.DefaultDelay)
				}
			} else {
				logger.Error(err, "Failed to get Stargate resource")
				return result.Error(err)
			}
		} else {
			if err = r.setStatusForStargate(kc, actualStargate, dcTemplate.Meta.Name); err != nil {
				logger.Error(err, "Failed to update status for stargate")
				return result.Error(err)
			}
			if !annotations.CompareHashAnnotations(desiredStargate, actualStargate) {
				logger.Info("Updating Stargate")
				resourceVersion := actualStargate.GetResourceVersion()
				desiredStargate.DeepCopyInto(actualStargate)
				actualStargate.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualStargate); err == nil {
					return result.RequeueSoon(r.DefaultDelay)
				} else {
					logger.Error(err, "Failed to update Stargate")
					return result.Error(err)
				}
			}
			if !actualStargate.Status.IsReady() {
				logger.Info("Waiting for Stargate to become ready")
				return result.RequeueSoon(r.DefaultDelay)
			}
			logger.Info("Stargate is ready")
		}
	} else {
		logger.Info("Stargate not present")

		// Test if Stargate was removed
		if err := remoteClient.Get(ctx, stargateKey, actualStargate); err != nil {
			if errors.IsNotFound(err) {
				// OK
			} else {
				logger.Error(err, "Failed to get Stargate")
				return result.Error(err)
			}
		} else if labels.IsPartOf(actualStargate, kcKey) {
			if err := remoteClient.Delete(ctx, actualStargate); err != nil {
				logger.Error(err, "Failed to delete Stargate")
				return result.Error(err)
			} else {
				r.removeStargateStatus(kc, dcTemplate.Meta.Name)
				logger.Info("Stargate deleted")
			}
		} else {
			logger.Info("Not deleting Stargate since it wasn't created by this controller")
		}
	}
	return result.Continue()
}

// TODO move to stargate package
func (r *K8ssandraClusterReconciler) newStargate(stargateKey types.NamespacedName, kc *api.K8ssandraCluster, stargateTemplate *stargateapi.StargateDatacenterTemplate, actualDc *cassdcapi.CassandraDatacenter) *stargateapi.Stargate {
	desiredStargate := &stargateapi.Stargate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   stargateKey.Namespace,
			Name:        stargateKey.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.NameLabel:                      api.NameLabelValue,
				api.PartOfLabel:                    api.PartOfLabelValue,
				api.ComponentLabel:                 api.ComponentLabelValueStargate,
				api.CreatedByLabel:                 api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterNameLabel:      kc.Name,
				api.K8ssandraClusterNamespaceLabel: kc.Namespace,
			},
		},
		Spec: stargateapi.StargateSpec{
			StargateDatacenterTemplate: *stargateTemplate,
			DatacenterRef:              corev1.LocalObjectReference{Name: actualDc.Name},
			Auth:                       kc.Spec.Auth,
		},
	}
	return desiredStargate
}

func (r *K8ssandraClusterReconciler) setStatusForStargate(kc *api.K8ssandraCluster, stargate *stargateapi.Stargate, dcName string) error {
	if len(kc.Status.Datacenters) == 0 {
		kc.Status.Datacenters = make(map[string]api.K8ssandraStatus)
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

	if kc.Status.Datacenters[dcName].Stargate.Progress == "" {
		kc.Status.Datacenters[dcName].Stargate.Progress = stargateapi.StargateProgressPending
	}
	return nil
}

func (r *K8ssandraClusterReconciler) reconcileStargateAuthSchema(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger) result.ReconcileResult {

	if !kc.HasStargates() {
		return result.Continue()
	}

	if recResult := r.versionCheck(ctx, kc); recResult.Completed() {
		return recResult
	}

	if remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext); err != nil {
		logger.Error(err, "Failed to get remote client")
		return result.Error(err)
	} else {
		dc := dcs[0]
		managementApi, err := r.ManagementApi.NewManagementApiFacade(ctx, dc, remoteClient, logger)
		if err != nil {
			logger.Error(err, "Failed to create ManagementApiFacade")
			return result.Error(err)
		}

		replication := cassandra.ComputeReplication(3, kc.Spec.Cassandra.Datacenters...)
		if err = managementApi.EnsureKeyspaceReplication(stargate.AuthKeyspace, replication); err != nil {
			logger.Error(err, "Failed to ensure keyspace replication")
			return result.Error(err)
		}

		if err = stargate.ReconcileAuthTable(managementApi, logger); err != nil {
			logger.Error(err, "Failed to reconcile Stargate auth table")
			return result.Error(err)
		}

		return result.Continue()
	}

}

func (r *K8ssandraClusterReconciler) removeStargateStatus(kc *api.K8ssandraCluster, dcName string) {
	if kdcStatus, found := kc.Status.Datacenters[dcName]; found {
		kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
			Stargate:  nil,
			Cassandra: kdcStatus.Cassandra.DeepCopy(),
			Reaper:    kdcStatus.Reaper.DeepCopy(),
		}
	}
}
