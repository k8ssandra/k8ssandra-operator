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

package clientcache

import (
	"testing"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetRestConfigRejectsUnsafeKubeconfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "remote", Namespace: "operator"},
		Data: map[string][]byte{
			"kubeconfig": []byte(`apiVersion: v1
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
`),
		},
	}
	localClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	cache := New(localClient, localClient, scheme)
	clientConfig := &configapi.ClientConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "remote", Namespace: "operator"},
		Spec: configapi.ClientConfigSpec{
			KubeConfigSecret: corev1.LocalObjectReference{Name: secret.Name},
		},
	}

	_, err := cache.GetRestConfig(clientConfig)
	require.ErrorContains(t, err, "exec authentication")
}

func TestAddClientRejectsDuplicateContextName(t *testing.T) {
	scheme := runtime.NewScheme()
	first := fake.NewClientBuilder().WithScheme(scheme).Build()
	second := fake.NewClientBuilder().WithScheme(scheme).Build()
	cache := New(first, first, scheme)

	require.NoError(t, cache.AddClient("remote", first))
	require.ErrorContains(t, cache.AddClient("remote", second), "already registered")
	require.Same(t, first, cache.GetRemoteClients()["remote"])
}

func TestCreateRemoteClientsRejectsUnsafeKubeconfig(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "remote", Namespace: "operator"},
		Data: map[string][]byte{
			"kubeconfig": []byte(`apiVersion: v1
kind: Config
clusters:
- name: remote
  cluster:
    server: https://remote.example.com
    insecure-skip-tls-verify: true
contexts:
- name: remote
  context:
    cluster: remote
    user: remote
users:
- name: remote
  user:
    token: token
`),
		},
	}
	localClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	cache := New(localClient, localClient, scheme)

	err := cache.CreateRemoteClientsFromSecret(
		client.ObjectKeyFromObject(secret),
	)
	require.ErrorContains(t, err, "insecure TLS")
}
