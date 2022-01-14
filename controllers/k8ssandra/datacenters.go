package k8ssandra

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	for idx, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		if !secret.HasReplicatedSecrets(ctx, r.Client, kcKey, dcTemplate.K8sContext) {
			// ReplicatedSecret has not replicated yet, wait until it has
			logger.Info("Waiting for replication to complete")
			return result.RequeueSoon(r.DefaultDelay), actualDcs
		}

		// Note that it is necessary to use a copy of the CassandraClusterTemplate because
		// its fields are pointers, and without the copy we could end of with shared
		// references that would lead to unexpected and incorrect values.
		dcConfig := cassandra.Coalesce(kc.Name, kc.Spec.Cassandra.DeepCopy(), dcTemplate.DeepCopy())
		cassandra.ApplyAuth(dcConfig, kc.Spec.IsAuthEnabled())

		// This is only really required when auth is enabled, but it doesn't hurt to apply system replication on
		// unauthenticated clusters.
		cassandra.ApplySystemReplication(dcConfig, *systemReplication)

		if !cassandra.IsCassandra3(dcConfig.ServerVersion) && kc.HasStargates() {
			// if we're not running Cassandra 3.11 and have Stargate pods, we need to allow alter RF during range movements
			cassandra.AllowAlterRfDuringRangeMovement(dcConfig)
		}
		reaperTemplate := reaper.Coalesce(kc.Spec.Reaper.DeepCopy(), dcTemplate.Reaper.DeepCopy())
		if reaperTemplate != nil {
			reaper.AddReaperSettingsToDcConfig(reaperTemplate, dcConfig, kc.Spec.IsAuthEnabled())
		}
		// Create Medusa related objects
		if medusaResult := r.ReconcileMedusa(ctx, dcConfig, dcTemplate, kc, logger); medusaResult.Completed() {
			return medusaResult, actualDcs
		}
		desiredDc, err := cassandra.NewDatacenter(kcKey, dcConfig)
		if err != nil {
			logger.Error(err, "Failed to create new CassandraDatacenter")
			return result.Error(err), actualDcs
		}
		dcKey := types.NamespacedName{Namespace: desiredDc.Namespace, Name: desiredDc.Name}
		logger := logger.WithValues("CassandraDatacenter", dcKey, "K8SContext", dcTemplate.K8sContext)

		if idx > 0 {
			desiredDc.Annotations[cassdcapi.SkipUserCreationAnnotation] = "true"
		}

		rebuildNeeded := datacenterAddedToExistingCluster(kc, desiredDc.Name)
		if rebuildNeeded {
			desiredDc.Labels[api.RebuildLabel] = "true"
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

			// We need to reevaluate rebuildNeeded again. After the CassandraDatacenter is
			// created, rebuildNeeded will be false on a subsequent reconciliation when we hit this
			// point, so we have to check for the presence of the rebuild label on the actual
			// CassandraDatacenter.
			if _, rebuildNeeded = actualDc.Labels[api.RebuildLabel]; rebuildNeeded {
				desiredDc.Labels[api.RebuildLabel] = "true"
				// We need to recompute and reset the resource annotation here. On the
				// first reconciliation after a DC has been added rebuildNeeded will be
				// true and included in the resource hash. On subsequent reconciliations
				// where the CassandraDatacenter exists rebuildNeeded will initially be set
				// to false. If we get here, the label is present and thus the resource hash
				// needs to be updated.
				annotations.AddHashAnnotation(desiredDc)
			}

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

				desiredConfig, err := utils.UnmarshalToMap(desiredDc.Spec.Config)
				if err != nil {
					return result.Error(err), actualDcs
				}
				actualConfig, err := utils.UnmarshalToMap(actualDc.Spec.Config)
				if err != nil {
					return result.Error(err), actualDcs
				}

				actualCassYaml, foundActualYaml := actualConfig["cassandra-yaml"].(map[string]interface{})
				desiredCassYaml, foundDesiredYaml := desiredConfig["cassandra-yaml"].(map[string]interface{})

				if foundActualYaml && foundDesiredYaml {
					if actualCassYaml["num_tokens"] != desiredCassYaml["num_tokens"] {
						err = fmt.Errorf("tried to change num_tokens in an existing datacenter")
						return result.Error(err), actualDcs
					}
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

			if recResult := r.reconcileStargateAuthSchema(ctx, kc, desiredDc, remoteClient, logger); recResult.Completed() {
				return recResult, actualDcs
			}

			if recResult := r.reconcileReaperSchema(ctx, kc, desiredDc, remoteClient, logger); recResult.Completed() {
				return recResult, actualDcs
			}

			if rebuildNeeded {
				// TODO We need to handle the Stargate auth and Reaper keyspaces here.

				if recResult := r.updateUserKeyspacesReplication(ctx, kc, desiredDc, remoteClient, logger); recResult.Completed() {
					return recResult, actualDcs
				}

				if recResult := reconcileDcRebuild(ctx, actualDc, actualDcs, remoteClient, logger); recResult.Completed() {
					return recResult, actualDcs
				}
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
		kc.Status.SetCondition(api.K8ssandraClusterCondition{
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

func datacenterAddedToExistingCluster(kc *api.K8ssandraCluster, dcName string) bool {
	_, found := kc.Status.Datacenters[dcName]
	return kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionTrue && !found
}

func getSourceDatacenterName(targetDc *cassdcapi.CassandraDatacenter, dcs []*cassdcapi.CassandraDatacenter) string {
	for _, dc := range dcs {
		if dc.Name != targetDc.Name {
			return dc.Name
		}
	}
	// TODO This should never happen. Should we also return an error here?
	return ""
}

func reconcileDcRebuild(ctx context.Context, dc *cassdcapi.CassandraDatacenter, dcs []*cassdcapi.CassandraDatacenter, remoteClient client.Client, logger logr.Logger) result.ReconcileResult {
	logger.Info("Reconciling rebuild")

	srcDc := getSourceDatacenterName(dc, dcs)
	desiredTask := newRebuildTask(dc.Name, dc.Namespace, srcDc)
	taskKey := client.ObjectKey{Namespace: desiredTask.Namespace, Name: desiredTask.Name}
	task := &cassctlapi.CassandraTask{}

	if err := remoteClient.Get(ctx, taskKey, task); err == nil {
		if taskFinished(task) {
			// TODO what should we do if it failed?
			logger.Info("Datacenter build finished")
			patch := client.MergeFromWithOptions(dc.DeepCopy())
			delete(dc.Labels, api.RebuildLabel)
			if err = remoteClient.Patch(ctx, dc, patch); err != nil {
				logger.Error(err, "Failed to remove rebuild label")
				return result.Error(err)
			}
			return result.Continue()
		} else {
			//logger.Info("Waiting for datacenter rebuild to complete", "Active", task.Status.Active,
			//	"Succeeded", task.Status.Succeeded, "Failed", task.Status.Failed)
			logger.Info("Waiting for datacenter rebuild to complete", "Task", task)
			return result.RequeueSoon(15)
		}
	} else {
		if errors.IsNotFound(err) {
			logger.Info("Creating rebuild task", "Task", taskKey)
			if err = remoteClient.Create(ctx, desiredTask); err != nil {
				logger.Error(err, "Failed to create rebuild task", "Task", taskKey)
				return result.Error(err)
			}
			return result.RequeueSoon(15)
		}
		logger.Error(err, "Failed to get rebuild task", "Task", taskKey)
		return result.Error(err)
	}
}

func taskFinished(task *cassctlapi.CassandraTask) bool {
	return len(task.Spec.Jobs) == int(task.Status.Succeeded+task.Status.Failed)
}

func newRebuildTask(targetDc, namespace, srcDc string) *cassctlapi.CassandraTask {
	now := metav1.Now()
	task := &cassctlapi.CassandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      targetDc + "-rebuild",
		},
		Spec: cassctlapi.CassandraTaskSpec{
			ScheduledTime: &now,
			Datacenter: corev1.ObjectReference{
				Namespace: namespace,
				Name:      targetDc,
			},
			Jobs: []cassctlapi.CassandraJob{
				{
					Name:    targetDc + "-rebuild",
					Command: "rebuild",
					Arguments: map[string]string{
						"source_datacenter": srcDc,
					},
				},
			},
		},
	}

	annotations.AddHashAnnotation(task)

	return task
}
