/*
Copyright 2022.

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
	"sync"

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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/shared"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

// MedusaTaskReconciler reconciles a MedusaTask object
type MedusaTaskReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme *runtime.Scheme
	medusa.ClientFactory
}

// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusatasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusatasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusatasks/finalizers,verbs=update
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=cassandradatacenters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace="k8ssandra",resources=pods;services,verbs=get;list;watch

func (r *MedusaTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rootLogger := log.FromContext(ctx).WithValues("MedusaTask", req.NamespacedName)

	// Fetch the MedusaTask instance
	instance := &medusav1alpha1.MedusaTask{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		rootLogger.Error(err, "Failed to get MedusaTask")
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	task := instance.DeepCopy()
	logger := rootLogger.WithValues("cassdc", task.Spec.CassandraDatacenter)
	logger.Info("Starting reconciliation for MedusaTask")

	cassdcKey := types.NamespacedName{Namespace: task.Namespace, Name: task.Spec.CassandraDatacenter}
	cassdc := &cassdcapi.CassandraDatacenter{}
	err = r.Get(ctx, cassdcKey, cassdc)
	if err != nil {
		logger.Error(err, "failed to get cassandradatacenter", "CassandraDatacenter", cassdcKey)
		return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
	}

	// Set an owner reference on the task so that it can be cleaned up when the cassandra datacenter is deleted
	if task.OwnerReferences == nil {
		if err = controllerutil.SetControllerReference(cassdc, task, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		}
		if err = r.Update(ctx, task); err != nil {
			logger.Error(err, "failed to update task with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
		} else {
			logger.Info("updated task with owner reference", "CassandraDatacenter", cassdcKey)
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}
	}

	// If the task is already finished, there is nothing to do.
	if taskFinished(task) {
		logger.Info("Task is already finished", "MedusaTask", req.NamespacedName, "Operation", task.Spec.Operation)
		return ctrl.Result{Requeue: false}, nil
	}

	// First check to see if the task is already in progress
	if !task.Status.StartTime.IsZero() {
		// If there is anything in progress, simply requeue the request
		if len(task.Status.InProgress) > 0 {
			logger.Info("Tasks already in progress")
			return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
		}

		logger.Info("task complete")

		// Schedule a sync if the task is a purge
		if task.Spec.Operation == medusav1alpha1.OperationTypePurge {
			if err := r.scheduleSyncForPurge(task); err != nil {
				logger.Error(err, "failed to schedule sync for purge")
				return ctrl.Result{}, err
			}
		}

		// Set the finish time
		// Note that the time here is not accurate, but that is ok. For now we are just
		// using it as a completion marker.
		patch := client.MergeFrom(task.DeepCopy())

		task.Status.FinishTime = metav1.Now()
		if err := r.Status().Patch(ctx, task, patch); err != nil {
			logger.Error(err, "failed to patch status with finish time")
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: false}, nil
	}

	// Task hasn't started yet

	pods, err := medusa.GetCassandraDatacenterPods(ctx, cassdc, r, logger)
	if err != nil {
		logger.Error(err, "Failed to get datacenter pods")
		return ctrl.Result{}, err
	}

	if task.Spec.Operation == medusav1alpha1.OperationTypePurge {
		return r.purgeOperation(ctx, task, pods, logger)
	} else if task.Spec.Operation == medusav1alpha1.OperationTypeSync {
		return r.syncOperation(ctx, task, pods, logger)
	} else if task.Spec.Operation == medusav1alpha1.OperationTypePrepareRestore {
		return r.prepareRestoreOperation(ctx, task, pods, logger)
	} else {
		return ctrl.Result{}, fmt.Errorf("unsupported operation %s", task.Spec.Operation)
	}
}

func (r *MedusaTaskReconciler) prepareRestoreOperation(ctx context.Context, task *medusav1alpha1.MedusaTask, pods []corev1.Pod, logger logr.Logger) (reconcile.Result, error) {
	logger.Info("Starting prepare restore operations", "cassdc", task.Spec.CassandraDatacenter)

	op := func(ctx context.Context, task *medusav1alpha1.MedusaTask, pod corev1.Pod) (*medusa.PurgeBackupsResponse, error) {
		return prepareRestore(ctx, task, &pod, r.ClientFactory)
	}

	return r.executePodOperations(ctx, task, pods, "prepare restore", op, logger)
}

func (r *MedusaTaskReconciler) purgeOperation(ctx context.Context, task *medusav1alpha1.MedusaTask, pods []corev1.Pod, logger logr.Logger) (reconcile.Result, error) {
	logger.Info("Starting purge operations", "cassdc", task.Spec.CassandraDatacenter)

	op := func(ctx context.Context, task *medusav1alpha1.MedusaTask, pod corev1.Pod) (*medusa.PurgeBackupsResponse, error) {
		return doPurge(ctx, task, &pod, r.ClientFactory)
	}

	return r.executePodOperations(ctx, task, pods, "purge", op, logger)
}

type podOperation func(ctx context.Context, task *medusav1alpha1.MedusaTask, pod corev1.Pod) (*medusa.PurgeBackupsResponse, error)

func (r *MedusaTaskReconciler) executePodOperations(
	ctx context.Context,
	task *medusav1alpha1.MedusaTask,
	pods []corev1.Pod,
	operationName string,
	operation podOperation,
	logger logr.Logger) (reconcile.Result, error) {
	// Set the start time and add the pods to the in progress list
	patch := client.MergeFrom(task.DeepCopy())
	task.Status.StartTime = metav1.Now()
	for _, pod := range pods {
		task.Status.InProgress = append(task.Status.InProgress, pod.Name)
	}

	if err := r.Status().Patch(ctx, task, patch); err != nil {
		logger.Error(err, "Failed to patch status")
		// We received a stale object, requeue for next processing
		return ctrl.Result{}, err
	}

	go func() {
		wg := sync.WaitGroup{}

		// Mutex to prevent concurrent updates to the Status object
		mutex := sync.Mutex{}
		patch := client.MergeFrom(task.DeepCopy())

		opLogger := logger.WithValues("Operation", operationName)

		for _, p := range pods {
			pod := p
			wg.Add(1)
			go func() {
				opLogger.Info("starting pod operation", "CassandraPod", pod.Name)
				succeeded := false
				purgeBackupsResponse, err := operation(ctx, task, p)
				if err == nil {
					logger.Info("finished pod operation", "CassandraPod", pod.Name)
					succeeded = true
				} else {
					logger.Error(err, "pod operation failed", "CassandraPod", pod.Name)
				}
				mutex.Lock()
				defer mutex.Unlock()
				defer wg.Done()
				task.Status.InProgress = utils.RemoveValue(task.Status.InProgress, pod.Name)
				var taskResult medusav1alpha1.TaskResult
				if succeeded {
					if purgeBackupsResponse == nil {
						taskResult = medusav1alpha1.TaskResult{
							PodName: pod.Name,
						}
					} else {
						taskResult = medusav1alpha1.TaskResult{
							PodName:                   pod.Name,
							NbBackupsPurged:           int(purgeBackupsResponse.NbBackupsPurged),
							NbObjectsPurged:           int(purgeBackupsResponse.NbObjectsPurged),
							TotalPurgedSize:           int(purgeBackupsResponse.TotalPurgedSize),
							TotalObjectsWithinGcGrace: int(purgeBackupsResponse.TotalObjectsWithinGcGrace),
						}
					}
					task.Status.Finished = append(task.Status.Finished, taskResult)
				} else {
					task.Status.Failed = append(task.Status.Failed, pod.Name)
				}
			}()
			wg.Wait()
			logger.Info("finished task operations")
			if err := r.Status().Patch(context.Background(), task, patch); err != nil {
				logger.Error(err, "failed to patch status", "MedusaTask", fmt.Sprint(task))
			}
		}
	}()

	return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
}

func (r *MedusaTaskReconciler) syncOperation(ctx context.Context, task *medusav1alpha1.MedusaTask, pods []corev1.Pod, logger logr.Logger) (reconcile.Result, error) {
	logger.Info("Starting sync operation")
	patch := client.MergeFrom(task.DeepCopy())
	task.Status.StartTime = metav1.Now()
	if err := r.Status().Patch(ctx, task, patch); err != nil {
		logger.Error(err, "failed to patch status", "MedusaTask", fmt.Sprint(task))
		return ctrl.Result{}, err
	}
	for _, pod := range pods {
		logger.Info("Listing Backups...", "CassandraPod", pod.Name)
		if remoteBackups, err := GetBackups(ctx, &pod, r.ClientFactory); err != nil {
			logger.Error(err, "failed to list backups", "CassandraPod", pod.Name)
		} else {
			for _, backup := range remoteBackups {
				logger.Info("Syncing Backup", "Backup", backup.BackupName)
				// Create backups that should exist but are missing
				backupKey := types.NamespacedName{Namespace: task.Namespace, Name: backup.BackupName}
				backupResource := &medusav1alpha1.MedusaBackup{}
				if err := r.Get(ctx, backupKey, backupResource); err != nil {
					if errors.IsNotFound(err) {
						// Backup doesn't exist, create it
						shouldReturn, ctrlResult, err := createMedusaBackup(logger, backup, task.Spec.CassandraDatacenter, task.Namespace, r, ctx)
						if shouldReturn {
							return ctrlResult, err
						}
					} else {
						logger.Error(err, "failed to get backup", "Backup", backup.BackupName)
						return ctrl.Result{}, err
					}
				}
			}

			// Delete backups that don't exist remotely but are found locally
			localBackups := &medusav1alpha1.MedusaBackupList{}
			if err = r.List(ctx, localBackups, client.InNamespace(task.Namespace)); err != nil {
				logger.Error(err, "failed to list backups")
				return ctrl.Result{}, err
			}
			for _, backup := range localBackups.Items {
				if !backupExistsRemotely(remoteBackups, backup.ObjectMeta.Name) && backup.Spec.CassandraDatacenter == task.Spec.CassandraDatacenter {
					logger.Info("Deleting Cassandra Backup", "Backup", backup.ObjectMeta.Name)
					if err := r.Delete(ctx, &backup); err != nil {
						logger.Error(err, "failed to delete backup", "MedusaBackup", backup.ObjectMeta.Name)
						return ctrl.Result{}, err
					} else {
						logger.Info("Deleted Cassandra Backup", "Backup", backup.ObjectMeta.Name)
					}
				}
			}

			// Update task status at the end of the reconcile
			logger.Info("finished task operations", "MedusaTask", fmt.Sprint(task))
			finishPatch := client.MergeFrom(task.DeepCopy())
			task.Status.FinishTime = metav1.Now()
			taskResult := medusav1alpha1.TaskResult{PodName: pod.Name}
			task.Status.Finished = append(task.Status.Finished, taskResult)
			if err := r.Status().Patch(ctx, task, finishPatch); err != nil {
				logger.Error(err, "failed to patch status", "MedusaTask", fmt.Sprint(task))
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{RequeueAfter: r.DefaultDelay}, nil
}

func createMedusaBackup(logger logr.Logger, backup *medusa.BackupSummary, datacenter, namespace string, r *MedusaTaskReconciler, ctx context.Context) (bool, reconcile.Result, error) {
	logger.Info("Creating Cassandra Backup", "Backup", backup.BackupName)
	startTime := metav1.Unix(backup.StartTime, 0)
	finishTime := metav1.Unix(backup.FinishTime, 0)
	backupResource := &medusav1alpha1.MedusaBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.BackupName,
			Namespace: namespace,
		},
		Spec: medusav1alpha1.MedusaBackupSpec{
			CassandraDatacenter: datacenter,
			Type:                shared.BackupType(backup.BackupType),
		},
	}
	if err := r.Create(ctx, backupResource); err != nil {
		logger.Error(err, "failed to create backup", "MedusaBackup", backup.BackupName)
		return true, ctrl.Result{}, err
	} else {
		logger.Info("Created Medusa Backup", "Backup", backupResource)
		backupPatch := client.MergeFrom(backupResource.DeepCopy())
		backupResource.Status.StartTime = startTime
		backupResource.Status.FinishTime = finishTime
		backupResource.Status.TotalNodes = backup.TotalNodes
		backupResource.Status.FinishedNodes = backup.FinishedNodes
		backupResource.Status.Nodes = make([]*medusav1alpha1.MedusaBackupNode, len(backup.Nodes))
		for i, node := range backup.Nodes {
			backupResource.Status.Nodes[i] = &medusav1alpha1.MedusaBackupNode{
				Host:       node.Host,
				Tokens:     node.Tokens,
				Datacenter: node.Datacenter,
				Rack:       node.Rack,
			}
		}
		backupResource.Status.Status = backup.Status.String()

		if err := r.Status().Patch(ctx, backupResource, backupPatch); err != nil {
			logger.Error(err, "failed to patch status with finish time")
			return true, ctrl.Result{}, err
		}

	}
	return false, reconcile.Result{}, nil
}

// If the task operation was a purge, we may need to schedule a sync operation next
func (r *MedusaTaskReconciler) scheduleSyncForPurge(task *medusav1alpha1.MedusaTask) error {
	// Check if sync operation exists for this task, and create it if it doesn't
	sync := &medusav1alpha1.MedusaTask{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: task.GetObjectMeta().GetName() + "-sync", Namespace: task.Namespace}, sync); err != nil {
		if errors.IsNotFound(err) {
			// Create the sync task
			sync = &medusav1alpha1.MedusaTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      task.GetObjectMeta().GetName() + "-sync",
					Namespace: task.Namespace,
				},
				Spec: medusav1alpha1.MedusaTaskSpec{
					Operation:           medusav1alpha1.OperationTypeSync,
					CassandraDatacenter: task.Spec.CassandraDatacenter,
				},
			}
			if err := r.Client.Create(context.Background(), sync); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func doPurge(ctx context.Context, task *medusav1alpha1.MedusaTask, pod *corev1.Pod, clientFactory medusa.ClientFactory) (*medusa.PurgeBackupsResponse, error) {
	addr := net.JoinHostPort(pod.Status.PodIP, fmt.Sprint(shared.BackupSidecarPort))
	if medusaClient, err := clientFactory.NewClient(addr); err != nil {
		return nil, err
	} else {
		defer medusaClient.Close()
		return medusaClient.PurgeBackups(ctx)
	}
}

func prepareRestore(ctx context.Context, task *medusav1alpha1.MedusaTask, pod *corev1.Pod, clientFactory medusa.ClientFactory) (*medusa.PurgeBackupsResponse, error) {
	addr := net.JoinHostPort(pod.Status.PodIP, fmt.Sprint(shared.BackupSidecarPort))
	if medusaClient, err := clientFactory.NewClient(addr); err != nil {
		return nil, err
	} else {
		defer medusaClient.Close()
		_, err = medusaClient.PrepareRestore(ctx, task.Spec.CassandraDatacenter, task.Spec.BackupName, task.Spec.RestoreKey)
		return nil, err
	}
}

func GetBackups(ctx context.Context, pod *corev1.Pod, clientFactory medusa.ClientFactory) ([]*medusa.BackupSummary, error) {
	addr := net.JoinHostPort(pod.Status.PodIP, fmt.Sprint(shared.BackupSidecarPort))
	if medusaClient, err := clientFactory.NewClient(addr); err != nil {
		return nil, err
	} else {
		defer medusaClient.Close()
		return medusaClient.GetBackups(ctx)
	}
}

func backupExistsRemotely(backups []*medusa.BackupSummary, backupName string) bool {
	for _, backup := range backups {
		if backup.BackupName == backupName {
			return true
		}
	}
	return false
}

func taskFinished(task *medusav1alpha1.MedusaTask) bool {
	return !task.Status.FinishTime.IsZero()
}

// SetupWithManager sets up the controller with the Manager.
func (r *MedusaTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&medusav1alpha1.MedusaTask{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
