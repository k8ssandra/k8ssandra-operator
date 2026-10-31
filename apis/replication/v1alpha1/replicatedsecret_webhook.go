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

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func SetupReplicatedSecretWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &ReplicatedSecret{}).
		WithValidator(&replicatedSecretCustomValidator{
			Authorizer: &subjectAccessReviewer{Client: mgr.GetClient()},
			Client:     mgr.GetAPIReader(),
		}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-replication-k8ssandra-io-v1alpha1-replicatedsecret,mutating=false,failurePolicy=fail,sideEffects=None,groups=replication.k8ssandra.io,resources=replicatedsecrets,verbs=create;update;delete,versions=v1alpha1,name=vreplicatedsecret.kb.io,admissionReviewVersions=v1

var _ admission.Validator[*ReplicatedSecret] = &replicatedSecretCustomValidator{}

type replicationAuthorizer interface {
	Review(context.Context, authenticationv1.UserInfo, authorizationv1.ResourceAttributes) (bool, error)
}

type replicatedSecretCustomValidator struct {
	Authorizer replicationAuthorizer
	Client     client.Reader
}

func (v *replicatedSecretCustomValidator) ValidateCreate(ctx context.Context, obj *ReplicatedSecret) (admission.Warnings, error) {
	return nil, v.validate(ctx, obj, true)
}

func (v *replicatedSecretCustomValidator) ValidateUpdate(ctx context.Context, _ *ReplicatedSecret, newSecret *ReplicatedSecret) (admission.Warnings, error) {
	return nil, v.validate(ctx, newSecret, true)
}

func (v *replicatedSecretCustomValidator) ValidateDelete(ctx context.Context, obj *ReplicatedSecret) (admission.Warnings, error) {
	return nil, v.validate(ctx, obj, false)
}

func (v *replicatedSecretCustomValidator) validate(ctx context.Context, obj *ReplicatedSecret, validateSelection bool) error {
	if validateSelection {
		if err := ValidateReplicationSelection(obj); err != nil {
			return err
		}
	}

	request, err := admission.RequestFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to read admission request: %w", err)
	}
	if validateSelection {
		if err := v.authorize(
			ctx,
			request.UserInfo,
			authorizationv1.ResourceAttributes{
				Namespace: obj.Namespace,
				Verb:      "list",
				Resource:  "secrets",
			},
			fmt.Sprintf("list secrets in namespace %q", obj.Namespace),
		); err != nil {
			return err
		}
	}

	for _, target := range obj.Spec.ReplicationTargets {
		if target.K8sContextName == "" {
			targetNamespace := target.Namespace
			if targetNamespace == "" {
				targetNamespace = obj.Namespace
			}
			for _, verb := range []string{"get", "list", "create", "update", "delete"} {
				if err := v.authorize(
					ctx,
					request.UserInfo,
					authorizationv1.ResourceAttributes{
						Namespace: targetNamespace,
						Verb:      verb,
						Resource:  "secrets",
					},
					fmt.Sprintf("%s secrets in namespace %q", verb, targetNamespace),
				); err != nil {
					return err
				}
			}
			continue
		}

		clientConfig, err := v.clientConfigForContext(ctx, target.K8sContextName)
		if err != nil {
			return err
		}
		if err := v.authorize(
			ctx,
			request.UserInfo,
			authorizationv1.ResourceAttributes{
				Group:     configapi.GroupVersion.Group,
				Namespace: clientConfig.Namespace,
				Verb:      "use",
				Resource:  "clientconfigs",
				Name:      clientConfig.Name,
			},
			fmt.Sprintf("use ClientConfig %q", client.ObjectKeyFromObject(clientConfig)),
		); err != nil {
			return err
		}
	}

	return nil
}

func (v *replicatedSecretCustomValidator) authorize(
	ctx context.Context,
	user authenticationv1.UserInfo,
	attributes authorizationv1.ResourceAttributes,
	action string,
) error {
	allowed, err := v.Authorizer.Review(ctx, user, attributes)
	if err != nil {
		return fmt.Errorf("failed to authorize request to %s: %w", action, err)
	}
	if !allowed {
		return fmt.Errorf("not authorized to %s", action)
	}
	return nil
}

func (v *replicatedSecretCustomValidator) clientConfigForContext(ctx context.Context, contextName string) (*configapi.ClientConfig, error) {
	configs := &configapi.ClientConfigList{}
	if err := v.Client.List(ctx, configs); err != nil {
		return nil, fmt.Errorf("failed to resolve client context %q: %w", contextName, err)
	}
	for i := range configs.Items {
		if configs.Items[i].GetContextName() == contextName {
			return &configs.Items[i], nil
		}
	}
	return nil, fmt.Errorf("no ClientConfig defines context %q", contextName)
}

func ValidateReplicationSelection(replicatedSecret *ReplicatedSecret) error {
	selector := replicatedSecret.Spec.Selector
	if replicatedSecret.Spec.ReplicateAll {
		if selector != nil {
			return fmt.Errorf("replicateAll cannot be combined with selector")
		}
		return nil
	}

	if selectorIsEmpty(selector) {
		return fmt.Errorf("selector must be non-empty unless replicateAll is true")
	}

	if _, err := metav1.LabelSelectorAsSelector(selector); err != nil {
		return fmt.Errorf("invalid selector: %w", err)
	}
	return nil
}

func ReplicationSelector(replicatedSecret *ReplicatedSecret) (labels.Selector, error) {
	if err := ValidateReplicationSelection(replicatedSecret); err != nil {
		return nil, err
	}
	if replicatedSecret.Spec.ReplicateAll {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(replicatedSecret.Spec.Selector)
}

// LegacyCompatibleReplicationSelector preserves the historical empty-selector
// behavior for objects created before admission validation was introduced.
func LegacyCompatibleReplicationSelector(replicatedSecret *ReplicatedSecret) (labels.Selector, bool, error) {
	if !replicatedSecret.Spec.ReplicateAll && selectorIsEmpty(replicatedSecret.Spec.Selector) {
		return labels.Everything(), true, nil
	}
	selector, err := ReplicationSelector(replicatedSecret)
	return selector, false, err
}

func selectorIsEmpty(selector *metav1.LabelSelector) bool {
	return selector == nil || (len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 0)
}

type subjectAccessReviewer struct {
	Client client.Client
}

func (r *subjectAccessReviewer) Review(
	ctx context.Context,
	user authenticationv1.UserInfo,
	attributes authorizationv1.ResourceAttributes,
) (bool, error) {
	extra := make(map[string]authorizationv1.ExtraValue, len(user.Extra))
	for key, values := range user.Extra {
		extra[key] = authorizationv1.ExtraValue(values)
	}
	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			ResourceAttributes: &attributes,
			User:               user.Username,
			UID:                user.UID,
			Groups:             user.Groups,
			Extra:              extra,
		},
	}
	if err := r.Client.Create(ctx, review); err != nil {
		return false, err
	}
	return review.Status.Allowed, nil
}
