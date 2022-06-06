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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
// +kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=cassandrabackups,verbs=get;list;watch;create

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

	previousExecution, err := getPreviousExecutionTime(ctx, backupSchedule)
	if err != nil {
		return ctrl.Result{}, err
	}

	now := r.Clock.Now().UTC()

	// Calculate the next execution time
	nextExecution := sched.Next(previousExecution).UTC()

	createBackup := false

	if nextExecution.Before(now) {
		nextExecution = sched.Next(now)
		previousExecution = now
		createBackup = true && !backupSchedule.Spec.Disabled
	}

	// Update the status if there are modifications
	if backupSchedule.Status.LastExecution.Time.Before(previousExecution) ||
		backupSchedule.Status.NextSchedule.Time.Before(nextExecution) {
		schedulePatch := client.MergeFrom(backupSchedule.DeepCopy())
		backupSchedule.Status.NextSchedule = metav1.NewTime(nextExecution)
		backupSchedule.Status.LastExecution = metav1.NewTime(previousExecution)

		if err := r.Client.Patch(ctx, backupSchedule, schedulePatch); err != nil {
			return ctrl.Result{}, err
		}
	}

	if createBackup {
		// TODO Verify here no previous jobs is still executing?
		logger.V(1).Info("Scheduled time has been reached, creating a backup job", "MedusaBackupSchedule", req.NamespacedName)
		generatedName := fmt.Sprintf("%s-%d", backupSchedule.Name, now.Unix())
		backupJob := &medusav1alpha1.MedusaBackupJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generatedName,
				Namespace: backupSchedule.Namespace,
				// TODO Labels?
			},
			Spec: backupSchedule.Spec.BackupSpec,
		}

		if err := r.Client.Create(ctx, backupJob); err != nil {
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

// SetupWithManager sets up the controller with the Manager.
func (r *MedusaBackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&medusav1alpha1.MedusaBackupSchedule{}).
		Complete(r)
}
