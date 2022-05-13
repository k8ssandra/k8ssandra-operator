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
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
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
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
	}

	backup := instance.DeepCopy()

	cassdcKey := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.CassandraDatacenter}
	cassdc := &cassdcapi.CassandraDatacenter{}
	err = r.Get(ctx, cassdcKey, cassdc)
	if err != nil {
		logger.Error(err, "failed to get cassandradatacenter", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
	}

	// Set an owner reference on the backup job so that it can be cleaned up when the cassandra datacenter is deleted
	if backup.OwnerReferences == nil {
		if err = controllerutil.SetControllerReference(cassdc, backup, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		}
		if err = r.Update(ctx, backup); err != nil {
			logger.Error(err, "failed to update MedusaBackupJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else {
			logger.Info("updated MedusaBackupJob with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	}

	pods, err := medusa.GetCassandraDatacenterPods(ctx, cassdc, r, logger)
	if err != nil {
		logger.Error(err, "Failed to get datacenter pods")
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
	}

	// If there is anything in progress, simply requeue the request
	if len(backup.Status.InProgress) > 0 {
		logger.Info("MedusaBackupJob is being processed already", "Backup", req.NamespacedName)
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
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
			logger.Info("Backups already in progress")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		logger.Info("backup complete")

		// The MedusaBackupJob is finished and we now need to create the MedusaBackup object.
		if err := r.createMedusaBackup(ctx, backup, logger); err != nil {
			logger.Error(err, "Failed to create MedusaBackup")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		}

		// Set the finish time
		// Note that the time here is not accurate, but that is ok. For now we are just
		// using it as a completion marker.
		patch := client.MergeFrom(backup.DeepCopy())
		backup.Status.FinishTime = metav1.Now()
		if err := r.Status().Patch(ctx, backup, patch); err != nil {
			logger.Error(err, "failed to patch status with finish time")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		}

		return ctrl.Result{Requeue: false}, nil
	}

	logger.Info("Backups have not been started yet")

	// Make sure that Medusa is deployed
	if !shared.IsMedusaDeployed(pods) {
		// TODO generate event and/or update status to indicate error condition
		logger.Error(medusa.BackupSidecarNotFound, "medusa is not deployed", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{RequeueAfter: r.LongDelay}, medusa.BackupSidecarNotFound
	}

	patch := client.MergeFromWithOptions(backup.DeepCopy(), client.MergeFromWithOptimisticLock{})

	backup.Status.StartTime = metav1.Now()
	for _, pod := range pods {
		backup.Status.InProgress = append(backup.Status.InProgress, pod.Name)
	}

	if err := r.Status().Patch(ctx, backup, patch); err != nil {
		logger.Error(err, "Failed to patch status")
		// We received a stale object, requeue for next processing
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
	}

	logger.Info("Starting backups")
	// Do the actual backup in the background
	go func() {
		wg := sync.WaitGroup{}

		// Mutex to prevent concurrent updates to the backup.Status object
		backupMutex := sync.Mutex{}
		patch := client.MergeFrom(backup.DeepCopy())

		for _, p := range pods {
			pod := p
			wg.Add(1)
			go func() {
				logger.Info("starting backup", "CassandraPod", pod.Name)
				succeeded := false
				if err := doMedusaBackup(ctx, backup.ObjectMeta.Name, backup.Spec.Type, &pod, r.ClientFactory, logger); err == nil {
					logger.Info("finished backup", "CassandraPod", pod.Name)
					succeeded = true
				} else {
					logger.Error(err, "backup failed", "CassandraPod", pod.Name)
				}
				backupMutex.Lock()
				defer backupMutex.Unlock()
				defer wg.Done()
				backup.Status.InProgress = utils.RemoveValue(backup.Status.InProgress, pod.Name)
				if succeeded {
					backup.Status.Finished = append(backup.Status.Finished, pod.Name)
				} else {
					backup.Status.Failed = append(backup.Status.Failed, pod.Name)
				}
			}()
		}
		wg.Wait()
		logger.Info("finished backup operations")
		if err := r.Status().Patch(context.Background(), backup, patch); err != nil {
			logger.Error(err, "failed to patch status", "Backup", fmt.Sprintf("%s/%s", backup.Name, backup.Namespace))
		}
	}()

	return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
}

func (r *MedusaBackupJobReconciler) createMedusaBackup(ctx context.Context, backup *medusav1alpha1.MedusaBackupJob, logger logr.Logger) error {
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
				if err := r.Status().Patch(ctx, backupResource, backupPatch); err != nil {
					logger.Error(err, "failed to patch status with finish time")
					return err
				}
			}
		}
	}
	return nil
}

func doMedusaBackup(ctx context.Context, name string, backupType shared.BackupType, pod *corev1.Pod, clientFactory medusa.ClientFactory, logger logr.Logger) error {
	addr := fmt.Sprintf("%s:%d", pod.Status.PodIP, shared.BackupSidecarPort)
	logger.Info("connecting to backup sidecar", "Pod", pod.Name, "Address", addr)
	if medusaClient, err := clientFactory.NewClient(addr); err != nil {
		return err
	} else {
		logger.Info("successfully connected to backup sidecar", "Pod", pod.Name, "Address", addr)
		defer medusaClient.Close()
		return medusaClient.CreateBackup(ctx, name, string(backupType))
	}
}

func medusaBackupFinished(backup *medusav1alpha1.MedusaBackupJob) bool {
	return !backup.Status.FinishTime.IsZero()
}

// SetupWithManager sets up the controller with the Manager.
func (r *MedusaBackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&medusav1alpha1.MedusaBackupJob{}).
		Complete(r)
}
