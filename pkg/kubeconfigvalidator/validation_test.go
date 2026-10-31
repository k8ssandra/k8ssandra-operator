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

package kubeconfigvalidator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRejectsUnsafeAuthenticationAndTransportFeatures(t *testing.T) {
	tests := []struct {
		name        string
		cluster     string
		user        string
		errorString string
	}{
		{
			name:        "exec plugin",
			user:        "exec:\n      apiVersion: client.authentication.k8s.io/v1\n      command: /bin/sh\n      interactiveMode: Never",
			errorString: "exec authentication",
		},
		{
			name:        "auth provider",
			user:        "auth-provider:\n      name: oidc",
			errorString: "auth-provider authentication",
		},
		{
			name:        "token file",
			user:        "tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token",
			errorString: "token file",
		},
		{
			name:        "client certificate file",
			user:        "client-certificate: /tmp/client.crt\n    client-key: /tmp/client.key",
			errorString: "client certificate or key file",
		},
		{
			name:        "certificate authority file",
			cluster:     "certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
			errorString: "certificate authority file",
		},
		{
			name:        "proxy URL",
			cluster:     "proxy-url: http://proxy.example.com",
			errorString: "proxy URL",
		},
		{
			name:        "insecure TLS",
			cluster:     "insecure-skip-tls-verify: true",
			errorString: "insecure TLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(testKubeconfig(tt.cluster, tt.user))
			require.ErrorContains(t, err, tt.errorString)
		})
	}
}

func TestValidateAcceptsEmbeddedCredentials(t *testing.T) {
	kubeconfig := testKubeconfig(
		"certificate-authority-data: Y2E=",
		"client-certificate-data: Y2VydA==\n    client-key-data: a2V5\n    token: token",
	)

	require.NoError(t, Validate(kubeconfig))
}

func TestValidateRejectsMalformedKubeconfig(t *testing.T) {
	require.Error(t, Validate([]byte("not: [valid")))
}

func testKubeconfig(cluster, user string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: remote
  cluster:
    server: https://remote.example.com
    %s
contexts:
- name: remote
  context:
    cluster: remote
    user: remote
current-context: remote
users:
- name: remote
  user:
    %s
`, cluster, user))
}
