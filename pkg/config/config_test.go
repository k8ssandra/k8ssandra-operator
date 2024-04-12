package config

import (
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestRegistryParsing(t *testing.T) {
	assert := assert.New(t)
	assert.Equal("docker.io", images.DefaultRegistry)
	assert.Equal(0, len(images.DefaultPullSecretOverride))
	assert.NotNil(images.DefaultPullSecretOverride)

	t.Setenv(DefaultRegistryEnvVar, "localhost:5001")
	InitConfig()
	assert.Equal("localhost:5001", images.DefaultRegistry)
	assert.NotNil(images.DefaultPullSecretOverride)

	images.DefaultRegistry = "docker.io"
	t.Setenv(DefaultPullSecretEnvVar, "my-super-secret")
	InitConfig()
	assert.Equal("localhost:5001", images.DefaultRegistry)
	assert.Equal(1, len(images.DefaultPullSecretOverride))
	assert.Equal("my-super-secret", images.DefaultPullSecretOverride[0].Name)

	images.DefaultPullSecretOverride = make([]corev1.LocalObjectReference, 0)
	t.Setenv(DefaultPullSecretEnvVar, "my-super-secret,secondary-secret")
	InitConfig()
	assert.Equal("localhost:5001", images.DefaultRegistry)
	assert.Equal(2, len(images.DefaultPullSecretOverride))
	assert.Equal("my-super-secret", images.DefaultPullSecretOverride[0].Name)
	assert.Equal("secondary-secret", images.DefaultPullSecretOverride[1].Name)
}
