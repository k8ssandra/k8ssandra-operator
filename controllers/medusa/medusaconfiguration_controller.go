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

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
)

const (
	MedusaStorageSecretIdentifierLabel = "k8ssandra.io/medusa-storage-secret"
)

// MedusaConfigurationReconciler reconciles a MedusaConfiguration object
type MedusaConfigurationReconciler struct {
	*config.ReconcilerConfig
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusaconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusaconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=medusa.k8ssandra.io,namespace="k8ssandra",resources=medusaconfigurations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MedusaConfiguration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *MedusaConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("medusabackupjob", req.NamespacedName)

	logger.Info("Starting reconciliation")

	// Fetch the MedusaConfiguration instance
	instance := &medusav1alpha1.MedusaConfiguration{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		logger.Error(err, "Failed to get MedusaConfiguration")
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	configuration := instance.DeepCopy()
	patch := client.MergeFrom(configuration.DeepCopy())

	// Check if the referenced secret exists
	if configuration.Spec.StorageProperties.StorageSecretRef.Name != "" {
		secret, err := r.GetSecret(ctx, r.Client, req, configuration.Spec.StorageProperties.StorageSecretRef.Name)
		if err != nil {
			logger.Error(err, "Failed to get MedusaConfiguration referenced secret")
			configuration.Status.SetCondition(medusav1alpha1.ControlStatusSecretAvailable, metav1.ConditionFalse)
			configuration.Status.SetConditionMessage(medusav1alpha1.ControlStatusSecretAvailable, err.Error())
			r.patchStatus(ctx, configuration, patch, logger)
			return ctrl.Result{}, err
		} else {
			configuration.Status.SetCondition(medusav1alpha1.ControlStatusSecretAvailable, metav1.ConditionTrue)
			if secret.Labels == nil {
				secret.Labels = make(map[string]string)
			}
			secret.Labels[MedusaStorageSecretIdentifierLabel] = utils.HashNameNamespace(secret.Name, secret.Namespace)
			recRes := reconciliation.ReconcileObject(ctx, r.Client, r.DefaultDelay, secret)
			switch {
			case recRes.IsError():
				return ctrl.Result{}, err
			case recRes.IsRequeue():
				return ctrl.Result{RequeueAfter: r.DefaultDelay}, err
			}
		}
	}

	configuration.Status.SetCondition(medusav1alpha1.ControlStatusReady, metav1.ConditionTrue)
	r.patchStatus(ctx, configuration, patch, logger)
	logger.Info("MedusaConfiguration Reconciliation complete", "MedusaConfiguration", req.NamespacedName)

	return ctrl.Result{}, nil
}

func (r *MedusaConfigurationReconciler) patchStatus(ctx context.Context, configuration *medusav1alpha1.MedusaConfiguration, patch client.Patch, logger logr.Logger) {
	if patchErr := r.Status().Patch(ctx, configuration, patch); patchErr != nil {
		logger.Error(patchErr, "failed to update MedusaConfiguration status")
	} else {
		logger.Info("updated MedusaConfiguration status")
	}
}

func (r *MedusaConfigurationReconciler) GetSecret(ctx context.Context, client client.Client, req ctrl.Request, secretName string) (corev1.Secret, error) {
	// Get the referenced secret to check if it exists
	secret := &corev1.Secret{}
	secretNamespacedName := types.NamespacedName{Namespace: req.Namespace, Name: secretName}
	err := client.Get(ctx, secretNamespacedName, secret)
	if err != nil {
		return corev1.Secret{}, err
	}

	return *secret, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *MedusaConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&medusav1alpha1.MedusaConfiguration{}).
		Complete(r)
}
