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
	"fmt"
	"net"
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
		if err = controllerutil.SetControllerReference(cassdc, backupJob, r.Scheme); err != nil {
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
		return ctrl.Result{Requeue: true}, nil
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
					status, err := backupStatus(ctx, backupJob.ObjectMeta.Name, &pod, r.ClientFactory, logger)
					if err != nil {
						return ctrl.Result{}, err
					}

					if status == medusa.StatusType_IN_PROGRESS {
						progress = append(progress, podName)
					} else if status == medusa.StatusType_SUCCESS {
						backupJob.Status.Finished = append(backupJob.Status.Finished, podName)
					} else if status == medusa.StatusType_FAILED || status == medusa.StatusType_UNKNOWN {
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
		return ctrl.Result{Requeue: true}, nil
	}

	// If the backup is already finished, there is nothing to do.
	if medusaBackupFinished(backupJob) {
		logger.Info("Backup operation is already finished")
		return ctrl.Result{Requeue: false}, nil
	}

	// First check to see if the backup is already in progress
	if !backupJob.Status.StartTime.IsZero() {
		// If there is anything in progress, simply requeue the request
		if len(backupJob.Status.InProgress) > 0 {
			logger.Info("Backup is still in progress")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		// there is nothing in progress, so the job is finished (not yet sure if successfully)
		// Regardless of the success, we set the job finish time
		// Note that the time here is not accurate, but that is ok. For now we are just
		// using it as a completion marker.
		patch := client.MergeFrom(backupJob.DeepCopy())
		backupJob.Status.FinishTime = metav1.Now()
		if err := r.Status().Patch(ctx, backupJob, patch); err != nil {
			logger.Error(err, "failed to patch status with finish time")
			return ctrl.Result{}, err
		}

		// if there are failures, we will end here and not proceed with creating a backup object
		if len(backupJob.Status.Failed) > 0 {
			logger.Info("Backup failed on some nodes", "BackupName", backupJob.Name, "Failed", backupJob.Status.Failed)
			return ctrl.Result{Requeue: false}, nil
		}

		logger.Info("backup complete")

		// The MedusaBackupJob is finished successfully and we now need to create the MedusaBackup object.
		backupSummary, err := r.getBackupSummary(ctx, backupJob, pods, logger)
		if err != nil {
			logger.Error(err, "Failed to get backup summary")
			return ctrl.Result{}, err
		}
		if err := r.createMedusaBackup(ctx, backupJob, backupSummary, logger); err != nil {
			logger.Error(err, "Failed to create MedusaBackup")
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: false}, nil
	}

	logger.Info("Backups have not been started yet")

	// Make sure that Medusa is deployed
	if !shared.IsMedusaDeployed(pods) {
		// TODO generate event and/or update status to indicate error condition
		logger.Error(medusa.BackupSidecarNotFound, "medusa is not deployed", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{}, medusa.BackupSidecarNotFound
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
		_, err := doMedusaBackup(ctx, backupJob.ObjectMeta.Name, backupJob.Spec.Type, &p, r.ClientFactory, logger)
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

func (r *MedusaBackupJobReconciler) getBackupSummary(ctx context.Context, backup *medusav1alpha1.MedusaBackupJob, pods []corev1.Pod, logger logr.Logger) (*medusa.BackupSummary, error) {
	for _, pod := range pods {
		if remoteBackups, err := GetBackups(ctx, &pod, r.ClientFactory); err != nil {
			logger.Error(err, "failed to list backups", "CassandraPod", pod.Name)
			return nil, err
		} else {
			for _, remoteBackup := range remoteBackups {
				if remoteBackup == nil {
					err := fmt.Errorf("backup %s summary is nil", backup.Name)
					logger.Error(err, "remote backup is nil")
					return nil, err
				}
				logger.Info("found backup", "CassandraPod", pod.Name, "Backup", remoteBackup.BackupName)
				if backup.ObjectMeta.Name == remoteBackup.BackupName {
					return remoteBackup, nil
				}
				logger.Info("backup name does not match", "CassandraPod", pod.Name, "Backup", remoteBackup.BackupName)
			}
		}
	}
	return nil, reconcile.TerminalError(fmt.Errorf("backup summary couldn't be found"))
}

func (r *MedusaBackupJobReconciler) createMedusaBackup(ctx context.Context, backup *medusav1alpha1.MedusaBackupJob, backupSummary *medusa.BackupSummary, logger logr.Logger) error {
	// Create a MedusaBackup object after a successful MedusaBackupJob execution.
	logger.Info("Creating MedusaBackup object", "MedusaBackup", backup.Name)
	backupKey := types.NamespacedName{Namespace: backup.ObjectMeta.Namespace, Name: backup.Name}
	backupResource := &medusav1alpha1.MedusaBackup{}
	if err := r.Get(ctx, backupKey, backupResource); err != nil {
		if apiErrors.IsNotFound(err) {
			// Backup doesn't exist, create it
			startTime := backup.Status.StartTime
			finishTime := metav1.Now()
			logger.Info("Creating Cassandra Backup", "Backup", backup.Name)
			backupResource := &medusav1alpha1.MedusaBackup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      backup.ObjectMeta.Name,
					Namespace: backup.ObjectMeta.Namespace,
				},
				Spec: medusav1alpha1.MedusaBackupSpec{
					CassandraDatacenter: backup.Spec.CassandraDatacenter,
					Type:                backup.Spec.Type,
				},
			}
			if err := r.Create(ctx, backupResource); err != nil {
				logger.Error(err, "failed to create backup", "MedusaBackup", backup.Name)
				return err
			} else {
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
			}
		}
	}
	return nil
}

func doMedusaBackup(ctx context.Context, name string, backupType shared.BackupType, pod *corev1.Pod, clientFactory medusa.ClientFactory, logger logr.Logger) (string, error) {
	addr := net.JoinHostPort(pod.Status.PodIP, fmt.Sprint(shared.BackupSidecarPort))
	logger.Info("connecting to backup sidecar", "Pod", pod.Name, "Address", addr)
	if medusaClient, err := clientFactory.NewClient(ctx, addr); err != nil {
		return "", err
	} else {
		logger.Info("successfully connected to backup sidecar", "Pod", pod.Name, "Address", addr)
		defer medusaClient.Close()
		resp, err := medusaClient.CreateBackup(ctx, name, string(backupType))
		if err != nil {
			return "", err
		}

		return resp.BackupName, nil
	}
}

func backupStatus(ctx context.Context, name string, pod *corev1.Pod, clientFactory medusa.ClientFactory, logger logr.Logger) (medusa.StatusType, error) {
	addr := net.JoinHostPort(pod.Status.PodIP, fmt.Sprint(shared.BackupSidecarPort))
	logger.Info("connecting to backup sidecar", "Pod", pod.Name, "Address", addr)
	if medusaClient, err := clientFactory.NewClient(ctx, addr); err != nil {
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
