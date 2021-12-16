package annotations

import (
	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
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
	AddAnnotation(obj, v1alpha1.ResourceHashAnnotation, h)
}

func CompareHashAnnotations(r1, r2 Annotated) bool {
	return CompareAnnotations(r1, r2, v1alpha1.ResourceHashAnnotation)
}
