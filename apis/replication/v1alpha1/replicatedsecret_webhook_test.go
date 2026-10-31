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
	"testing"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestValidateReplicationSelection(t *testing.T) {
	tests := []struct {
		name         string
		selector     *metav1.LabelSelector
		replicateAll bool
		wantError    string
	}{
		{
			name:      "rejects omitted selector",
			wantError: "selector must be non-empty",
		},
		{
			name:         "rejects empty selector",
			selector:     &metav1.LabelSelector{},
			wantError:    "selector must be non-empty",
			replicateAll: false,
		},
		{
			name:         "accepts explicit replicate all",
			replicateAll: true,
		},
		{
			name: "accepts non-empty selector",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "cassandra"},
			},
		},
		{
			name: "rejects selector with replicate all",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "cassandra"},
			},
			replicateAll: true,
			wantError:    "replicateAll cannot be combined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicatedSecret := &ReplicatedSecret{
				Spec: ReplicatedSecretSpec{
					Selector:     tt.selector,
					ReplicateAll: tt.replicateAll,
				},
			}

			err := ValidateReplicationSelection(replicatedSecret)
			if tt.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantError)
			}
		})
	}
}

func TestReplicationSelectorMatchesEverythingOnlyWhenExplicit(t *testing.T) {
	replicatedSecret := &ReplicatedSecret{
		Spec: ReplicatedSecretSpec{ReplicateAll: true},
	}

	selector, err := ReplicationSelector(replicatedSecret)
	require.NoError(t, err)
	require.True(t, selector.Matches(labels.Set{"any": "label"}))
}

func TestLegacyCompatibleReplicationSelectorPreservesExistingEmptySelector(t *testing.T) {
	replicatedSecret := &ReplicatedSecret{}

	selector, legacy, err := LegacyCompatibleReplicationSelector(replicatedSecret)
	require.NoError(t, err)
	require.True(t, legacy)
	require.True(t, selector.Matches(labels.Set{"any": "label"}))
}

func TestReplicatedSecretValidatorRequiresSourceSecretAccess(t *testing.T) {
	validator := &replicatedSecretCustomValidator{
		Authorizer: &fakeReplicationAuthorizer{},
	}

	_, err := validator.ValidateCreate(admissionContext(), selectedReplicatedSecret())
	require.ErrorContains(t, err, "not authorized to list secrets")
}

func TestReplicatedSecretValidatorRequiresLocalTargetAccess(t *testing.T) {
	authorizer := &fakeReplicationAuthorizer{
		allow: func(attributes authorizationv1.ResourceAttributes) bool {
			return attributes.Namespace == "source" && attributes.Verb == "list"
		},
	}
	validator := &replicatedSecretCustomValidator{Authorizer: authorizer}
	replicatedSecret := selectedReplicatedSecret()
	replicatedSecret.Spec.ReplicationTargets = []ReplicationTarget{{Namespace: "target"}}

	_, err := validator.ValidateCreate(admissionContext(), replicatedSecret)
	require.ErrorContains(t, err, "not authorized to get secrets in namespace \"target\"")
}

func TestReplicatedSecretValidatorRequiresUseOfRemoteClientConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, configapi.AddToScheme(scheme))
	clientConfig := &configapi.ClientConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "remote-client", Namespace: "operator"},
		Spec:       configapi.ClientConfigSpec{ContextName: "remote"},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientConfig).Build()
	authorizer := &fakeReplicationAuthorizer{
		allow: func(attributes authorizationv1.ResourceAttributes) bool {
			return attributes.Resource == "secrets"
		},
	}
	validator := &replicatedSecretCustomValidator{
		Authorizer: authorizer,
		Client:     k8sClient,
	}
	replicatedSecret := selectedReplicatedSecret()
	replicatedSecret.Spec.ReplicationTargets = []ReplicationTarget{{K8sContextName: "remote"}}

	_, err := validator.ValidateCreate(admissionContext(), replicatedSecret)
	require.ErrorContains(t, err, `not authorized to use ClientConfig "operator/remote-client"`)
}

func TestReplicatedSecretValidatorAcceptsAuthorizedReplication(t *testing.T) {
	validator := &replicatedSecretCustomValidator{
		Authorizer: &fakeReplicationAuthorizer{
			allow: func(authorizationv1.ResourceAttributes) bool { return true },
		},
	}
	replicatedSecret := selectedReplicatedSecret()
	replicatedSecret.Spec.ReplicationTargets = []ReplicationTarget{{Namespace: "target"}}

	_, err := validator.ValidateCreate(admissionContext(), replicatedSecret)
	require.NoError(t, err)
}

type fakeReplicationAuthorizer struct {
	allow func(authorizationv1.ResourceAttributes) bool
}

func (f *fakeReplicationAuthorizer) Review(
	_ context.Context,
	_ authenticationv1.UserInfo,
	attributes authorizationv1.ResourceAttributes,
) (bool, error) {
	if f.allow == nil {
		return false, nil
	}
	return f.allow(attributes), nil
}

func admissionContext() context.Context {
	request := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{
				Username: "tenant",
				Groups:   []string{"developers"},
			},
		},
	}
	return admission.NewContextWithRequest(context.Background(), request)
}

func selectedReplicatedSecret() *ReplicatedSecret {
	return &ReplicatedSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "replication", Namespace: "source"},
		Spec: ReplicatedSecretSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "cassandra"},
			},
		},
	}
}
