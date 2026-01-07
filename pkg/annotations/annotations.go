package annotations

import (
	"github.com/adutra/goalesce"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

type Annotated interface {
	GetAnnotations() map[string]string
	SetAnnotations(annotations map[string]string)
}

func AddAnnotation(obj Annotated, annotationKey string, annotationValue string) {
	m := obj.GetAnnotations()
	if m == nil {
		m = map[string]string{}
	}
	m[annotationKey] = annotationValue
	obj.SetAnnotations(m)
}

func GetAnnotation(component Annotated, annotationKey string) string {
	m := component.GetAnnotations()
	return m[annotationKey]
}

func HasAnnotationWithValue(component Annotated, annotationKey string, annotationValue string) bool {
	return GetAnnotation(component, annotationKey) == annotationValue
}

func CompareAnnotations(r1, r2 Annotated, annotationKey string) bool {
	annotationValue := GetAnnotation(r1, annotationKey)
	if annotationValue == "" {
		return false
	}
	return HasAnnotationWithValue(r2, annotationKey, annotationValue)
}

func AddHashAnnotation(obj Annotated) {
	h := utils.DeepHashString(obj)
	AddAnnotation(obj, k8ssandraapi.ResourceHashAnnotation, h)
}

func CompareHashAnnotations(r1, r2 Annotated) bool {
	return CompareAnnotations(r1, r2, k8ssandraapi.ResourceHashAnnotation)
}

func AddCommonAnnotations(component Annotated, k8c *k8ssandraapi.K8ssandraCluster) {
	if k8c.Spec.Cassandra != nil && k8c.Spec.Cassandra.Meta.CommonAnnotations != nil {
		component.SetAnnotations(goalesce.MustDeepMerge(k8c.Spec.Cassandra.Meta.CommonAnnotations, component.GetAnnotations()))
	}
}

func AddCommonAnnotationsFromReaper(component Annotated, reaper *reaperapi.Reaper) {
	if reaper.Spec.ResourceMeta != nil && reaper.Spec.ResourceMeta.CommonAnnotations != nil {
		component.SetAnnotations(goalesce.MustDeepMerge(reaper.Spec.ResourceMeta.CommonAnnotations, component.GetAnnotations()))
	}
}
