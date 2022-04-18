package k8ssandra

import (
	"context"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	kerrors "github.com/k8ssandra/k8ssandra-operator/pkg/errors"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"k8s.io/apimachinery/pkg/api/errors"
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
	stargateKey := types.NamespacedName{
		Namespace: actualDc.Namespace,
		Name:      stargate.ResourceName(actualDc),
	}
	logger = logger.WithValues("Stargate", stargateKey)

	stargateTemplate := dcTemplate.Stargate.MergeWith(kc.Spec.Stargate)

	if actualDc.Spec.Stopped && stargateTemplate != nil {
		logger.Info("DC is stopped: skipping Stargate deployment")
		stargateTemplate = nil
	}

	actualStargate := &stargateapi.Stargate{}

	if stargateTemplate != nil {
		logger.Info("Reconcile Stargate")
		stargateTemplate.SecretsProvider = kc.Spec.SecretsProvider
		desiredStargate := stargate.NewStargate(stargateKey, kc, stargateTemplate, actualDc, dcTemplate, logger)
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
	mgmtApi cassandra.ManagementApiFacade,
	logger logr.Logger) result.ReconcileResult {

	if !kc.HasStargates() {
		return result.Continue()
	}

	if recResult := r.versionCheck(ctx, kc); recResult.Completed() {
		return recResult
	}

	datacenters := kc.GetInitializedDatacenters()
	replication := cassandra.ComputeReplicationFromDatacenters(3, kc.Spec.ExternalDatacenters, datacenters...)

	if err := stargate.ReconcileAuthKeyspace(mgmtApi, replication, logger); err != nil {
		if kerrors.IsSchemaDisagreement(err) {
			return result.RequeueSoon(r.DefaultDelay)
		}
		return result.Error(err)
	}

	return result.Continue()
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
