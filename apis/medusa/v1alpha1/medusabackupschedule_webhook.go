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
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *MedusaBackupSchedule) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-medusa-k8ssandra-io-v1alpha1-medusabackupschedule,mutating=false,failurePolicy=fail,sideEffects=None,groups=medusa.k8ssandra.io,resources=medusabackupschedules,verbs=create;update,versions=v1alpha1,name=vmedusabackupschedule.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &MedusaBackupSchedule{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *MedusaBackupSchedule) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validateCronSchedule()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *MedusaBackupSchedule) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return nil, r.validateCronSchedule()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *MedusaBackupSchedule) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *MedusaBackupSchedule) validateCronSchedule() error {
	if _, err := cron.ParseStandard(r.Spec.CronSchedule); err != nil {
		// The schedule is in incorrect format
		return err
	}

	return nil
}
