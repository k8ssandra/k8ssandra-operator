package secret

import (
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddInjectionAnnotationEmptyTags(t *testing.T) {
	tags := meta.Tags{}

	err := AddInjectionAnnotation(&tags, "test-secret")
	require.NoError(t, err)

	val, ok := tags.Annotations[SecretInjectionAnnotation]
	assert.True(t, ok)

	expected := `[{"secretName":"test-secret","path":"/etc/secrets/test-secret"}]`
	assert.Equal(t, expected, val)
}

func TestAddInjectionAnnotationNonEmptyTags(t *testing.T) {
	tags := meta.Tags{
		Annotations: map[string]string{
			"non-secret-key": "non-secret-value",
		},
	}

	err := AddInjectionAnnotation(&tags, "test-secret")
	require.NoError(t, err)

	val, ok := tags.Annotations[SecretInjectionAnnotation]
	assert.True(t, ok)

	expected := `[{"secretName":"test-secret","path":"/etc/secrets/test-secret"}]`
	assert.Equal(t, expected, val)

	val, ok = tags.Annotations["non-secret-key"]
	assert.True(t, ok)
	assert.Equal(t, val, "non-secret-value")
}

func TestAddInjectionAnnotationWithInjectionAnnotationPresent(t *testing.T) {
	tags := meta.Tags{
		Annotations: map[string]string{
			SecretInjectionAnnotation: `[{"secretName":"test-secret","path":"/etc/secrets/test-secret"}]`,
		},
	}

	err := AddInjectionAnnotation(&tags, "other-secret")
	require.NoError(t, err)

	expected := `[{"secretName":"test-secret","path":"/etc/secrets/test-secret"},{"secretName":"other-secret","path":"/etc/secrets/other-secret"}]`
	val, ok := tags.Annotations[SecretInjectionAnnotation]
	assert.True(t, ok)
	assert.Equal(t, expected, val)
}

func TestAddInjectionAnnotationDuplicate(t *testing.T) {
	tags := meta.Tags{
		Annotations: map[string]string{
			SecretInjectionAnnotation: `[{"secretName":"test-secret","path":"/etc/secrets/test-secret"}]`,
		},
	}

	err := AddInjectionAnnotation(&tags, "test-secret")
	require.NoError(t, err)

	expected := `[{"secretName":"test-secret","path":"/etc/secrets/test-secret"}]`
	val, ok := tags.Annotations[SecretInjectionAnnotation]
	assert.True(t, ok)
	assert.Equal(t, expected, val)
}

func TestAddInjectionAnnotationNonJson(t *testing.T) {
	tags := meta.Tags{
		Annotations: map[string]string{
			SecretInjectionAnnotation: `non-json-string-specified`,
		},
	}

	err := AddInjectionAnnotation(&tags, "test-secret")
	require.Error(t, err)
}

func TestIsSecretIncluded(t *testing.T) {
	secrets := []SecretInjection{
		SecretInjection{SecretName: "test", Path: "/etc/secrets"},
		SecretInjection{SecretName: "other", Path: "/etc/secrets"},
	}

	assert.True(t, isSecretIncluded(secrets, "test"))
	assert.True(t, isSecretIncluded(secrets, "other"))
	assert.False(t, isSecretIncluded(secrets, "nothing"))

}

func TestIsSecretIncludedNilSlice(t *testing.T) {
	var secrets []SecretInjection
	assert.False(t, isSecretIncluded(secrets, "test"))
}
