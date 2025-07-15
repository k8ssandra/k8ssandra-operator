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

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func SetupMedusaBackupScheduleWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&MedusaBackupSchedule{}).
		WithValidator(&MedusaBackupScheduleValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-medusa-k8ssandra-io-v1alpha1-medusabackupschedule,mutating=false,failurePolicy=fail,sideEffects=None,groups=medusa.k8ssandra.io,resources=medusabackupschedules,verbs=create;update,versions=v1alpha1,name=vmedusabackupschedule.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &MedusaBackupScheduleValidator{}

type MedusaBackupScheduleValidator struct {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *MedusaBackupScheduleValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, validateCronSchedule(obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *MedusaBackupScheduleValidator) ValidateUpdate(ctx context.Context, old runtime.Object, new runtime.Object) (admission.Warnings, error) {
	return nil, validateCronSchedule(new)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *MedusaBackupScheduleValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateCronSchedule(obj runtime.Object) error {
	schedule, ok := obj.(*MedusaBackupSchedule)
	if !ok {
		return fmt.Errorf("expected a MedusaBackupSchedule object but got %T", obj)
	}

	if _, err := cron.ParseStandard(schedule.Spec.CronSchedule); err != nil {
		// The schedule is in incorrect format
		return err
	}

	return nil
}
