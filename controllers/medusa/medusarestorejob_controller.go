/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package medusa

import (
	"context"
	"encoding/json"
	goerrors "errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"

	"github.com/go-logr/logr"
	cassimages "github.com/k8ssandra/cass-operator/pkg/images"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/google/uuid"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

const (
	restoreContainerName = "medusa-restore"
	backupNameEnvVar     = "BACKUP_NAME"
	restoreKeyEnvVar     = "RESTORE_KEY"
	restoreMappingEnvVar = "RESTORE_MAPPING"
)

// MedusaRestoreJobReconciler reconciles a MedusaRestoreJob object
type MedusaRestoreJobReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme *runtime.Scheme
	medusa.ClientFactory
	ImageRegistry cassimages.ImageRegistry
}

// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusarestorejobs,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusarestorejobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackups,verbs=get;list;watch
// +kubebuilder:rbac:groups=cassandra.datastax.com,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,namespace="k8ssandra",resources=statefulsets,verbs=list;watch

func (r *MedusaRestoreJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("medusarestorejob", req.NamespacedName)
	factory := medusa.NewFactory(r.Client, logger)
	logger.Info("Starting reconcile", "MedusaRestoreJob", req.Name)

	// A finished job must never touch the datacenter again. Checked before the request is
	// built, which fetches the MedusaBackup, so a job whose backup was purged also returns here.
	restoreJob := &medusav1alpha1.MedusaRestoreJob{}
	if err := r.Get(ctx, req.NamespacedName, restoreJob); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get MedusaRestoreJob")
		return ctrl.Result{}, err
	}
	if !restoreJob.Status.FinishTime.IsZero() {
		logger.V(1).Info("MedusaRestoreJob has already finished, skipping reconcile", "FinishTime", restoreJob.Status.FinishTime)
		return ctrl.Result{}, nil
	}

	request, result, err := factory.NewMedusaRestoreRequest(ctx, req.NamespacedName)

	if result != nil {
		return *result, err
	}

	// The validating webhook rejects a bad spec at admission. This covers clusters where it is not running.
	if err := medusav1alpha1.ValidateRestoreJobSpec(&request.RestoreJob.Spec); err != nil {
		return r.failTerminal(ctx, request, err)
	}

	if request.RestoreJob.Status.StartTime.IsZero() {
		request.SetMedusaRestoreStartTime(metav1.Now())
		request.SetMedusaRestoreKey(uuid.New().String())
		request.SetMedusaRestorePrepared(false)
	}

	cassdcKey := types.NamespacedName{Namespace: request.RestoreJob.Namespace, Name: request.Datacenter.Name}
	cassdc := &cassdcapi.CassandraDatacenter{}
	err = r.Get(ctx, cassdcKey, cassdc)
	if err != nil {
		logger.Error(err, "failed to get cassandradatacenter", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{}, err
	}

	// Set an owner reference on the restore job so that it can be cleaned up when the cassandra datacenter is deleted
	if request.RestoreJob.OwnerReferences == nil {
		if err = controllerutil.SetOwnerReference(cassdc, request.RestoreJob, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{}, err
		}
		if err = r.Update(ctx, request.RestoreJob); err != nil {
			logger.Error(err, "failed to update MedusaRestoreJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{}, err
		} else {
			logger.Info("updated MedusaRestoreJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	}

	// Verify the backup can be used for restore. A PITR job has no MedusaBackup: Medusa checks the
	// base backups of every node in PreparePitrRestore.
	if request.RestoreJob.Spec.TargetTimestamp == "" {
		if err := validateBackupForRestore(request.MedusaBackup, cassdc); err != nil {
			request.RestoreJob.Status.FinishTime = metav1.Now()
			request.RestoreJob.Status.Message = err.Error()
			if err = r.Status().Update(ctx, request.RestoreJob); err != nil {
				logger.Error(err, "failed to update MedusaRestoreJob with error message", "MedusaRestoreJob", req.Name)
				return ctrl.Result{}, err
			}

			logger.Error(fmt.Errorf("unable to use target backup for restore of CassandraDatacenter: %s", request.RestoreJob.Status.Message), "backup can not be used for restore")
			return ctrl.Result{}, nil // No requeue, because this error is not transient
		}
	}

	// Prepare the restore by placing a mapping file in the Cassandra data volume.
	if !request.RestoreJob.Status.RestorePrepared {
		restoreMapping, err := r.prepareRestore(ctx, request, logger)
		if err != nil {
			var terminal *medusa.TerminalError
			if goerrors.As(err, &terminal) {
				return r.failTerminal(ctx, request, err)
			}
			logger.Error(err, "Failed to prepare restore")
			return ctrl.Result{}, err
		}
		request.SetMedusaRestorePrepared(true)
		request.SetMedusaRestoreMapping(*restoreMapping)
		return r.applyUpdatesAndRequeue(ctx, request)
	}

	if request.RestoreJob.Status.DatacenterStopped.IsZero() {
		if stopped := stopDatacenterRestoreJob(request); !stopped {
			return r.applyUpdatesAndRequeue(ctx, request)
		}
	}

	if err := updateMedusaRestoreInitContainer(request); err != nil {
		request.Log.Error(err, "The datacenter is not properly configured for backup/restore")
		// No need to requeue here because the datacenter is not properly configured for
		// backup/restore with Medusa.
		return ctrl.Result{}, err
	}

	complete, err := r.podTemplateSpecUpdateComplete(ctx, request)

	if err != nil {
		request.Log.Error(err, "Failed to check if datacenter update is complete")
		// Not going to bother applying updates here since we hit an error.
		return ctrl.Result{}, err
	}

	if !complete {
		request.Log.Info("Waiting for datacenter update to complete")
		return r.applyUpdatesAndRequeue(ctx, request)
	}

	request.Log.Info("The datacenter has been updated")

	if request.Datacenter.Spec.Stopped {
		request.Log.Info("Starting the datacenter")
		request.Datacenter.Spec.Stopped = false

		return r.applyUpdatesAndRequeue(ctx, request)
	}

	if !cassandra.DatacenterReady(request.Datacenter) {
		request.Log.Info("Waiting for datacenter to come back online")
		return r.applyUpdatesAndRequeue(ctx, request)
	}

	// Refresh the secrets before persisting FinishTime. Once FinishTime is set the job is never
	// reconciled again, so a failed refresh has to be retried while the job is still unfinished.
	// The refresh annotation is derived from StartTime, so retries write the same value.
	recRes := medusa.RefreshSecrets(request.Datacenter, ctx, r.Client, request.Log, r.DefaultDelay, request.RestoreJob.Status.StartTime)
	switch {
	case recRes.IsError():
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, recRes.GetError()
	case recRes.IsRequeue():
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}

	request.SetMedusaRestoreFinishTime(metav1.Now())
	if err := r.applyUpdates(ctx, request); err != nil {
		return ctrl.Result{}, err
	}

	request.Log.Info("The restore operation is complete for DC", "CassandraDatacenter", request.Datacenter.Name)

	return ctrl.Result{}, nil
}

// applyUpdates patches the CassandraDatacenter if its spec has been updated and patches
// the MedusaRestoreJob if its status has been updated.
func (r *MedusaRestoreJobReconciler) applyUpdates(ctx context.Context, req *medusa.RestoreRequest) error {
	if req.DatacenterModified() {
		if err := r.Patch(ctx, req.Datacenter, req.GetDatacenterPatch()); err != nil {
			if errors.IsResourceExpired(err) {
				req.Log.Info("CassandraDatacenter version expired!")
			}
			req.Log.Error(err, "Failed to patch the CassandraDatacenter")
			return err
		}
	}

	if req.MedusaRestoreModified() {
		if err := r.Status().Patch(ctx, req.RestoreJob, req.GetRestorePatch()); err != nil {
			req.Log.Error(err, "Failed to patch the MedusaRestoreJob")
			return err
		}
	}

	return nil
}

func (r *MedusaRestoreJobReconciler) applyUpdatesAndRequeue(ctx context.Context, req *medusa.RestoreRequest) (ctrl.Result, error) {
	if err := r.applyUpdates(ctx, req); err != nil {
		req.Log.Error(err, "Failed to apply updates")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
}

// failTerminal marks the MedusaRestoreJob as finished with err as the reason. The job is never reconciled
// again. Every caller runs before the datacenter is stopped, so a terminal failure never leaves it down.
func (r *MedusaRestoreJobReconciler) failTerminal(ctx context.Context, req *medusa.RestoreRequest, err error) (ctrl.Result, error) {
	req.Log.Error(err, "The restore cannot run, marking the MedusaRestoreJob as failed")
	req.SetMedusaRestoreFinishTime(metav1.Now())
	req.RestoreJob.Status.Message = err.Error()
	if err := r.applyUpdates(ctx, req); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// podTemplateSpecUpdateComplete checks that the pod template spec changes, namely the ones
// with the restore container, have been pushed down to the StatefulSets. Return true if
// the changes have been applied.
func (r *MedusaRestoreJobReconciler) podTemplateSpecUpdateComplete(ctx context.Context, req *medusa.RestoreRequest) (bool, error) {
	if updated := cassandra.DatacenterUpdatedAfter(req.RestoreJob.Status.DatacenterStopped.Add(-1*time.Second), req.Datacenter); !updated {
		return false, nil
	}

	// It may not be sufficient to check for the update only via status conditions. We will
	// check the template spec of the StatefulSets to be certain that the update has been
	// applied. We do this in order to avoid an extra rolling restart after the
	// StatefulSets are scaled back up.

	statefulsetList := &appsv1.StatefulSetList{}
	labels := client.MatchingLabels{cassdcapi.ClusterLabel: cassdcapi.CleanLabelValue(req.Datacenter.Spec.ClusterName), cassdcapi.DatacenterLabel: req.Datacenter.Name}

	if err := r.List(ctx, statefulsetList, labels, client.InNamespace(req.Datacenter.Namespace)); err != nil {
		req.Log.Error(err, "Failed to get StatefulSets")
		return false, err
	}

	for _, statefulset := range statefulsetList.Items {
		container := getRestoreInitContainerFromStatefulSet(&statefulset)

		if container == nil {
			return false, nil
		}

		// A PITR restore has no backup name. RESTORE_KEY below is unique to this job and is written in the
		// same update as BACKUP_NAME and RESTORE_MAPPING, so it proves that the mapping is this job's too.
		backupName := ""
		if req.RestoreJob.Spec.TargetTimestamp == "" {
			backupName = req.MedusaBackup.Name
		}
		if !utils.ContainerHasEnvVar(container, backupNameEnvVar, backupName) {
			return false, nil
		}

		if !utils.ContainerHasEnvVar(container, restoreKeyEnvVar, req.RestoreJob.Status.RestoreKey) {
			return false, nil
		}
	}

	return true, nil
}

// prepareRestore prepares the MedusaRestoreMapping for the restore operation.
// It uses the Medusa client to get the host map for the restore operation, using the first pod answering on the backup sidecar port.
func (r *MedusaRestoreJobReconciler) prepareRestore(ctx context.Context, request *medusa.RestoreRequest, logger logr.Logger) (*medusav1alpha1.MedusaRestoreMapping, error) {
	pods, err := medusa.GetCassandraDatacenterPods(ctx, request.Datacenter, r, request.Log)
	if err != nil {
		logger.Error(err, "Failed to get datacenter pods")
		return nil, err
	}

	for _, pod := range pods {
		if medusaClient, err := newClient(ctx, r.Client, request.Datacenter, ptr.To(pod), r.ClientFactory); err != nil {
			logger.Error(err, "Failed to create Medusa client", "Pod", pod.Name)
		} else {
			if request.RestoreJob.Spec.TargetTimestamp != "" {
				// PITR restore: Medusa picks the base backups of every node in one coordinator call
				return r.preparePitrRestore(ctx, request, medusaClient, logger)
			}

			restoreHostMap, err := medusa.GetHostMap(request.Datacenter, *request.RestoreJob, medusaClient, ctx)
			if err != nil {
				return nil, err
			}

			medusaRestoreMapping, err := restoreHostMap.ToMedusaRestoreMapping()
			if err != nil {
				return nil, err
			}

			return &medusaRestoreMapping, nil
		}
	}
	return nil, fmt.Errorf("failed to get host map from backup")
}

// preparePitrRestore asks Medusa to check that a point-in-time restore of the datacenter to the job's
// targetTimestamp is possible, and to pick a base backup for each node. On success it returns the restore
// mapping with the pitr block attached. It returns a medusa.TerminalError when Medusa does not have the RPC,
// when Medusa rejects the restore, or when the base backups do not fit the datacenter. Any other error is
// transient and is retried by controller-runtime; the RPC is read-only, so a retry is safe.
func (r *MedusaRestoreJobReconciler) preparePitrRestore(ctx context.Context, request *medusa.RestoreRequest, client medusa.Client, logger logr.Logger) (*medusav1alpha1.MedusaRestoreMapping, error) {
	// The timestamp goes to Medusa and into the mapping as written in the spec. Medusa converts it when it
	// renders the replay configuration.
	targetTimestamp := request.RestoreJob.Spec.TargetTimestamp

	// The Cassandra datacenter name, not the object name: it is the name Medusa stores in its backups.
	resp, err := client.PreparePitrRestore(ctx, request.Datacenter.DatacenterName(), targetTimestamp)
	if status.Code(err) == codes.Unimplemented {
		return nil, medusa.Terminal(fmt.Errorf("the Medusa image does not support PITR, upgrade the Medusa image"))
	}
	if err != nil {
		return nil, fmt.Errorf("PreparePitrRestore failed: %w", err)
	}
	// The response carries no reason. The Medusa log of the coordinator pod has it.
	if resp.Status == medusa.StatusType_FAILED {
		return nil, medusa.Terminal(fmt.Errorf("PITR restore to %s rejected by Medusa", targetTimestamp))
	}

	// The same rack-by-rack pairing as a backup restore. Pod names may differ (in_place false).
	hostMap, err := medusa.GetPitrHostMap(request.Datacenter, resp.NodeBaseBackups, client, ctx)
	if err != nil {
		return nil, err
	}
	mapping, err := hostMap.ToMedusaRestoreMapping()
	if err != nil {
		return nil, err
	}
	mapping.Pitr = &medusav1alpha1.PitrRestoreBlock{
		TargetTimestamp: targetTimestamp,      // spec.targetTimestamp, unchanged
		NodeBackups:     resp.NodeBaseBackups, // keyed by source node name, each pod resolves its own through host_map
	}
	logger.Info("Medusa accepted the PITR restore", "TargetTimestamp", targetTimestamp, "Nodes", len(resp.NodeBaseBackups))
	return &mapping, nil
}

func validateBackupForRestore(backup *medusav1alpha1.MedusaBackup, cassdc *cassdcapi.CassandraDatacenter) error {
	if backup.Status.TotalNodes == 0 && backup.Status.FinishedNodes == 0 {
		// This is an old backup without enough data, need to skip for backwards compatibility
		return nil
	}

	if backup.Status.FinishTime.IsZero() {
		return fmt.Errorf("target backup has not finished")
	}

	if backup.Status.FinishedNodes != backup.Status.TotalNodes {
		// In Medusa, a failed backup is not considered Finished. In MedusaBackupJob, a failed backup is considered finished, but failed.
		return fmt.Errorf("target backup has not completed successfully")
	}

	if backup.Status.TotalNodes != cassdc.Spec.Size {
		return fmt.Errorf("node counts differ for source backup and destination datacenter")
	}

	rackSizes := make(map[string]int)
	for _, n := range backup.Status.Nodes {
		if n.Datacenter != cassdc.DatacenterName() {
			return fmt.Errorf("target datacenter has different name than backup")
		}
		if c, found := rackSizes[n.Rack]; !found {
			rackSizes[n.Rack] = 1
		} else {
			rackSizes[n.Rack] = c + 1
		}
	}

	if len(cassdc.Spec.Racks) > 0 {
		if len(rackSizes) != len(cassdc.Spec.Racks) {
			return fmt.Errorf("amount of racks must match in backup and target datacenter")
		}

		for _, r := range cassdc.Spec.Racks {
			if _, found := rackSizes[r.Name]; !found {
				return fmt.Errorf("rack names must match in backup and target datacenter")
			}
		}
	} else {
		// cass-operator treats this as single rack setup, with name "default"
		if len(rackSizes) > 1 {
			return fmt.Errorf("amount of racks must match in backup and target datacenter")
		}

		if backup.Status.Nodes[0].Rack != "default" {
			return fmt.Errorf("rack names must match in backup and target datacenter")
		}
	}

	return nil
}

// stopDatacenter sets the Stopped property in the Datacenter spec to true. Returns true if
// the datacenter is stopped.
func stopDatacenterRestoreJob(req *medusa.RestoreRequest) bool {
	if cassandra.DatacenterStopped(req.Datacenter) {
		req.Log.Info("The datacenter is stopped", "Status", req.Datacenter.Status)
		req.SetDatacenterStoppedTimeRestoreJob(metav1.Now())
		return true
	}

	if cassandra.DatacenterStopping(req.Datacenter) {
		req.Log.Info("Waiting for datacenter to stop")
		return false
	}

	req.Log.Info("Stopping datacenter")
	req.Datacenter.Spec.Stopped = true
	return false
}

// updateRestoreInitContainer sets the backup name, restore key and restore mapping env vars in the medusa-restore
// init container. An error is returned if the container is not found.
func updateMedusaRestoreInitContainer(req *medusa.RestoreRequest) error {
	// A PITR restore has no single backup: the base backups are in the mapping. BACKUP_NAME is cleared so
	// that a value left by an earlier backup restore cannot open the backup gate of the restore entrypoint.
	backupName := ""
	if req.RestoreJob.Spec.TargetTimestamp == "" {
		backupName = req.MedusaBackup.Name
	}
	if err := setBackupNameInRestoreContainer(backupName, req.Datacenter); err != nil {
		return err
	}

	if err := setRestoreMappingInRestoreContainer(req.RestoreJob.Status.RestoreMapping, req.Datacenter); err != nil {
		return err
	}

	return setRestoreKeyInRestoreContainer(req.RestoreJob.Status.RestoreKey, req.Datacenter)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MedusaRestoreJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&medusav1alpha1.MedusaRestoreJob{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func getRestoreInitContainerFromStatefulSet(statefulset *appsv1.StatefulSet) *corev1.Container {
	for _, container := range statefulset.Spec.Template.Spec.InitContainers {
		if container.Name == restoreContainerName {
			return &container
		}
	}
	return nil
}

func setBackupNameInRestoreContainer(backupName string, cassdc *cassdcapi.CassandraDatacenter) error {
	index, err := getRestoreInitContainerIndex(cassdc)
	if err != nil {
		return err
	}

	restoreContainer := &cassdc.Spec.PodTemplateSpec.Spec.InitContainers[index]
	envVars := restoreContainer.Env
	envVarIdx := utils.GetEnvVarIndex(backupNameEnvVar, envVars)

	if envVarIdx > -1 {
		envVars[envVarIdx].Value = backupName
	} else {
		envVars = append(envVars, corev1.EnvVar{Name: backupNameEnvVar, Value: backupName})
	}
	restoreContainer.Env = envVars

	return nil
}

func setRestoreKeyInRestoreContainer(restoreKey string, dc *cassdcapi.CassandraDatacenter) error {
	index, err := getRestoreInitContainerIndex(dc)
	if err != nil {
		return err
	}

	restoreContainer := &dc.Spec.PodTemplateSpec.Spec.InitContainers[index]
	envVars := restoreContainer.Env
	envVarIdx := utils.GetEnvVarIndex(restoreKeyEnvVar, envVars)

	if envVarIdx > -1 {
		envVars[envVarIdx].Value = restoreKey
	} else {
		envVars = append(envVars, corev1.EnvVar{Name: restoreKeyEnvVar, Value: restoreKey})
	}
	restoreContainer.Env = envVars

	return nil
}

func setRestoreMappingInRestoreContainer(restoreMapping medusav1alpha1.MedusaRestoreMapping, dc *cassdcapi.CassandraDatacenter) error {
	// Marshall the restore mapping to a json string
	restoreMappingBytes, err := json.Marshal(restoreMapping)
	if err != nil {
		return err
	}

	index, err := getRestoreInitContainerIndex(dc)
	if err != nil {
		return err
	}

	restoreContainer := &dc.Spec.PodTemplateSpec.Spec.InitContainers[index]
	envVars := restoreContainer.Env
	envVarIdx := utils.GetEnvVarIndex(restoreMappingEnvVar, envVars)

	if envVarIdx > -1 {
		envVars[envVarIdx].Value = string(restoreMappingBytes)
	} else {
		envVars = append(envVars, corev1.EnvVar{Name: restoreMappingEnvVar, Value: string(restoreMappingBytes)})
	}
	restoreContainer.Env = envVars

	return nil
}

func getRestoreInitContainerIndex(dc *cassdcapi.CassandraDatacenter) (int, error) {
	spec := dc.Spec.PodTemplateSpec
	initContainers := &spec.Spec.InitContainers

	for i, container := range *initContainers {
		if container.Name == restoreContainerName {
			return i, nil
		}
	}

	return 0, fmt.Errorf("restore initContainer (%s) not found", restoreContainerName)
}
