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

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Validate rejects kubeconfig features that can execute processes, read files
// from the operator container, proxy traffic, or disable TLS verification.
func Validate(data []byte) error {
	config, err := clientcmd.Load(data)
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	return ValidateConfig(config)
}

// ValidateConfig validates an already parsed kubeconfig.
func ValidateConfig(config *clientcmdapi.Config) error {
	for name, authInfo := range config.AuthInfos {
		switch {
		case authInfo.Exec != nil:
			return fmt.Errorf("user %q uses exec authentication", name)
		case authInfo.AuthProvider != nil:
			return fmt.Errorf("user %q uses auth-provider authentication", name)
		case authInfo.TokenFile != "":
			return fmt.Errorf("user %q uses a token file", name)
		case authInfo.ClientCertificate != "" || authInfo.ClientKey != "":
			return fmt.Errorf("user %q uses a client certificate or key file", name)
		}
	}

	for name, cluster := range config.Clusters {
		switch {
		case cluster.CertificateAuthority != "":
			return fmt.Errorf("cluster %q uses a certificate authority file", name)
		case cluster.ProxyURL != "":
			return fmt.Errorf("cluster %q uses a proxy URL", name)
		case cluster.InsecureSkipTLSVerify:
			return fmt.Errorf("cluster %q uses insecure TLS", name)
		}
	}

	return nil
}
