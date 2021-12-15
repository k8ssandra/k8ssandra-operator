package utils

const (
	ResourceHashAnnotation = "k8ssandra.io/resource-hash"
	
	// SystemReplicationAnnotation provides the initial replication of system keyspaces
	// (system_auth, system_distributed, system_traces) encoded as JSON. This annotation
	// is set on a K8ssandraCluster when it is first created. The value does not change
	// regardless of whether the replication of the system keyspaces changes.
	SystemReplicationAnnotation = "k8ssandra.io/system-replication"
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
