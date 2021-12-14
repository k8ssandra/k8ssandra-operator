package images

import (
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

const DefaultRegistry = "docker.io"

// ImageId is the set of components that uniquely identify a container image: registry, repository, name and tag.
type ImageId interface {
	GetRegistry() string
	GetRepository() string
	GetName() string
	GetTag() string
}

type imageId struct {
	registry   string
	repository string
	name       string
	tag        string
}

func (i *imageId) GetRegistry() string {
	return i.registry
}

func (i *imageId) GetRepository() string {
	return i.repository
}

func (i *imageId) GetName() string {
	return i.name
}

func (i *imageId) GetTag() string {
	return i.tag
}

// NewImageId returns a new ImageId for the given components. It does not check that the components are valid.
func NewImageId(registry, repository, name, tag string) ImageId {
	return &imageId{
		registry:   registry,
		repository: repository,
		name:       name,
		tag:        tag,
	}
}

// Image uniquely identifies a container image and specifies how to pull it from its remote registry.
type Image interface {
	ImageId
	GetPullPolicy() corev1.PullPolicy
	GetPullSecretRef() *corev1.LocalObjectReference
}

type image struct {
	ImageId
	pullPolicy    corev1.PullPolicy
	pullSecretRef *corev1.LocalObjectReference
}

func (i *image) GetPullPolicy() corev1.PullPolicy {
	return i.pullPolicy
}

func (i *image) GetPullSecretRef() *corev1.LocalObjectReference {
	return i.pullSecretRef
}

// NewImage returns a new Image for the given id and pull settings. It panics if id is nil. It does not check that the
// image components are valid.
func NewImage(id ImageId, pullPolicy corev1.PullPolicy, pullSecretRef *corev1.LocalObjectReference) Image {
	return &image{
		ImageId:       id,
		pullPolicy:    pullPolicy,
		pullSecretRef: pullSecretRef,
	}
}

// ImageString returns the string that uniquely identifies the given ImageId. It panics if id is nil. It does not check
// if the id components are valid.
func ImageString(i ImageId) string {
	return fmt.Sprintf("%v/%v/%v:%v", i.GetRegistry(), i.GetRepository(), i.GetName(), i.GetTag())
}

// Coalesce returns a new Image built from the given image, using the given defaults for components that were not
// explicitly provided.
// The pull policy is computed as follows: if the image specifies a pull policy, that policy is returned; otherwise, if
// the image tag is "latest", Always is returned; otherwise, the default pull policy is returned.
func Coalesce(image, defaults Image) Image {
	id := NewImageId(registry(image, defaults), repository(image, defaults), name(image, defaults), tag(image, defaults))
	return NewImage(id, pullPolicy(image, defaults), pullSecretRef(image, defaults))
}

// CollectPullSecrets returns a slice of secret references required to pull all the given images. The slice will be
// empty if none of the images requires a secret to be successfully pulled.
func CollectPullSecrets(images ...Image) []corev1.LocalObjectReference {
	var secrets []corev1.LocalObjectReference
	for _, image := range images {
		if image != nil && image.GetPullSecretRef() != nil {
			secrets = append(secrets, *image.GetPullSecretRef())
		}
	}
	return secrets
}

func registry(id, defaults ImageId) string {
	if !utils.IsNil(id) && id.GetRegistry() != "" {
		return id.GetRegistry()
	}
	return defaults.GetRegistry()
}

func repository(id, defaults ImageId) string {
	if !utils.IsNil(id) && id.GetRepository() != "" {
		return id.GetRepository()
	}
	return defaults.GetRepository()
}

func name(id, defaults ImageId) string {
	if !utils.IsNil(id) && id.GetName() != "" {
		return id.GetName()
	}
	return defaults.GetName()
}

func tag(id, defaults ImageId) string {
	if !utils.IsNil(id) && id.GetTag() != "" {
		return id.GetTag()
	}
	return defaults.GetTag()
}

func pullPolicy(image, defaults Image) corev1.PullPolicy {
	if !utils.IsNil(image) && image.GetPullPolicy() != "" {
		return image.GetPullPolicy()
	} else if tag(image, defaults) == "latest" {
		return corev1.PullAlways
	}
	return defaults.GetPullPolicy()
}

func pullSecretRef(image, defaults Image) *corev1.LocalObjectReference {
	if !utils.IsNil(image) && image.GetPullSecretRef() != nil {
		return image.GetPullSecretRef()
	}
	return defaults.GetPullSecretRef()
}
