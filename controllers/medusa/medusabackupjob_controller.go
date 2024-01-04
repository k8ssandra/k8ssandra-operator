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
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	backup := instance.DeepCopy()

	cassdcKey := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.CassandraDatacenter}
	cassdc := &cassdcapi.CassandraDatacenter{}
	err = r.Get(ctx, cassdcKey, cassdc)
	if err != nil {
		logger.Error(err, "failed to get cassandradatacenter", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{}, err
	}

	// Set an owner reference on the backup job so that it can be cleaned up when the cassandra datacenter is deleted
	if backup.OwnerReferences == nil {
		if err = controllerutil.SetControllerReference(cassdc, backup, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{}, err
		}
		if err = r.Update(ctx, backup); err != nil {
			logger.Error(err, "failed to update MedusaBackupJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{}, err
		} else {
			logger.Info("updated MedusaBackupJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{}, nil
		}
	}

	pods, err := medusa.GetCassandraDatacenterPods(ctx, cassdc, r, logger)
	if err != nil {
		logger.Error(err, "Failed to get datacenter pods")
		return ctrl.Result{}, err
	}

	// If there is anything in progress, simply requeue the request until each pod has finished or errored
	if len(backup.Status.InProgress) > 0 {
		logger.Info("There are backups in progress, checking them..")
		progress := make([]string, 0, len(backup.Status.InProgress))
		patch := client.MergeFrom(backup.DeepCopy())

	StatusCheck:
		for _, podName := range backup.Status.InProgress {
			for _, pod := range pods {
				if podName == pod.Name {
					status, err := backupStatus(ctx, backup.ObjectMeta.Name, &pod, r.ClientFactory, logger)
					if err != nil {
						return ctrl.Result{}, err
					}

					if status == medusa.StatusType_IN_PROGRESS {
						progress = append(progress, podName)
					} else if status == medusa.StatusType_SUCCESS {
						backup.Status.Finished = append(backup.Status.Finished, podName)
					} else if status == medusa.StatusType_FAILED || status == medusa.StatusType_UNKNOWN {
						backup.Status.Failed = append(backup.Status.Failed, podName)
					}

					continue StatusCheck
				}
			}
		}

		if len(backup.Status.InProgress) != len(progress) {
			backup.Status.InProgress = progress
			if err := r.Status().Patch(ctx, backup, patch); err != nil {
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
	if medusaBackupFinished(backup) {
		logger.Info("Backup operation is already finished")
		return ctrl.Result{Requeue: false}, nil
	}

	// First check to see if the backup is already in progress
	if !backup.Status.StartTime.IsZero() {
		// If there is anything in progress, simply requeue the request
		if len(backup.Status.InProgress) > 0 {
			logger.Info("Backup is still in progress")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		logger.Info("backup complete")

		// The MedusaBackupJob is finished and we now need to create the MedusaBackup object.
		backupSummary, err := r.getBackupSummary(ctx, backup, pods, logger)
		if err != nil {
			logger.Error(err, "Failed to get backup summary")
			return ctrl.Result{}, err
		}
		if err := r.createMedusaBackup(ctx, backup, backupSummary, logger); err != nil {
			logger.Error(err, "Failed to create MedusaBackup")
			return ctrl.Result{}, err
		}

		// Set the finish time
		// Note that the time here is not accurate, but that is ok. For now we are just
		// using it as a completion marker.
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.FinishTime = metav1.Now()
		if err := r.Status().Patch(ctx, backup, patch); err != nil {
			logger.Error(err, "failed to patch status with finish time")
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

	patch := client.MergeFromWithOptions(backup.DeepCopy(), client.MergeFromWithOptimisticLock{})

	backup.Status.StartTime = metav1.Now()

	if err := r.Status().Patch(ctx, backup, patch); err != nil {
		logger.Error(err, "Failed to patch status")
		// We received a stale object, requeue for next processing
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}

	logger.Info("Starting backups")
	patch = client.MergeFrom(backup.DeepCopy())

	for _, p := range pods {
		logger.Info("starting backup", "CassandraPod", p.Name)
		_, err := doMedusaBackup(ctx, backup.ObjectMeta.Name, backup.Spec.Type, &p, r.ClientFactory, logger)
		if err != nil {
			logger.Error(err, "backup failed", "CassandraPod", p.Name)
		}

		backup.Status.InProgress = append(backup.Status.InProgress, p.Name)
	}
	// logger.Info("finished backup operations")
	if err := r.Status().Patch(context.Background(), backup, patch); err != nil {
		logger.Error(err, "failed to patch status", "Backup", fmt.Sprintf("%s/%s", backup.Name, backup.Namespace))
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
				logger.Info("found backup", "CassandraPod", pod.Name, "Backup", remoteBackup.BackupName)
				if backup.ObjectMeta.Name == remoteBackup.BackupName {
					return remoteBackup, nil
				}
				logger.Info("backup name does not match", "CassandraPod", pod.Name, "Backup", remoteBackup.BackupName)
			}
		}
	}
	return nil, nil
}

func (r *MedusaBackupJobReconciler) createMedusaBackup(ctx context.Context, backup *medusav1alpha1.MedusaBackupJob, backupSummary *medusa.BackupSummary, logger logr.Logger) error {
	// Create a MedusaBackup object after a successful MedusaBackupJob execution.
	logger.Info("Creating MedusaBackup object", "MedusaBackup", backup.Name)
	backupKey := types.NamespacedName{Namespace: backup.ObjectMeta.Namespace, Name: backup.Name}
	backupResource := &medusav1alpha1.MedusaBackup{}
	if err := r.Get(ctx, backupKey, backupResource); err != nil {
		if errors.IsNotFound(err) {
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
		return medusa.StatusType_UNKNOWN, err
	} else {
		resp, err := medusaClient.BackupStatus(ctx, name)
		if err != nil {
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
