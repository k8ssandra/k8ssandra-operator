package goalesceutils

import (
	"github.com/adutra/goalesce"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
)

// Merge returns a new object built by merging the given objects, with the required options to
// make the merge work as expected for Kubernetes CRDs.
func Merge[T any](cluster, dc T) T {
	// We use MustDeepMerge here because errors cannot happen with the given options.
	return goalesce.MustDeepMerge(
		cluster, dc,
		// Trileans are required to correctly merge *bool fields with the expected semantic, that
		// is, if cluster has &true and dc has &false, we want &false.
		goalesce.WithTrileanMerge(),
		// resource.Quantity is a struct with unexported fields and should be copied and merged
		// atomically.
		goalesce.WithAtomicMerge(reflect.TypeOf(resource.Quantity{})),
		goalesce.WithAtomicCopy(reflect.TypeOf(resource.Quantity{})),
		// VolumeSource is a struct where only one of its fields can be set at a time, so we
		// cannot merge 2 volume sources together.
		goalesce.WithAtomicMerge(reflect.TypeOf(corev1.VolumeSource{})),
		// The following slices are merged with merge-by-id semantics by Kubernetes.
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.Container{}), "Name"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.ContainerPort{}), "ContainerPort"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.EnvVar{}), "Name"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.Volume{}), "Name"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.VolumeMount{}), "MountPath"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.VolumeDevice{}), "DevicePath"),
		// Also best merged with merge-by-id semantics.
		goalesce.WithSliceMergeByID(reflect.TypeOf([]cassdcapi.AdditionalVolumes{}), "Name"),
	)
}
