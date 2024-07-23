package k8ssandra

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassctlapi "github.com/k8ssandra/cass-operator/apis/control/v1alpha1"
	ktaskapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	agent "github.com/k8ssandra/k8ssandra-operator/pkg/telemetry/cassandra_agent"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rebuildNodesLabel = "k8ssandra.io/rebuild-nodes"
)

func AllowUpdate(kc *api.K8ssandraCluster, logger logr.Logger) bool {
	logger.Info(fmt.Sprintf("Generation: %d, ObservedGeneration: %d", kc.Generation, kc.Status.ObservedGeneration))
	return kc.GenerationChanged() || metav1.HasAnnotation(kc.ObjectMeta, api.AutomatedUpdateAnnotation)
}

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

	dcConfigs, err := r.createDatacenterConfigs(ctx, kc, logger, systemReplication)
	if err != nil {
		return result.Error(err), nil
	}

	actualDcs := make([]*cassdcapi.CassandraDatacenter, 0, len(kc.Spec.Cassandra.Datacenters))

	cassClusterName := kc.CassClusterName()

	seeds, err := r.findSeeds(ctx, kc, cassClusterName, logger)
	if err != nil {
		logger.Error(err, "Failed to find seed nodes")
		return result.Error(err), actualDcs
	}

	// Reconcile CassandraDatacenter objects only
	for idx, dcConfig := range sortDatacentersByPriority(dcConfigs) {

		if !kc.Spec.UseExternalSecrets() && !secret.HasReplicatedSecrets(ctx, r.Client, kcKey, dcConfig.K8sContext) {
			// ReplicatedSecret has not replicated yet, wait until it has
			logger.Info("Waiting for replication to complete")
			return result.RequeueSoon(r.DefaultDelay), actualDcs
		}

		dcKey := types.NamespacedName{Namespace: utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kcKey.Namespace), Name: dcConfig.Meta.Name}
		dcLogger := logger.WithValues("CassandraDatacenter", dcKey, "K8SContext", dcConfig.K8sContext)

		remoteClient, err := r.ClientCache.GetRemoteClient(dcConfig.K8sContext)
		if err != nil {
			dcLogger.Error(err, "Failed to get remote client")
			return result.Error(err), actualDcs
		}

		// Create Medusa related objects
		if medusaResult := r.reconcileMedusa(ctx, kc, dcConfig, remoteClient, dcLogger); medusaResult.Completed() {
			return medusaResult, actualDcs
		}

		// Create per-node configuration ConfigMap
		if recResult := r.reconcilePerNodeConfiguration(ctx, kc, dcConfig, remoteClient, dcLogger); recResult.Completed() {
			return recResult, actualDcs
		}

		// Create Vector related objects
		if vectorResult := r.reconcileVector(ctx, kc, dcConfig, remoteClient, dcLogger); vectorResult.Completed() {
			return vectorResult, actualDcs
		}

		desiredDc, err := cassandra.NewDatacenter(kcKey, dcConfig)
		if err != nil {
			dcLogger.Error(err, "Failed to create new CassandraDatacenter")
			return result.Error(err), actualDcs
		}
		if idx > 0 {
			desiredDc.Annotations[cassdcapi.SkipUserCreationAnnotation] = "true"
		}

		mergedTelemetrySpec := kc.Spec.Cassandra.Datacenters[idx].Telemetry.MergeWith(kc.Spec.Cassandra.Telemetry)
		if mergedTelemetrySpec == nil {
			mergedTelemetrySpec = &telemetryapi.TelemetrySpec{}
		}
		agentCfg := agent.Configurator{
			TelemetrySpec: *mergedTelemetrySpec,
			RemoteClient:  remoteClient,
			Ctx:           ctx,
			Kluster:       kc,
			RequeueDelay:  r.DefaultDelay,
			DcNamespace:   desiredDc.Namespace,
			DcName:        desiredDc.DatacenterName(),
		}
		agentRes := agentCfg.ReconcileTelemetryAgentConfig(desiredDc)
		if agentRes.IsRequeue() {
			return result.RequeueSoon(r.DefaultDelay), actualDcs
		}

		if agentRes.IsError() {
			return agentRes, actualDcs
		}

		// Note: desiredDc should not be modified from now on
		annotations.AddHashAnnotation(desiredDc)

		if recResult := r.checkRebuildAnnotation(ctx, kc, dcKey.Name); recResult.Completed() {
			return recResult, actualDcs
		}

		actualDc := &cassdcapi.CassandraDatacenter{}

		if recResult := r.reconcileSeedsEndpoints(ctx, desiredDc, seeds, dcConfig.AdditionalSeeds, remoteClient, dcLogger); recResult.Completed() {
			return recResult, actualDcs
		}

		if err = remoteClient.Get(ctx, dcKey, actualDc); err == nil {
			// Fail the reconcile if cluster name has changed
			if actualDc.Spec.ClusterName != cassClusterName {
				return result.Error(fmt.Errorf("CassandraDatacenter %s has cluster name %s, but expected %s. Cluster name cannot be changed in an existing cluster", dcKey, actualDc.Spec.ClusterName, cassClusterName)), actualDcs
			}

			r.setStatusForDatacenter(kc, actualDc)

			if !annotations.CompareHashAnnotations(actualDc, desiredDc) && !AllowUpdate(kc, logger) {
				logger.Info("Datacenter requires an update, but we're not allowed to do it", "CassandraDatacenter", dcKey)
				// We're not allowed to update, but need to
				patch := client.MergeFrom(kc.DeepCopy())
				kc.Status.SetConditionStatus(api.ClusterRequiresUpdate, corev1.ConditionTrue)
				if err := r.Client.Status().Patch(ctx, kc, patch); err != nil {
					return result.Error(fmt.Errorf("failed to set %s condition: %v", api.ClusterRequiresUpdate, err)), actualDcs
				}
			} else if !annotations.CompareHashAnnotations(actualDc, desiredDc) {
				if actualDc.Spec.SuperuserSecretName != desiredDc.Spec.SuperuserSecretName {
					// If actualDc is created with SuperuserSecretName, it can't be changed anymore. We should reject all changes coming from K8ssandraCluster
					desiredDc.Spec.SuperuserSecretName = actualDc.Spec.SuperuserSecretName
					err = fmt.Errorf("tried to update superuserSecretName in K8ssandraCluster")
					dcLogger.Error(err, "SuperuserSecretName is immutable, reverting to existing value in CassandraDatacenter")
				}

				if annotations.HasAnnotationWithValue(kc, api.RebuildDcAnnotation, dcKey.Name) && desiredDc.Spec.Stopped {
					desiredDc.Spec.Stopped = false
					err = fmt.Errorf("tried to stop a datacenter that is being rebuilt")
					dcLogger.Error(err, "Stopped cannot be set to true until the CassandraDatacenter is fully rebuilt")
				}

				// Set the proper default during upgrade from 3.x to 4.x
				if kc.Spec.Cassandra.ServerType == api.ServerDistributionCassandra && semver.MustParse(actualDc.Spec.ServerVersion).Major() == 3 && semver.MustParse(desiredDc.Spec.ServerVersion).Major() > 3 {
					if desiredDc, err = cassandra.SetNewDefaultNumTokens(kc, desiredDc, actualDc); err != nil {
						return result.Error(fmt.Errorf("couldn't set the proper default during upgrade: %v", err)), actualDcs
					}
				}

				if err = cassandra.ValidateConfig(desiredDc, actualDc); err != nil {
					return result.Error(fmt.Errorf("invalid Cassandra config: %v", err)), actualDcs
				}

				actualDc = actualDc.DeepCopy()
				resourceVersion := actualDc.GetResourceVersion()
				desiredDc.DeepCopyInto(actualDc)
				actualDc.SetResourceVersion(resourceVersion)
				if err = remoteClient.Update(ctx, actualDc); err != nil {
					dcLogger.Error(err, "Failed to update datacenter")
					return result.Error(err), actualDcs
				}

				return result.RequeueSoon(r.DefaultDelay), actualDcs
			}

			if actualDc.Spec.Stopped {
				if !cassandra.DatacenterStopped(actualDc) {
					dcLogger.Info("Waiting for datacenter to satisfy Stopped condition")
					return result.Done(), actualDcs
				}
			} else {
				if !cassandra.DatacenterReady(actualDc) {
					dcLogger.Info("Waiting for datacenter to satisfy Ready condition")
					return result.Done(), actualDcs
				}
			}

			// DC is in the process of being upgraded but hasn't completed yet. Let's wait for it to go through.
			if actualDc.GetGeneration() != actualDc.Status.ObservedGeneration {
				dcLogger.Info("CassandraDatacenter is being updated. Requeuing the reconcile.", "Generation", actualDc.GetGeneration(), "ObservedGeneration", actualDc.Status.ObservedGeneration)
				return result.Done(), actualDcs
			}

			dcLogger.Info("The datacenter is reconciled")

			actualDcs = append(actualDcs, actualDc)

			if !actualDc.Spec.Stopped {

				if recResult := r.checkSchemas(ctx, kc, actualDc, remoteClient, dcLogger); recResult.Completed() {
					return recResult, actualDcs
				}

				if annotations.HasAnnotationWithValue(kc, api.RebuildDcAnnotation, dcKey.Name) {
					if recResult := r.reconcileDcRebuild(ctx, kc, actualDc, remoteClient, dcLogger); recResult.Completed() {
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
					dcLogger.Error(err, "Failed to create datacenter")
					return result.Error(err), actualDcs
				}
				return result.RequeueSoon(r.DefaultDelay), actualDcs
			} else {
				dcLogger.Error(err, "Failed to get datacenter")
				return result.Error(err), actualDcs
			}
		}
	}

	if AllowUpdate(kc, logger) {
		dcsRequiringUpdate := make([]string, 0, len(actualDcs))
		for _, dc := range actualDcs {
			if dc.Status.GetConditionStatus(cassdcapi.DatacenterRequiresUpdate) == corev1.ConditionTrue {
				dcsRequiringUpdate = append(dcsRequiringUpdate, dc.ObjectMeta.Name)
			}
		}

		if len(dcsRequiringUpdate) > 0 {
			generatedName := fmt.Sprintf("refresh-%d-%d", kc.Generation, kc.Status.ObservedGeneration)
			internalTask := &ktaskapi.K8ssandraTask{}
			err := r.Get(ctx, types.NamespacedName{Namespace: kc.Namespace, Name: generatedName}, internalTask)
			// If task wasn't found, create it and if task is still running, requeue
			if errors.IsNotFound(err) {
				// Delegate work to the task controller for Datacenter operations
				task := &ktaskapi.K8ssandraTask{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: kc.Namespace,
						Name:      generatedName,
						Labels:    labels.WatchedByK8ssandraClusterLabels(kcKey),
					},
					Spec: ktaskapi.K8ssandraTaskSpec{
						Cluster: corev1.ObjectReference{
							Name: kc.Name,
						},
						Datacenters: dcsRequiringUpdate,
						Template: cassctlapi.CassandraTaskTemplate{
							Jobs: []cassctlapi.CassandraJob{{
								Name:    fmt.Sprintf("refresh-%s", kc.Name),
								Command: "refresh",
							}},
						},
						DcConcurrencyPolicy: batchv1.ForbidConcurrent,
					},
				}

				if err := r.Create(ctx, task); err != nil {
					return result.Error(err), actualDcs
				}

				return result.RequeueSoon(r.DefaultDelay), actualDcs

			} else if internalTask.Status.CompletionTime.IsZero() {
				return result.RequeueSoon(r.DefaultDelay), actualDcs
			}
		}
	}

	// If we reach this point all CassandraDatacenters are ready. We only set the
	// CassandraInitialized condition if it is unset, i.e., only once. This allows us to
	// distinguish whether we are deploying a CassandraDatacenter as part of a new cluster
	// or as part of an existing cluster.
	if kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionUnknown {
		kc.Status.SetConditionStatus(api.CassandraInitialized, corev1.ConditionTrue)
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

func datacenterAddedToExistingCluster(kc *api.K8ssandraCluster, dcName string) bool {
	_, found := kc.Status.Datacenters[dcName]
	// Only request rebuild for the datacenter if it's not already in the cluster and if we have at least one datacenter already initialized.
	return !found && len(kc.Status.Datacenters) > 0
}

func getSourceDatacenterName(targetDc *cassdcapi.CassandraDatacenter, kc *api.K8ssandraCluster) (string, error) {
	dcNames := make([]string, 0)

	for _, dc := range kc.Spec.Cassandra.Datacenters {
		if dcStatus, found := kc.Status.Datacenters[dc.Meta.Name]; found {
			if dcStatus.Cassandra.GetConditionStatus(cassdcapi.DatacenterReady) == corev1.ConditionTrue {
				dcNames = append(dcNames, dc.CassDcName())
			}
		}
	}

	if rebuildFrom, found := kc.Annotations[api.RebuildSourceDcAnnotation]; found {
		if rebuildFrom == targetDc.DatacenterName() {
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
			return r.removeRebuildDcAnnotation(kc, ctx)
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
			Datacenter: corev1.ObjectReference{
				Namespace: namespace,
				Name:      targetDc,
			},
			CassandraTaskTemplate: cassctlapi.CassandraTaskTemplate{
				ScheduledTime: &now,
				Jobs: []cassctlapi.CassandraJob{
					{
						Name:    targetDc + "-rebuild",
						Command: "rebuild",
						Arguments: cassctlapi.JobArguments{
							SourceDatacenter: srcDc,
						},
					},
				},
			},
		},
	}

	annotations.AddHashAnnotation(task)

	return task
}

// sortDatacentersByPriority sorts the datacenters by upgrade priority.
// The datacenters are sorted in descending order of priority.
// The datacenters with the highest priority are first in the list.
// The datacenters with the lowest priority are last in the list.
func sortDatacentersByPriority(datacenters []*cassandra.DatacenterConfig) []*cassandra.DatacenterConfig {
	sortedDatacenters := make([]*cassandra.DatacenterConfig, len(datacenters))
	copy(sortedDatacenters, datacenters)
	sort.Slice(sortedDatacenters, func(i, j int) bool {
		return dcUpgradePriority(sortedDatacenters[i]) > dcUpgradePriority(sortedDatacenters[j])
	})
	return sortedDatacenters
}

// dcUpgradePriority returns the upgrade priority for the given datacenter.
func dcUpgradePriority(dc *cassandra.DatacenterConfig) int {
	if dc.DseWorkloads == nil {
		// Cassandra workload
		return 2
	}

	if dc.DseWorkloads.AnalyticsEnabled {
		return 3
	}

	if dc.DseWorkloads.SearchEnabled {
		return 1
	}

	return 2
}

func (r *K8ssandraClusterReconciler) removeRebuildDcAnnotation(kc *api.K8ssandraCluster, ctx context.Context) result.ReconcileResult {
	patch := client.MergeFrom(kc.DeepCopy())
	delete(kc.Annotations, api.RebuildDcAnnotation)
	if err := r.Client.Patch(ctx, kc, patch); err != nil {
		err = fmt.Errorf("failed to remove %s annotation: %v", api.RebuildDcAnnotation, err)
		return result.Error(err)
	}
	return result.Continue()
}
