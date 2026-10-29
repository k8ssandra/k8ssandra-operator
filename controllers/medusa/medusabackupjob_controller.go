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
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassimages "github.com/k8ssandra/cass-operator/pkg/images"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
)

// MedusaBackupJobReconciler reconciles a MedusaBackupJob object
type MedusaBackupJobReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme *runtime.Scheme
	medusa.ClientFactory
	ImageRegistry cassimages.ImageRegistry
}

// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackupjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackupjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackupjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",namespace="k8ssandra",resources=pods;services,verbs=get;list;watch
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackups/finalizers,verbs=update

func (r *MedusaBackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("medusabackupjob", req.NamespacedName)

	logger.Info("Starting reconciliation")

	instance := &medusav1alpha1.MedusaBackupJob{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		logger.Error(err, "Failed to get MedusaBackupJob")
		if apiErrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	backupJob := instance.DeepCopy()

	cassdcKey := types.NamespacedName{Namespace: backupJob.Namespace, Name: backupJob.Spec.CassandraDatacenter}
	cassdc := &cassdcapi.CassandraDatacenter{}
	err = r.Get(ctx, cassdcKey, cassdc)
	if err != nil {
		logger.Error(err, "failed to get cassandradatacenter", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{}, err
	}

	// Set an owner reference on the backup job so that it can be cleaned up when the cassandra datacenter is deleted
	if backupJob.OwnerReferences == nil {
		if err = controllerutil.SetOwnerReference(cassdc, backupJob, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{}, err
		}
		if err = r.Update(ctx, backupJob); err != nil {
			logger.Error(err, "failed to update MedusaBackupJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{}, err
		} else {
			logger.Info("updated MedusaBackupJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	}

	pods, err := medusa.GetCassandraDatacenterPods(ctx, cassdc, r, logger)
	if err != nil {
		logger.Error(err, "Failed to get datacenter pods")
		return ctrl.Result{}, err
	}
	if len(pods) == 0 {
		logger.Info("No pods found for datacenter", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}

	// If there is anything in progress, simply requeue the request until each pod has finished or errored
	if len(backupJob.Status.InProgress) > 0 {
		logger.Info("There are backups in progress, checking them..")
		progress := make([]string, 0, len(backupJob.Status.InProgress))
		patch := client.MergeFrom(backupJob.DeepCopy())

	StatusCheck:
		for _, podName := range backupJob.Status.InProgress {
			for _, pod := range pods {
				if podName == pod.Name {
					status, err := backupStatus(ctx, backupJob.Name, &pod, cassdc, r.Client, r.ClientFactory, logger)
					if err != nil {
						return ctrl.Result{}, err
					}

					switch status {
					case medusa.StatusType_IN_PROGRESS:
						progress = append(progress, podName)
					case medusa.StatusType_SUCCESS:
						backupJob.Status.Finished = append(backupJob.Status.Finished, podName)
					case medusa.StatusType_FAILED, medusa.StatusType_UNKNOWN:
						backupJob.Status.Failed = append(backupJob.Status.Failed, podName)
					}

					continue StatusCheck
				}
			}
		}

		if len(backupJob.Status.InProgress) != len(progress) {
			backupJob.Status.InProgress = progress
			if err := r.Status().Patch(ctx, backupJob, patch); err != nil {
				logger.Error(err, "failed to patch status")
				return ctrl.Result{}, err
			}
		}

		if len(progress) > 0 {
			logger.Info("MedusaBackupJob is still being processed", "Backup", req.NamespacedName)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}

	// If the backup is already finished, there is nothing to do.
	if medusaBackupFinished(backupJob) {
		logger.Info("Backup operation is already finished")
		return ctrl.Result{}, nil
	}

	// First check to see if the backup is already in progress
	if !backupJob.Status.StartTime.IsZero() {
		// If there is anything in progress, simply requeue the request
		if len(backupJob.Status.InProgress) > 0 {
			logger.Info("Backup is still in progress")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		// there is nothing in progress, so the job is finished (not yet sure if successfully)

		// if there are failures, we will end here and not proceed with creating a backup object
		if len(backupJob.Status.Failed) > 0 {
			logger.Info("Backup failed on some nodes", "BackupName", backupJob.Name, "Failed", backupJob.Status.Failed)
			return ctrl.Result{}, r.setFinishTime(ctx, backupJob, logger)
		}

		logger.Info("backup complete")

		// The MedusaBackupJob is finished successfully and we now need to create the MedusaBackup object.
		backupSummary, err := r.getBackupSummary(ctx, backupJob, pods, cassdc, logger)
		if err != nil {
			logger.Error(err, "Failed to get backup summary")
			if errors.Is(err, reconcile.TerminalError(nil)) {
				// retrying cannot help, so mark the job as finished, but still return the
				// error so that it gets reported like any other terminal error
				if finishErr := r.setFinishTime(ctx, backupJob, logger); finishErr != nil {
					return ctrl.Result{}, finishErr
				}
				return ctrl.Result{}, err
			}
			// the error is likely transient, leave the finish time unset so the retry can still
			// create the MedusaBackup
			return ctrl.Result{}, err
		}
		if err := r.createMedusaBackup(ctx, backupJob, backupSummary, logger); err != nil {
			logger.Error(err, "Failed to create MedusaBackup")
			return ctrl.Result{}, err
		}

		// the finish time is what makes the job terminal, so it is only set now that the
		// MedusaBackup exists
		return ctrl.Result{}, r.setFinishTime(ctx, backupJob, logger)
	}

	logger.Info("Backups have not been started yet")

	// Make sure that Medusa is deployed
	if !shared.IsMedusaDeployed(pods) {
		// TODO generate event and/or update status to indicate error condition
		logger.Error(medusa.ErrBackupSidecarNotFound, "medusa is not deployed", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{}, medusa.ErrBackupSidecarNotFound
	}

	patch := client.MergeFromWithOptions(backupJob.DeepCopy(), client.MergeFromWithOptimisticLock{})

	backupJob.Status.StartTime = metav1.Now()

	if err := r.Status().Patch(ctx, backupJob, patch); err != nil {
		logger.Error(err, "Failed to patch status")
		// We received a stale object, requeue for next processing
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}

	logger.Info("Starting backups")
	patch = client.MergeFrom(backupJob.DeepCopy())

	for _, p := range pods {
		logger.Info("starting backup", "CassandraPod", p.Name)
		_, err := doMedusaBackup(ctx, backupJob.Name, backupJob.Spec.Type, &p, cassdc, r.Client, r.ClientFactory, logger)
		if err != nil {
			logger.Error(err, "backup failed", "CassandraPod", p.Name)
		}

		backupJob.Status.InProgress = append(backupJob.Status.InProgress, p.Name)
	}
	// logger.Info("finished backup operations")
	if err := r.Status().Patch(context.Background(), backupJob, patch); err != nil {
		logger.Error(err, "failed to patch status", "Backup", fmt.Sprintf("%s/%s", backupJob.Name, backupJob.Namespace))
	}

	return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
}

// setFinishTime marks the backup job as terminal. The time is not accurate, but that is ok, we
// are only using it as a completion marker.
func (r *MedusaBackupJobReconciler) setFinishTime(ctx context.Context, backupJob *medusav1alpha1.MedusaBackupJob, logger logr.Logger) error {
	patch := client.MergeFrom(backupJob.DeepCopy())
	backupJob.Status.FinishTime = metav1.Now()
	if err := r.Status().Patch(ctx, backupJob, patch); err != nil {
		logger.Error(err, "failed to patch status with finish time")
		return err
	}
	return nil
}

func (r *MedusaBackupJobReconciler) getBackupSummary(ctx context.Context, backup *medusav1alpha1.MedusaBackupJob, pods []corev1.Pod, cassdc *cassdcapi.CassandraDatacenter, logger logr.Logger) (*medusa.BackupSummary, error) {
	// every node sees the same backups, so any pod that answers can provide the summary
	var lastErr error
	absentEverywhere := true
	for _, pod := range pods {
		remoteBackup, err := getBackup(ctx, backup.Name, &pod, cassdc, r.Client, r.ClientFactory, logger)
		if err != nil {
			logger.Error(err, "failed to get backup", "CassandraPod", pod.Name)
			lastErr = err
			absentEverywhere = absentEverywhere && backupAbsent(err, backup.Name)
			continue
		}
		if remoteBackup == nil {
			logger.Info("backup summary is nil", "CassandraPod", pod.Name)
			continue
		}
		logger.Info("found backup", "CassandraPod", pod.Name, "Backup", remoteBackup.BackupName)
		return remoteBackup, nil
	}
	if lastErr != nil && !absentEverywhere {
		// at least one pod failed for some other reason, so the backup may well be there
		return nil, fmt.Errorf("backup summary couldn't be found, and some pods could not be queried: %w", lastErr)
	}
	return nil, reconcile.TerminalError(fmt.Errorf("backup summary couldn't be found"))
}

func getBackup(ctx context.Context, name string, pod *corev1.Pod, cassdc *cassdcapi.CassandraDatacenter, c client.Client, clientFactory medusa.ClientFactory, logger logr.Logger) (*medusa.BackupSummary, error) {
	medusaClient, err := newClient(ctx, c, cassdc, pod, clientFactory)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := medusaClient.Close(); err != nil {
			logger.Error(err, ErrFailedCloseMedusaClient)
		}
	}()
	return medusaClient.GetBackup(ctx, name)
}

// Medusa does not answer NOT_FOUND for a backup it does not have, it fails the call with a plain
// INTERNAL error, so the message is the only thing we can go by. Match the whole sentence, since
// other failures, a missing bucket for instance, also talk about something not existing.
func backupAbsent(err error, name string) bool {
	return strings.Contains(err.Error(), fmt.Sprintf("The backup %s does not exist", name))
}

func (r *MedusaBackupJobReconciler) createMedusaBackup(ctx context.Context, backup *medusav1alpha1.MedusaBackupJob, backupSummary *medusa.BackupSummary, logger logr.Logger) error {
	// Create a MedusaBackup object after a successful MedusaBackupJob execution.
	logger.Info("Creating MedusaBackup object", "MedusaBackup", backup.Name)
	backupKey := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}
	backupResource := &medusav1alpha1.MedusaBackup{}
	err := r.Get(ctx, backupKey, backupResource)
	if err == nil {
		// the backup already exists, nothing to do
		return nil
	}
	if !apiErrors.IsNotFound(err) {
		// we do not know whether the MedusaBackup exists, so we must not report success
		logger.Error(err, "failed to get MedusaBackup", "MedusaBackup", backup.Name)
		return err
	}

	// Backup doesn't exist, create it
	startTime := backup.Status.StartTime
	finishTime := metav1.Now()
	logger.Info("Creating Cassandra Backup", "Backup", backup.Name)
	backupResource = &medusav1alpha1.MedusaBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		},
		Spec: medusav1alpha1.MedusaBackupSpec{
			CassandraDatacenter: backup.Spec.CassandraDatacenter,
			Type:                backup.Spec.Type,
		},
	}
	if err := r.Create(ctx, backupResource); err != nil {
		logger.Error(err, "failed to create backup", "MedusaBackup", backup.Name)
		return err
	}

	logger.Info("Created Medusa Backup", "Backup", backupResource)
	backupPatch := client.MergeFrom(backupResource.DeepCopy())
	backupResource.Status.StartTime = startTime
	backupResource.Status.FinishTime = finishTime
	backupResource.Status.TotalNodes = backupSummary.TotalNodes
	backupResource.Status.FinishedNodes = backupSummary.FinishedNodes
	backupResource.Status.TotalFiles = backupSummary.TotalObjects
	backupResource.Status.TotalSize = humanize(backupSummary.TotalSize)
	backupResource.Status.Nodes = make([]*medusav1alpha1.MedusaBackupNode, len(backupSummary.Nodes))
	for i, node := range backupSummary.Nodes {
		backupResource.Status.Nodes[i] = &medusav1alpha1.MedusaBackupNode{
			Host:       node.Host,
			Tokens:     node.Tokens,
			Datacenter: node.Datacenter,
			Rack:       node.Rack,
		}
	}
	backupResource.Status.Status = backupSummary.Status.String()
	if err := r.Status().Patch(ctx, backupResource, backupPatch); err != nil {
		logger.Error(err, "failed to patch status with finish time")
		return err
	}
	return nil
}

func doMedusaBackup(ctx context.Context, name string, backupType shared.BackupType, pod *corev1.Pod, cassdc *cassdcapi.CassandraDatacenter, c client.Client, clientFactory medusa.ClientFactory, logger logr.Logger) (string, error) {
	logger.Info("connecting to backup sidecar", "Pod", pod.Name)
	if medusaClient, err := newClient(ctx, c, cassdc, pod, clientFactory); err != nil {
		return "", err
	} else {
		logger.Info("successfully connected to backup sidecar", "Pod", pod.Name)
		defer func() {
			if err := medusaClient.Close(); err != nil {
				logger.Error(err, ErrFailedCloseMedusaClient)
			}
		}()
		resp, err := medusaClient.CreateBackup(ctx, name, string(backupType))
		if err != nil {
			return "", err
		}

		return resp.BackupName, nil
	}
}

func backupStatus(ctx context.Context, name string, pod *corev1.Pod, cassdc *cassdcapi.CassandraDatacenter, c client.Client, clientFactory medusa.ClientFactory, logger logr.Logger) (medusa.StatusType, error) {
	logger.Info("connecting to backup sidecar", "Pod", pod.Name)
	if medusaClient, err := newClient(ctx, c, cassdc, pod, clientFactory); err != nil {
		logger.Error(err, "Could not make a new medusa client")
		return medusa.StatusType_UNKNOWN, err
	} else {
		resp, err := medusaClient.BackupStatus(ctx, name)
		if err != nil {
			// the gRPC client does not return proper NotFound error, we need to check the payload too
			if apiErrors.IsNotFound(err) || strings.Contains(err.Error(), "NotFound") {
				logger.Info(fmt.Sprintf("did not find backup %s for pod %s", name, pod.Name))
				return medusa.StatusType_UNKNOWN, nil
			}
			logger.Error(err, fmt.Sprintf("getting backup status for backup %s and pod %s failed", name, pod.Name))
			return medusa.StatusType_UNKNOWN, err
		}

		return resp.Status, nil
	}
}

func medusaBackupFinished(backup *medusav1alpha1.MedusaBackupJob) bool {
	return !backup.Status.FinishTime.IsZero()
}

// SetupWithManager sets up the controller with the Manager.
func (r *MedusaBackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&medusav1alpha1.MedusaBackupJob{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func humanize(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	size := float64(bytes)
	i := 0
	for ; size >= 1024 && i < len(units)-1; i++ {
		size /= 1024
	}
	return fmt.Sprintf("%.2f %s", size, units[i])
}
