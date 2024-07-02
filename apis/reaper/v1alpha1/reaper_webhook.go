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

package v1alpha1

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var webhookLog = logf.Log.WithName("reaper-webhook")

func (r *Reaper) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

var _ webhook.Defaulter = &Reaper{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Reaper) Default() {
	webhookLog.Info("Reaper default values", "Reaper", r.Name)
}

//+kubebuilder:webhook:path=/validate-reaper-k8ssandra-io-v1alpha1-reaper,mutating=false,failurePolicy=fail,sideEffects=None,groups=reaper.k8ssandra.io,resources=reapers,verbs=create;update,versions=v1alpha1,name=vreaper.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Reaper{}

func (r *Reaper) ValidateCreate() (admission.Warnings, error) {
	webhookLog.Info("validate Reaper create", "Reaper", r.Name)
	return nil, r.validateReaper()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Reaper) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	webhookLog.Info("validate Reaper update", "Reaper", r.Name)
	return nil, r.validateReaper()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Reaper) ValidateDelete() (admission.Warnings, error) {
	webhookLog.Info("validate Reaper delete", "Reaper", r.Name)
	return nil, nil
}

func (r *Reaper) validateReaper() error {
	if r == nil {
		return fmt.Errorf("cannot validate Reaper if its not set")
	}
	if r.Spec.HasReaperRef() && r.Spec.HasDatacenterRef() {
		return fmt.Errorf("cannot set both ReaperRef and DatacenterRef")
	}

	// we don't validate much if ReaperRef is set. It implies an external reaper is used, so we're not gonna touch those fields anyway
	if r.Spec.HasReaperRef() {
		return nil
	}
	if r.Spec.IsControlPlane() && r.Spec.StorageType != StorageTypeLocal {
		return fmt.Errorf("a Control Plane reaper can only use local storage")
	}
	if r.Spec.IsControlPlane() && r.Spec.Telemetry != nil && r.Spec.Telemetry.IsVectorEnabled() {
		return fmt.Errorf("a Control Plane reaper cannot have vector enabled")
	}
	if r.Spec.StorageType == StorageTypeLocal {
		return r.validateReaperWithLocalStorage()
	}
	if r.Spec.StorageType == StorageTypeCassandra {
		return r.validateReaperWithCassandraStorage()
	}
	return nil
}

func (r *Reaper) validateReaperWithLocalStorage() error {
	if r.Spec.StorageConfig == nil {
		return fmt.Errorf("must set StorageConfig when using local storage")
	}
	// skipping storage class name check because k8s uses a default one
	if r.Spec.StorageConfig.AccessModes == nil {
		return fmt.Errorf("must set StorageConfig.AccessModes when using local storage")
	}
	if r.Spec.StorageConfig.Resources.Requests.Storage().IsZero() {
		return fmt.Errorf("must set StorageConfig.Resources.Requests.Storage.Size when using local storage")
	}
	return nil
}

func (r *Reaper) validateReaperWithCassandraStorage() error {
	if r.Spec.HasReaperRef() {
		return fmt.Errorf("cannot set ReaperRef when using Cassandra storage")
	}
	return nil
}
