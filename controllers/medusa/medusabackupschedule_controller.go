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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cron "github.com/robfig/cron/v3"
)

type Clock interface {
	Now() time.Time
}

// MedusaBackupScheduleReconciler reconciles a MedusaBackupSchedule object
type MedusaBackupScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Clock  Clock
}

type RealClock struct{}

func (r *RealClock) Now() time.Time {
	return time.Now()
}

//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackupschedules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackupschedules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackupschedules/finalizers,verbs=update
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusabackupjobs,verbs=get;list;watch;create

func (r *MedusaBackupScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("medusabackupscheduler", req.NamespacedName)

	backupSchedule := &medusav1alpha1.MedusaBackupSchedule{}
	err := r.Get(ctx, req.NamespacedName, backupSchedule)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	sched, err := cron.ParseStandard(backupSchedule.Spec.CronSchedule)
	if err != nil {
		// The schedule is in incorrect format
		return ctrl.Result{}, err
	}

	dcKey := types.NamespacedName{Namespace: backupSchedule.Namespace, Name: backupSchedule.Spec.BackupSpec.CassandraDatacenter}
	dc := &cassdcapi.CassandraDatacenter{}
	if err := r.Get(ctx, dcKey, dc); err != nil {
		logger.Error(err, "failed to get cassandradatacenter", "CassandraDatacenter", dcKey)
		return ctrl.Result{}, err
	}

	// Set an owner reference on the task so that it can be cleaned up when the cassandra datacenter is deleted
	if backupSchedule.OwnerReferences == nil {
		if err = controllerutil.SetControllerReference(dc, backupSchedule, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "CassandraDatacenter", dcKey)
			return ctrl.Result{}, err
		}
		if err = r.Update(ctx, backupSchedule); err != nil {
			logger.Error(err, "failed to update task with owner reference", "CassandraDatacenter", dcKey)
			return ctrl.Result{}, err
		} else {
			logger.Info("updated task with owner reference", "CassandraDatacenter", dcKey)
		}
	}

	defaults(backupSchedule)

	previousExecution, err := getPreviousExecutionTime(ctx, backupSchedule)
	if err != nil {
		return ctrl.Result{}, err
	}

	now := r.Clock.Now().UTC()

	// Calculate the next execution time
	nextExecution := sched.Next(previousExecution).UTC()

	createBackup := false
	createPurge := false

	if nextExecution.Before(now) {
		if backupSchedule.Spec.ConcurrencyPolicy == batchv1.ForbidConcurrent {
			if activeTasks, err := r.activeTasks(backupSchedule, dc, backupSchedule.Spec.OperationType); err != nil {
				logger.V(1).Info("failed to get activeTasks", "error", err)
				return ctrl.Result{}, err
			} else {
				if activeTasks > 0 {
					logger.V(1).Info("Postponing backup schedule due to an unfinished existing job", "MedusaBackupSchedule", req.NamespacedName)
					return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
				}
			}
		}
		nextExecution = sched.Next(now)
		previousExecution = now
		createBackup = true && !backupSchedule.Spec.Disabled && (backupSchedule.Spec.OperationType == "backup" || backupSchedule.Spec.OperationType == "")
		createPurge = true && !backupSchedule.Spec.Disabled && (backupSchedule.Spec.OperationType == string(medusav1alpha1.OperationTypePurge))
	}

	// Update the status if there are modifications
	if backupSchedule.Status.LastExecution.Time.Before(previousExecution) ||
		backupSchedule.Status.NextSchedule.Time.Before(nextExecution) {
		backupSchedule.Status.NextSchedule = metav1.NewTime(nextExecution)
		backupSchedule.Status.LastExecution = metav1.NewTime(previousExecution)

		if err := r.Client.Status().Update(ctx, backupSchedule); err != nil {
			return ctrl.Result{}, err
		}
	}

	if createBackup {
		logger.V(1).Info("Scheduled time has been reached, creating a backup job", "MedusaBackupSchedule", req.NamespacedName)
		generatedName := fmt.Sprintf("%s-%d", backupSchedule.Name, now.Unix())
		backupJob := &medusav1alpha1.MedusaBackupJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generatedName,
				Namespace: backupSchedule.Namespace,
				Labels:    dc.GetDatacenterLabels(),
			},
			Spec: backupSchedule.Spec.BackupSpec,
		}

		if err := r.Client.Create(ctx, backupJob); err != nil {
			// We've already updated the Status times.. we'll miss this job now?
			return ctrl.Result{}, err
		}
	}

	if createPurge {
		logger.V(1).Info("Scheduled time has been reached, creating a backup purge job", "MedusaBackupSchedule", req.NamespacedName)
		generatedName := fmt.Sprintf("%s-%d", backupSchedule.Name, now.Unix())
		purgeJob := &medusav1alpha1.MedusaTask{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generatedName,
				Namespace: backupSchedule.Namespace,
				Labels:    dc.GetDatacenterLabels(),
			},
			Spec: medusav1alpha1.MedusaTaskSpec{
				CassandraDatacenter: backupSchedule.Spec.BackupSpec.CassandraDatacenter,
				Operation:           medusav1alpha1.OperationTypePurge,
			},
		}

		if err := r.Client.Create(ctx, purgeJob); err != nil {
			// We've already updated the Status times.. we'll miss this job now?
			return ctrl.Result{}, err
		}
	}

	nextRunTime := nextExecution.Sub(now)
	logger.V(1).Info("Requeing for next scheduled event", "nextRuntime", nextRunTime.String())
	return ctrl.Result{RequeueAfter: nextRunTime}, nil
}

func getPreviousExecutionTime(ctx context.Context, backupSchedule *medusav1alpha1.MedusaBackupSchedule) (time.Time, error) {
	previousExecution := backupSchedule.Status.LastExecution

	if previousExecution.IsZero() {
		// This job has never been executed, we use creationTimestamp
		previousExecution = backupSchedule.CreationTimestamp
	}

	return previousExecution.Time.UTC(), nil
}

func defaults(backupSchedule *medusav1alpha1.MedusaBackupSchedule) {
	if backupSchedule.Spec.ConcurrencyPolicy == "" {
		backupSchedule.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	}
}

func (r *MedusaBackupScheduleReconciler) activeTasks(backupSchedule *medusav1alpha1.MedusaBackupSchedule, dc *cassdcapi.CassandraDatacenter, operationType string) (int, error) {
	if operationType == "" || operationType == "backup" {
		backupJobs := &medusav1alpha1.MedusaBackupJobList{}
		if err := r.Client.List(context.Background(), backupJobs, client.InNamespace(backupSchedule.Namespace), client.MatchingLabels(dc.GetDatacenterLabels())); err != nil {
			return 0, err
		}
		activeJobs := make([]medusav1alpha1.MedusaBackupJob, 0)
		for _, job := range backupJobs.Items {
			if job.Status.FinishTime.IsZero() {
				activeJobs = append(activeJobs, job)
			}
		}

		return len(activeJobs), nil
	} else if operationType == string(medusav1alpha1.OperationTypePurge) {
		medusaTasks := &medusav1alpha1.MedusaTaskList{}
		if err := r.Client.List(context.Background(), medusaTasks, client.InNamespace(backupSchedule.Namespace), client.MatchingLabels(dc.GetDatacenterLabels())); err != nil {
			return 0, err
		}
		activeJobs := make([]medusav1alpha1.MedusaTask, 0)
		for _, job := range medusaTasks.Items {
			if job.Spec.Operation == medusav1alpha1.OperationTypePurge && job.Status.FinishTime.IsZero() {
				activeJobs = append(activeJobs, job)
			}
		}

		return len(activeJobs), nil
	}
	return 0, fmt.Errorf("unknown operation type %s", operationType)
}

// SetupWithManager sets up the controller with the Manager.
func (r *MedusaBackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&medusav1alpha1.MedusaBackupSchedule{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
