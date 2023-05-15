package secret

import (
	"encoding/json"
	"fmt"

	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
)

// e.g. k8ssandra.io/inject-secret: '[{ "name": "test-secret", "path": "/etc/test/test-secret", "containers": ["c1", "c2"]}]'
const (
	SecretInjectionAnnotation = "k8ssandra.io/inject-secret"
	credentialsMountPath      = "/etc/secrets"
)

type SecretInjection struct {
	SecretName string   `json:"name"`
	Path       string   `json:"path"`
	Containers []string `json:"containers,omitempty"`
}

func AddInjectionAnnotationCassandraContainers(t *meta.Tags, secretName string) error {
	return AddInjectionAnnotation(t, secretName, []string{"cassandra"})
}

func AddInjectionAnnotationMedusaContainers(t *meta.Tags, secretName string) error {
	return AddInjectionAnnotation(t, secretName, []string{"medusa", "medusa-restore"})
}

func AddInjectionAnnotationReaperContainers(t *meta.Tags, secretName string) error {
	return AddInjectionAnnotation(t, secretName, []string{"reaper", "reaper-schema-init"})
}

func AddInjectionAnnotation(t *meta.Tags, secretName string, containers []string) error {
	if t.Annotations == nil {
		t.Annotations = make(map[string]string)
	}

	var secrets []SecretInjection
	if val, ok := t.Annotations[SecretInjectionAnnotation]; ok {
		if err := json.Unmarshal([]byte(val), &secrets); err != nil {
			return err
		}
	}

	if isSecretIncluded(secrets, secretName) {
		return nil
	}

	secretsStr, err := addSecretToAnnotationString(secrets, secretName, fmt.Sprintf("%s/%s", credentialsMountPath, secretName), containers)
	if err != nil {
		return err
	}

	t.Annotations[SecretInjectionAnnotation] = string(secretsStr)
	return nil
}

// checks if a secret with the same name and path has already been included in
// the list of secrets to be injected
func isSecretIncluded(secrets []SecretInjection, secretName string) bool {
	for _, secret := range secrets {
		if secret.SecretName == secretName {
			return true
		}
	}
	return false
}

func addSecretToAnnotationString(secrets []SecretInjection, secretName string, path string, containers []string) (string, error) {
	secret := SecretInjection{
		SecretName: secretName,
		Path:       path,
		Containers: containers,
	}

	secrets = append(secrets, secret)

	secretsStr, err := json.Marshal(secrets)
	if err != nil {
		return "", err
	}

	return string(secretsStr), nil
}
