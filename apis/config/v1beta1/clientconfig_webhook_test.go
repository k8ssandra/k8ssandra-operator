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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClientConfigValidatorRejectsUnsafeKubeconfig(t *testing.T) {
	scheme := clientConfigTestScheme(t)
	secret := clientConfigSecret("remote-config", unsafeExecKubeconfig)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	validator := &clientConfigCustomValidator{
		ConfigReader: k8sClient,
		SecretReader: k8sClient,
	}

	_, err := validator.ValidateCreate(context.Background(), clientConfig("remote", "remote-config", ""))
	require.ErrorContains(t, err, "exec authentication")
}

func TestClientConfigValidatorRejectsDuplicateEffectiveContext(t *testing.T) {
	scheme := clientConfigTestScheme(t)
	firstSecret := clientConfigSecret("first-config", safeKubeconfig)
	secondSecret := clientConfigSecret("second-config", safeKubeconfig)
	existing := clientConfig("first", "first-config", "shared")
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(firstSecret, secondSecret, existing).Build()
	validator := &clientConfigCustomValidator{
		ConfigReader: k8sClient,
		SecretReader: k8sClient,
	}

	_, err := validator.ValidateCreate(context.Background(), clientConfig("second", "second-config", "shared"))
	require.ErrorContains(t, err, `context name "shared" is already used`)
}

func TestClientConfigValidatorAcceptsSafeUniqueContext(t *testing.T) {
	scheme := clientConfigTestScheme(t)
	secret := clientConfigSecret("remote-config", safeKubeconfig)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	validator := &clientConfigCustomValidator{
		ConfigReader: k8sClient,
		SecretReader: k8sClient,
	}

	_, err := validator.ValidateCreate(context.Background(), clientConfig("remote", "remote-config", ""))
	require.NoError(t, err)
}

func clientConfigTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, AddToScheme(scheme))
	return scheme
}

func clientConfigSecret(name string, kubeconfig []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "operator"},
		Data:       map[string][]byte{"kubeconfig": kubeconfig},
	}
}

func clientConfig(name, secretName, contextName string) *ClientConfig {
	return &ClientConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "operator"},
		Spec: ClientConfigSpec{
			ContextName: contextName,
			KubeConfigSecret: corev1.LocalObjectReference{
				Name: secretName,
			},
		},
	}
}

var safeKubeconfig = []byte(`apiVersion: v1
kind: Config
clusters:
- name: remote
  cluster:
    server: https://remote.example.com
    certificate-authority-data: Y2E=
contexts:
- name: remote
  context:
    cluster: remote
    user: remote
users:
- name: remote
  user:
    token: token
`)

var unsafeExecKubeconfig = []byte(`apiVersion: v1
kind: Config
clusters:
- name: remote
  cluster:
    server: https://remote.example.com
    certificate-authority-data: Y2E=
contexts:
- name: remote
  context:
    cluster: remote
    user: remote
users:
- name: remote
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: /bin/sh
      interactiveMode: Never
`)
