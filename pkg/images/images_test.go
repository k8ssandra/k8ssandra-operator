package images

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

var (
	defaultId = NewImageId("default.registry.io", "default-repo", "default-name", "default-tag")
	latestId  = NewImageId("other.registry.io", "other-repo", "other-name", "latest")
	customId  = NewImageId("", "", "custom-name", "")
	mergedId  = NewImageId(defaultId.GetRegistry(), defaultId.GetRepository(), customId.GetName(), defaultId.GetTag())
	emptyId   = NewImageId("", "", "", "")
)

var (
	defaultImage = NewImage(defaultId, corev1.PullIfNotPresent, nil)
	latestImage  = NewImage(latestId, "", &corev1.LocalObjectReference{Name: "other-pull-secret"})
	customImage  = NewImage(customId, corev1.PullAlways, &corev1.LocalObjectReference{Name: "custom-pull-secret"})
	mergedImage  = NewImage(mergedId, customImage.GetPullPolicy(), customImage.GetPullSecretRef())
	emptyImage   = NewImage(emptyId, "", nil)
)

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name     string
		image    Image
		defaults Image
		expected Image
	}{
		{"nil", nil, defaultImage, defaultImage},
		{"empty", emptyImage, defaultImage, defaultImage},
		{"non-empty", customImage, defaultImage, mergedImage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Coalesce(tt.image, tt.defaults)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestImageString(t *testing.T) {
	tests := []struct {
		name     string
		image    Image
		expected string
	}{
		{"default", defaultImage, "default.registry.io/default-repo/default-name:default-tag"},
		{"latest", latestImage, "other.registry.io/other-repo/other-name:latest"},
		{"custom", customImage, "//custom-name:"},
		{"empty", emptyImage, "//:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ImageString(tt.image)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestCollectPullSecrets(t *testing.T) {
	tests := []struct {
		name     string
		image1   Image
		image2   Image
		expected []corev1.LocalObjectReference
	}{
		{"nil", nil, nil, nil},
		{"no secrets", defaultImage, defaultImage, nil},
		{"some secrets", latestImage, defaultImage, []corev1.LocalObjectReference{*latestImage.GetPullSecretRef()}},
		{"all secrets", latestImage, customImage, []corev1.LocalObjectReference{*latestImage.GetPullSecretRef(), *customImage.GetPullSecretRef()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CollectPullSecrets(tt.image1, tt.image2)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_registry(t *testing.T) {
	tests := []struct {
		name     string
		id       ImageId
		defaults ImageId
		expected string
	}{
		{"nil", nil, defaultId, defaultId.GetRegistry()},
		{"empty", emptyId, defaultId, defaultId.GetRegistry()},
		{"non-empty", latestId, defaultId, latestId.GetRegistry()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := registry(tt.id, tt.defaults)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_repository(t *testing.T) {
	tests := []struct {
		name     string
		id       ImageId
		defaults ImageId
		expected string
	}{
		{"nil", nil, defaultId, defaultId.GetRepository()},
		{"empty", emptyId, defaultId, defaultId.GetRepository()},
		{"non-empty", latestId, defaultId, latestId.GetRepository()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := repository(tt.id, tt.defaults)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_name(t *testing.T) {
	tests := []struct {
		name     string
		id       ImageId
		defaults ImageId
		expected string
	}{
		{"nil", nil, defaultId, defaultId.GetName()},
		{"empty", emptyId, defaultId, defaultId.GetName()},
		{"non-empty", latestId, defaultId, latestId.GetName()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := name(tt.id, tt.defaults)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_tag(t *testing.T) {
	tests := []struct {
		tag      string
		id       ImageId
		defaults ImageId
		expected string
	}{
		{"nil", nil, defaultId, defaultId.GetTag()},
		{"empty", emptyId, defaultId, defaultId.GetTag()},
		{"non-empty", latestId, defaultId, latestId.GetTag()},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			actual := tag(tt.id, tt.defaults)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func Test_pullPolicy(t *testing.T) {
	tests := []struct {
		pullPolicy string
		image      Image
		defaults   Image
		expected   corev1.PullPolicy
	}{
		{"nil", nil, defaultImage, defaultImage.GetPullPolicy()},
		{"empty", emptyImage, defaultImage, defaultImage.GetPullPolicy()},
		{"non-empty", customImage, defaultImage, customImage.GetPullPolicy()},
		{"latest", latestImage, defaultImage, corev1.PullAlways},
	}
	for _, tt := range tests {
		t.Run(tt.pullPolicy, func(t *testing.T) {
			actual := pullPolicy(tt.image, tt.defaults)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
