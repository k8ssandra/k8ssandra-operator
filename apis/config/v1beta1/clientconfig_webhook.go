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

package v1beta1

import (
	"context"
	"fmt"

	"github.com/k8ssandra/k8ssandra-operator/pkg/kubeconfigvalidator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func SetupClientConfigWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ClientConfig{}).
		WithValidator(&clientConfigCustomValidator{
			ConfigReader: mgr.GetClient(),
			SecretReader: mgr.GetAPIReader(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-config-k8ssandra-io-v1beta1-clientconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=config.k8ssandra.io,resources=clientconfigs,verbs=create;update,versions=v1beta1,name=vclientconfig.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*ClientConfig] = &clientConfigCustomValidator{}

type clientConfigCustomValidator struct {
	ConfigReader client.Reader
	SecretReader client.Reader
}

func (v *clientConfigCustomValidator) ValidateCreate(ctx context.Context, obj *ClientConfig) (admission.Warnings, error) {
	return nil, v.validate(ctx, obj)
}

func (v *clientConfigCustomValidator) ValidateUpdate(ctx context.Context, _ *ClientConfig, newConfig *ClientConfig) (admission.Warnings, error) {
	return nil, v.validate(ctx, newConfig)
}

func (v *clientConfigCustomValidator) ValidateDelete(context.Context, *ClientConfig) (admission.Warnings, error) {
	return nil, nil
}

func (v *clientConfigCustomValidator) validate(ctx context.Context, obj *ClientConfig) error {
	if obj.Spec.KubeConfigSecret.Name == "" {
		return fmt.Errorf("kubeConfigSecret.name must not be empty")
	}

	configs := &ClientConfigList{}
	if err := v.ConfigReader.List(ctx, configs); err != nil {
		return fmt.Errorf("failed to check ClientConfig context uniqueness: %w", err)
	}
	for i := range configs.Items {
		existing := &configs.Items[i]
		if existing.Namespace == obj.Namespace && existing.Name == obj.Name {
			continue
		}
		if existing.GetContextName() == obj.GetContextName() {
			return fmt.Errorf(
				"context name %q is already used by ClientConfig %s/%s",
				obj.GetContextName(),
				existing.Namespace,
				existing.Name,
			)
		}
	}

	secretKey := types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Spec.KubeConfigSecret.Name,
	}
	secret := &corev1.Secret{}
	if err := v.SecretReader.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to read kubeconfig Secret %s: %w", secretKey, err)
	}
	data, found := secret.Data["kubeconfig"]
	if !found {
		return fmt.Errorf("Secret %s is missing required kubeconfig property", secretKey)
	}

	config, err := clientcmd.Load(data)
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig in Secret %s: %w", secretKey, err)
	}
	if err := kubeconfigvalidator.ValidateConfig(config); err != nil {
		return fmt.Errorf("unsafe kubeconfig in Secret %s: %w", secretKey, err)
	}
	if _, found := config.Contexts[obj.GetContextName()]; !found {
		return fmt.Errorf("kubeconfig in Secret %s does not contain context %q", secretKey, obj.GetContextName())
	}

	return nil
}
