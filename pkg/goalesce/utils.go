package goalesceutils

import (
	"github.com/adutra/goalesce"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
)

// MergeCRDs returns a new object built by merging the given objects, with the required options to
// make the merge work as expected for Kubernetes CRDs.
func MergeCRDs[T any](cluster, dc T) T {
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
		// Types where only one of the fields can be set at a time, so we cannot merge them
		// together.
		goalesce.WithAtomicMerge(reflect.TypeOf(corev1.VolumeSource{})),
		goalesce.WithAtomicMerge(reflect.TypeOf(corev1.EnvVarSource{})),
		// The following slices are merged with merge-by-id semantics by Kubernetes.
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.Container{}), "Name"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.ContainerPort{}), "ContainerPort"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.EnvVar{}), "Name"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.Volume{}), "Name"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.VolumeMount{}), "MountPath"),
		goalesce.WithSliceMergeByID(reflect.TypeOf([]corev1.VolumeDevice{}), "DevicePath"),
		// Also best merged with merge-by-id semantics.
		goalesce.WithSliceMergeByID(reflect.TypeOf([]cassdcapi.AdditionalVolumes{}), "Name"),
		// EnvVar cannot have both Value and ValueFrom set at the same time, so we use a custom
		// merger.
		goalesce.WithTypeMergerProvider(reflect.TypeOf(corev1.EnvVar{}),
			func(merger goalesce.DeepMergeFunc, copier goalesce.DeepCopyFunc) goalesce.DeepMergeFunc {
				return func(v1, v2 reflect.Value) (reflect.Value, error) {
					if v1.IsZero() {
						return copier(v2)
					} else if v2.IsZero() {
						return copier(v1)
					}
					merged := reflect.New(v1.Type()).Elem()
					name, _ := merger(v1.FieldByName("Name"), v2.FieldByName("Name"))
					merged.FieldByName("Name").Set(name)
					value, _ := merger(v1.FieldByName("Value"), v2.FieldByName("Value"))
					merged.FieldByName("Value").Set(value)
					valueFrom, _ := merger(v1.FieldByName("ValueFrom"), v2.FieldByName("ValueFrom"))
					merged.FieldByName("ValueFrom").Set(valueFrom)
					if !value.IsZero() && !valueFrom.IsZero() {
						if v2.FieldByName("Value").IsZero() {
							merged.FieldByName("Value").Set(reflect.Zero(value.Type()))
						} else {
							merged.FieldByName("ValueFrom").Set(reflect.Zero(valueFrom.Type()))
						}
					}
					return merged, nil
				}
			}),
	)
}
