package images

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

var (
	defaultImage = Image{
		"",
		"default-repo",
		"default-name",
		"default-tag",
		"",
		nil,
	}
	latestImage = Image{
		"latest.registry.io",
		"latest-repo",
		"latest-name",
		"latest",
		"",
		&corev1.LocalObjectReference{Name: "latest-pull-secret"},
	}
	customImage = Image{
		"",
		"",
		"custom-name",
		"",
		corev1.PullAlways,
		&corev1.LocalObjectReference{Name: "custom-pull-secret"},
	}
	coalescedImage1 = Image{
		DefaultRegistry,
		"default-repo",
		"default-name",
		"default-tag",
		corev1.PullIfNotPresent,
		nil,
	}
	coalescedImage2 = Image{
		DefaultRegistry,
		defaultImage.Repository,
		customImage.Name,
		defaultImage.Tag,
		customImage.PullPolicy,
		customImage.PullSecretRef,
	}
	coalescedImage3 = Image{
		latestImage.Registry,
		latestImage.Repository,
		latestImage.Name,
		latestImage.Tag,
		corev1.PullAlways,
		latestImage.PullSecretRef,
	}
	emptyImage = Image{}
)

func TestImageString(t *testing.T) {
	tests := []struct {
		name     string
		image    Image
		expected string
	}{
		{"default", defaultImage, "/default-repo/default-name:default-tag"},
		{"latest", latestImage, "latest.registry.io/latest-repo/latest-name:latest"},
		{"custom", customImage, "//custom-name:"},
		{"empty", emptyImage, "//:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.image.String()
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestImageMerge(t *testing.T) {
	tests := []struct {
		name     string
		image    *Image
		defaults Image
		expected *Image
	}{
		{"nil", nil, defaultImage, &coalescedImage1},
		{"empty", &emptyImage, defaultImage, &coalescedImage1},
		{"non-empty", &customImage, defaultImage, &coalescedImage2},
		{"latest", &latestImage, defaultImage, &coalescedImage3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.image.Merge(tt.defaults)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestCollectPullSecrets(t *testing.T) {
	tests := []struct {
		name     string
		image1   *Image
		image2   *Image
		expected []corev1.LocalObjectReference
	}{
		{"nil", nil, nil, nil},
		{"no secrets", &defaultImage, &defaultImage, nil},
		{
			"some secrets",
			&latestImage,
			&defaultImage,
			[]corev1.LocalObjectReference{{Name: "latest-pull-secret"}},
		},
		{
			"all secrets",
			&latestImage,
			&customImage,
			[]corev1.LocalObjectReference{{Name: "latest-pull-secret"}, {Name: "custom-pull-secret"}},
		},
		{
			"duplicated secrets",
			&latestImage,
			&latestImage,
			[]corev1.LocalObjectReference{{Name: "latest-pull-secret"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CollectPullSecrets(tt.image1, tt.image2)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
