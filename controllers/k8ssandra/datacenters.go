package k8ssandra

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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

const (
	rebuildNodesLabel = "k8ssandra.io/rebuild-nodes"
)

func (r *K8ssandraClusterReconciler) reconcileDatacenters(ctx context.Context, kc *api.K8ssandraCluster, logger logr.Logger) (result.ReconcileResult, []*cassdcapi.CassandraDatacenter) {
	kcKey := utils.GetKey(kc)

	if recResult := r.checkDcDeletion(ctx, kc, logger); recResult.Completed() {
		return recResult, nil
	}

	systemReplication, err := r.checkInitialSystemReplication(ctx, kc, logger)
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
		if kc.Spec.Reaper != nil {
			reaper.AddReaperSettingsToDcConfig(kc.Spec.Reaper.DeepCopy(), dcConfig, kc.Spec.IsAuthEnabled())
		}
		// Create Medusa related objects
		if medusaResult := r.ReconcileMedusa(ctx, dcConfig, dcTemplate, kc, logger); medusaResult.Completed() {
			return medusaResult, actualDcs
		}

		remoteClient, err := r.ClientCache.GetRemoteClient(dcTemplate.K8sContext)
		if err != nil {
			logger.Error(err, "Failed to get remote client")
			return result.Error(err), actualDcs
		}

		err = cassandra.ReadEncryptionStoresSecrets(ctx, kcKey, dcConfig, remoteClient, logger)
		if err != nil {
			logger.Error(err, "Failed to read encryption secrets")
			return result.Error(err), actualDcs
		}
		desiredDc, err := cassandra.NewDatacenter(kcKey, dcConfig)
		if err != nil {
			logger.Error(err, "Failed to create new CassandraDatacenter")
			return result.Error(err), actualDcs
		}
		if idx > 0 {
			desiredDc.Annotations[cassdcapi.SkipUserCreationAnnotation] = "true"
		}

		// Note: desiredDc should not be modified from now on
		annotations.AddHashAnnotation(desiredDc)

		dcKey := types.NamespacedName{Namespace: desiredDc.Namespace, Name: desiredDc.Name}
		logger := logger.WithValues("CassandraDatacenter", dcKey, "K8SContext", dcTemplate.K8sContext)

		if recResult := r.checkRebuildAnnotation(ctx, kc, dcKey.Name); recResult.Completed() {
			return recResult, actualDcs
		}

		actualDc := &cassdcapi.CassandraDatacenter{}

		if recResult := r.reconcileSeedsEndpoints(ctx, desiredDc, seeds, dcConfig.AdditionalSeeds, remoteClient, logger); recResult.Completed() {
			return recResult, actualDcs
		}

		if err = remoteClient.Get(ctx, dcKey, actualDc); err == nil {
			r.setStatusForDatacenter(kc, actualDc)

			if !annotations.CompareHashAnnotations(actualDc, desiredDc) {
				logger.Info("Updating datacenter")

				if actualDc.Spec.SuperuserSecretName != desiredDc.Spec.SuperuserSecretName {
					// If actualDc is created with SuperuserSecretName, it can't be changed anymore. We should reject all changes coming from K8ssandraCluster
					desiredDc.Spec.SuperuserSecretName = actualDc.Spec.SuperuserSecretName
					err = fmt.Errorf("tried to update superuserSecretName in K8ssandraCluster")
					logger.Error(err, "SuperuserSecretName is immutable, reverting to existing value in CassandraDatacenter")
				}

				if annotations.HasAnnotationWithValue(kc, api.RebuildDcAnnotation, dcKey.Name) && desiredDc.Spec.Stopped {
					desiredDc.Spec.Stopped = false
					err = fmt.Errorf("tried to stop a datacenter that is being rebuilt")
					logger.Error(err, "Stopped cannot be set to true until the CassandraDatacenter is fully rebuilt")
				}

				if err := cassandra.ValidateConfig(desiredDc, actualDc); err != nil {
					return result.Error(fmt.Errorf("invalid Cassandra config: %v", err)), actualDcs
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

			if actualDc.Spec.Stopped {
				if !cassandra.DatacenterStopped(actualDc) {
					logger.Info("Waiting for datacenter to satisfy Stopped condition")
					return result.Done(), actualDcs
				}
			} else {
				if !cassandra.DatacenterReady(actualDc) {
					logger.Info("Waiting for datacenter to satisfy Ready condition")
					return result.Done(), actualDcs
				}
			}

			logger.Info("The datacenter is reconciled")

			actualDcs = append(actualDcs, actualDc)

			if !actualDc.Spec.Stopped {

				if recResult := r.checkSchemas(ctx, kc, actualDc, remoteClient, logger); recResult.Completed() {
					return recResult, actualDcs
				}

				if annotations.HasAnnotationWithValue(kc, api.RebuildDcAnnotation, dcKey.Name) {
					if recResult := r.reconcileDcRebuild(ctx, kc, actualDc, remoteClient, logger); recResult.Completed() {
						return recResult, actualDcs
					}
				}

			}

		} else {
			if errors.IsNotFound(err) {
				if annotations.HasAnnotationWithValue(kc, api.RebuildDcAnnotation, dcKey.Name) && desiredDc.Spec.Stopped {
					err := fmt.Errorf("cannot add a datacenter in stopped state to an existing cluster")
					return result.Error(err), actualDcs
				}
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

func (r *K8ssandraClusterReconciler) setStatusForDatacenter(kc *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter) {
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
}

func (r *K8ssandraClusterReconciler) removeDatacenterStatus(kc *api.K8ssandraCluster, dcName string) {
	if kdcStatus, found := kc.Status.Datacenters[dcName]; found {
		if kdcStatus.Stargate == nil {
			delete(kc.Status.Datacenters, dcName)
		} else {
			kc.Status.Datacenters[dcName] = api.K8ssandraStatus{
				Stargate:  kdcStatus.Stargate.DeepCopy(),
				Cassandra: nil,
				Reaper:    kdcStatus.Reaper.DeepCopy(),
			}
		}
	}
}

func datacenterAddedToExistingCluster(kc *api.K8ssandraCluster, dcName string) bool {
	_, found := kc.Status.Datacenters[dcName]
	return kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionTrue && !found
}

func getSourceDatacenterName(targetDc *cassdcapi.CassandraDatacenter, kc *api.K8ssandraCluster) (string, error) {
	dcNames := make([]string, 0)

	for _, dc := range kc.Spec.Cassandra.Datacenters {
		if dcStatus, found := kc.Status.Datacenters[dc.Meta.Name]; found {
			if dcStatus.Cassandra.GetConditionStatus(cassdcapi.DatacenterReady) == corev1.ConditionTrue {
				dcNames = append(dcNames, dc.Meta.Name)
			}
		}
	}

	if rebuildFrom, found := kc.Annotations[api.RebuildSourceDcAnnotation]; found {
		if rebuildFrom == targetDc.Name {
			return "", fmt.Errorf("rebuild error: src dc and target dc cannot be the same")
		}

		for _, dc := range dcNames {
			if rebuildFrom == dc {
				return dc, nil
			}
		}

		return "", fmt.Errorf("rebuild error: src dc must a ready dc")
	}

	for _, dc := range dcNames {
		if dc != targetDc.Name {
			return dc, nil
		}
	}

	return "", fmt.Errorf("rebuild error: unable to determine src dc for target dc (%s) from dc list (%s)",
		targetDc.Name, strings.Trim(fmt.Sprint(dcNames), "[]"))
}

func (r *K8ssandraClusterReconciler) checkRebuildAnnotation(ctx context.Context, kc *api.K8ssandraCluster, dcName string) result.ReconcileResult {
	if !annotations.HasAnnotationWithValue(kc, api.RebuildDcAnnotation, dcName) && datacenterAddedToExistingCluster(kc, dcName) {
		patch := client.MergeFromWithOptions(kc.DeepCopy())
		annotations.AddAnnotation(kc, api.RebuildDcAnnotation, dcName)
		if err := r.Client.Patch(ctx, kc, patch); err != nil {
			return result.Error(fmt.Errorf("failed to add rebuild annotation: %v", err))
		}
	}

	return result.Continue()
}

func (r *K8ssandraClusterReconciler) reconcileDcRebuild(
	ctx context.Context,
	kc *api.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	remoteClient client.Client,
	logger logr.Logger) result.ReconcileResult {

	logger.Info("Reconciling rebuild")

	srcDc, err := getSourceDatacenterName(dc, kc)
	if err != nil {
		return result.Error(err)
	}

	desiredTask := newRebuildTask(dc.Name, dc.Namespace, srcDc, int(dc.Spec.Size))
	taskKey := client.ObjectKey{Namespace: desiredTask.Namespace, Name: desiredTask.Name}
	task := &cassctlapi.CassandraTask{}

	if err := remoteClient.Get(ctx, taskKey, task); err == nil {
		finished, err := taskFinished(task)
		if err != nil {
			return result.Error(err)
		}
		if finished {
			// TODO what should we do if it failed?
			logger.Info("Datacenter rebuild finished")
			patch := client.MergeFromWithOptions(kc.DeepCopy())
			delete(kc.Annotations, api.RebuildDcAnnotation)
			if err = r.Client.Patch(ctx, kc, patch); err != nil {
				err = fmt.Errorf("failed to remove %s annotation: %v", api.RebuildDcAnnotation, err)
				return result.Error(err)
			}
			return result.Continue()
		} else {
			logger.Info("Waiting for datacenter rebuild to complete", "Task", taskKey)
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

func taskFinished(task *cassctlapi.CassandraTask) (bool, error) {
	label, found := task.Labels[rebuildNodesLabel]
	if !found {
		return false, fmt.Errorf("rebuild task %s is missing required label %s", utils.GetKey(task), rebuildNodesLabel)
	}

	numNodes, err := strconv.Atoi(label)
	if err != nil {
		return false, fmt.Errorf("failed to parse label %s value for rebuild task %s", rebuildNodesLabel, utils.GetKey(task))
	}
	return numNodes == task.Status.Succeeded+task.Status.Failed, nil
}

func newRebuildTask(targetDc, namespace, srcDc string, numNodes int) *cassctlapi.CassandraTask {
	now := metav1.Now()
	task := &cassctlapi.CassandraTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      targetDc + "-rebuild",
			Labels: map[string]string{
				rebuildNodesLabel: strconv.Itoa(numNodes),
			},
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
