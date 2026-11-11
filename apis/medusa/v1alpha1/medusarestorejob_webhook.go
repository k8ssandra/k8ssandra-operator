/*
Copyright 2026.

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

	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupMedusaRestoreJobWebhookWithManager registers the MedusaRestoreJob validating webhook with the manager.
func SetupMedusaRestoreJobWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &MedusaRestoreJob{}).
		WithValidator(&MedusaRestoreJobValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-medusa-k8ssandra-io-v1alpha1-medusarestorejob,mutating=false,failurePolicy=fail,sideEffects=None,groups=medusa.k8ssandra.io,resources=medusarestorejobs,verbs=create;update,versions=v1alpha1,name=vmedusarestorejob.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*MedusaRestoreJob] = &MedusaRestoreJobValidator{}

// MedusaRestoreJobValidator validates MedusaRestoreJob objects at admission using ValidateRestoreJobSpec.
// It does not read any other object from the cluster.
type MedusaRestoreJobValidator struct {
}

// ValidateCreate implements admission.Validator.
func (v *MedusaRestoreJobValidator) ValidateCreate(ctx context.Context, obj *MedusaRestoreJob) (admission.Warnings, error) {
	return nil, ValidateRestoreJobSpec(&obj.Spec)
}

// ValidateUpdate implements admission.Validator. An update that leaves the spec unchanged is always allowed,
// so metadata-only changes (for example removing a finalizer) are never blocked. The spec cannot change once
// the restore has started (status.startTime is set), because the restore mapping was already built from it.
func (v *MedusaRestoreJobValidator) ValidateUpdate(ctx context.Context, old *MedusaRestoreJob, new *MedusaRestoreJob) (admission.Warnings, error) {
	if equality.Semantic.DeepEqual(old.Spec, new.Spec) {
		return nil, nil
	}
	if !old.Status.StartTime.IsZero() {
		return nil, fmt.Errorf("spec cannot be changed after the restore has started (status.startTime is set)")
	}
	return nil, ValidateRestoreJobSpec(&new.Spec)
}

// ValidateDelete implements admission.Validator. Delete is always allowed.
func (v *MedusaRestoreJobValidator) ValidateDelete(ctx context.Context, obj *MedusaRestoreJob) (admission.Warnings, error) {
	return nil, nil
}
