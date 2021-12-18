package k8ssandra

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reaper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *K8ssandraClusterReconciler) reconcileDatacenters(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) (result.ReconcileResult, []*cassdcapi.CassandraDatacenter) {
	kcKey := utils.GetKey(kc)

	systemReplication, err := r.checkSystemReplication(ctx, kc, logger)
	if err != nil {
		logger.Error(err, "System replication check failed")
		return result.Error(err), nil
	}

	actualDcs := make([]*cassdcapi.CassandraDatacenter, 0, len(kc.Spec.Cassandra.Datacenters))

	seeds, err := r.findSeeds(ctx, kc, logger)
	if err != nil {
		logger.Error(err, "Failed to find seed nodes")
		return result.Error(err), actualDcs
	}

	// Reconcile CassandraDatacenter objects only
	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {

		if !secret.HasReplicatedSecrets(ctx, r.Client, kcKey, dcTemplate.K8sContext) {
			// ReplicatedSecret has not replicated yet, wait until it has
			logger.Info("Waiting for replication to complete")
			return result.RequeueSoon(r.DefaultDelay), actualDcs
		}

		// Note that it is necessary to use a copy of the CassandraClusterTemplate because
		// its fields are pointers, and without the copy we could end of with shared
		// references that would lead to unexpected and incorrect values.
		dcConfig := cassandra.Coalesce(kc.Spec.Cassandra.DeepCopy(), dcTemplate.DeepCopy())
		cassandra.ApplySystemReplication(dcConfig, *systemReplication)
		if !cassandra.IsCassandra3(dcConfig.ServerVersion) && kc.HasStargates() {
			// if we're not running Cassandra 3.11 and have Stargate pods, we need to allow alter RF during range movements
			cassandra.AllowAlterRfDuringRangeMovement(dcConfig)
		}
		reaperTemplate := reaper.Coalesce(kc.Spec.Reaper.DeepCopy(), dcTemplate.Reaper.DeepCopy())
		if reaperTemplate != nil {
			reaper.AddReaperSettingsToDcConfig(reaperTemplate, dcConfig)
		}
		// Create Medusa related objects
		if medusaResult := r.ReconcileMedusa(ctx, dcConfig, dcTemplate, kc, logger); medusaResult.Completed() {
			return medusaResult, actualDcs
		}
		desiredDc, err := cassandra.NewDatacenter(kcKey, dcConfig)
		dcKey := types.NamespacedName{Namespace: desiredDc.Namespace, Name: desiredDc.Name}
		logger := logger.WithValues("CassandraDatacenter", dcKey, "K8SContext", dcTemplate.K8sContext)

		if err != nil {
			logger.Error(err, "Failed to create new CassandraDatacenter")
			return result.Error(err), actualDcs
		}

		annotations.AddHashAnnotation(desiredDc)

		actualDc := &cassdcapi.CassandraDatacenter{}

		remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
		if err != nil {
			logger.Error(err, "Failed to get remote client")
			return result.Error(err), actualDcs
		}

		if recResult := r.reconcileSeedsEndpoints(ctx, desiredDc, seeds, remoteClient, logger); recResult.Completed() {
			return recResult, actualDcs
		}

		if err = remoteClient.Get(ctx, dcKey, actualDc); err == nil {
			// cassdc already exists, we'll update it
			if err = r.setStatusForDatacenter(kc, actualDc); err != nil {
				logger.Error(err, "Failed to update status for datacenter")
				return result.Error(err), actualDcs
			}

			if !annotations.CompareHashAnnotations(actualDc, desiredDc) {
				logger.Info("Updating datacenter")

				if actualDc.Spec.SuperuserSecretName != desiredDc.Spec.SuperuserSecretName {
					// If actualDc is created with SuperuserSecretName, it can't be changed anymore. We should reject all changes coming from K8ssandraCluster
					desiredDc.Spec.SuperuserSecretName = actualDc.Spec.SuperuserSecretName
					err = fmt.Errorf("tried to update superuserSecretName in K8ssandraCluster")
					logger.Error(err, "SuperuserSecretName is immutable, reverting to existing value in CassandraDatacenter")
				}

				actualDc = actualDc.DeepCopy()
				resourceVersion := actualDc.GetResourceVersion()
				desiredDc.DeepCopyInto(actualDc)
				actualDc.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualDc); err != nil {
					logger.Error(err, "Failed to update datacenter")
					return result.Error(err), actualDcs
				}
			}

			if !cassandra.DatacenterReady(actualDc) {
				logger.Info("Waiting for datacenter to become ready")
				return result.RequeueSoon(r.DefaultDelay), actualDcs
			}

			logger.Info("The datacenter is ready")

			actualDcs = append(actualDcs, actualDc)

			if recResult := r.updateReplicationOfSystemKeyspaces(ctx, kc, desiredDc, remoteClient, logger); recResult.Completed() {
				return recResult, actualDcs
			}

		} else {
			if errors.IsNotFound(err) {
				// cassdc doesn't exist, we'll create it
				if err = remoteClient.Create(ctx, desiredDc); err != nil {
					logger.Error(err, "Failed to create datacenter")
					return result.Error(err), actualDcs
				}
				return result.RequeueSoon(r.DefaultDelay), actualDcs
			} else {
				logger.Error(err, "Failed to get datacenter")
				return result.Error(err), actualDcs
			}
		}
	}

	// If we reach this point all CassandraDatacenters are ready. We only set the
	// CassandraInitialized condition if it is unset, i.e., only once. This allows us to
	// distinguish whether we are deploying a CassandraDatacenter as part of a new cluster
	// or as part of an existing cluster.
	if kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionUnknown {
		now := metav1.Now()
		(&kc.Status).SetCondition(api.K8ssandraClusterCondition{
			Type:               api.CassandraInitialized,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &now,
		})
	}

	return result.Continue(), actualDcs
}

func (r *K8ssandraClusterReconciler) setStatusForDatacenter(kc *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter) error {
	if len(kc.Status.Datacenters) == 0 {
		kc.Status.Datacenters = make(map[string]api.K8ssandraStatus, 0)
	}

	kdcStatus, found := kc.Status.Datacenters[dc.Name]

	if found {
		dc.Status.DeepCopyInto(kdcStatus.Cassandra)
	} else {
		kc.Status.Datacenters[dc.Name] = api.K8ssandraStatus{
			Cassandra: dc.Status.DeepCopy(),
		}
	}

	return nil
}
