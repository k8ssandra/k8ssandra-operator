package images

import (
	"fmt"
	"github.com/imdario/mergo"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultRegistry          = "docker.io"
	DockerOfficialRepository = "library"
)

// Image uniquely describes a container image and also specifies how to pull it from its remote repository.
// More info: https://kubernetes.io/docs/concepts/containers/images.
// +kubebuilder:object:generate=true
type Image struct {

	// The Docker registry to use. Defaults to "docker.io", the official Docker Hub.
	// +kubebuilder:default="docker.io"
	// +optional
	Registry string `json:"registry,omitempty"`

	// The Docker repository to use.
	// +optional
	Repository string `json:"repository,omitempty"`

	// The image name to use.
	// +optional
	Name string `json:"name,omitempty"`

	// The image tag to use. Defaults to "latest".
	// +kubebuilder:default="latest"
	// +optional
	Tag string `json:"tag,omitempty"`

	// The image pull policy to use. Defaults to "Always" if the tag is "latest", otherwise to "IfNotPresent".
	// +optional
	// +kubebuilder:validation:Enum:=Always;IfNotPresent;Never
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// The secret to use when pulling the image from private repositories. If specified, this secret will be passed to
	// individual puller implementations for them to use. For example, in the case of Docker, only DockerConfig type
	// secrets are honored. More info:
	// https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	PullSecretRef *corev1.LocalObjectReference `json:"pullSecretRef,omitempty"`
}

// String returns this image's Docker name. It does not validate that the returned name is a valid Docker name.
func (in Image) String() string {
	return fmt.Sprintf("%v/%v/%v:%v", in.Registry, in.Repository, in.Name, in.Tag)
}

// Merge returns a new Image built by coalescing this image with the given image; defaults from the given image
// are used for components that were not explicitly provided in this image.
// The registry is computed as follows: if the image specifies a registry, that registry is returned; otherwise, if the
// default image specifies a registry, that registry is returned; otherwise, the default registry is returned.
// The tag is computed as follows: if the image specifies a tag, that tag is returned; otherwise, if the default image
// specifies a tag, that tag is returned; otherwise, "latest" returned.
// The pull policy is computed as follows: if the image specifies a pull policy, that policy is returned; otherwise, if
// the image tag is "latest", Always is returned; otherwise, the default pull policy is returned.
// Other components are computed as follows: if the image specifies a (non-empty) component, that component is returned;
// otherwise, the component from the default image is returned.
func (in *Image) Merge(i Image) *Image {
	merged := in.DeepCopy()
	if merged == nil {
		merged = &Image{}
	}
	_ = mergo.Merge(merged, i)
	if merged.Registry == "" {
		merged.Registry = DefaultRegistry
	}
	if merged.Tag == "" {
		merged.Tag = "latest"
	}
	if merged.PullPolicy == "" {
		if merged.Tag == "latest" {
			merged.PullPolicy = corev1.PullAlways
		} else {
			merged.PullPolicy = corev1.PullIfNotPresent
		}
	}
	return merged
}

// CollectPullSecrets returns a slice of secret references required to pull all the given images. The slice will be
// empty if none of the images requires a secret to be successfully pulled.
func CollectPullSecrets(images ...*Image) []corev1.LocalObjectReference {
	var secrets []corev1.LocalObjectReference
	var secretNames []string
	for _, image := range images {
		if image != nil && image.PullSecretRef != nil && !utils.SliceContains(secretNames, image.PullSecretRef.Name) {
			secrets = append(secrets, *image.PullSecretRef)
			secretNames = append(secretNames, image.PullSecretRef.Name)
		}
	}
	return secrets
}
