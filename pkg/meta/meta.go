package meta

import "github.com/k8ssandra/k8ssandra-operator/pkg/utils"

// +kubebuilder:object:generate=true
type MetaTags struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// merge MetaTags into a destination MetaTags. If properties are shared between the two configs,
// the destination MetaTags property will be used
func MergeMetaTags(m1 *MetaTags, m2 *MetaTags) *MetaTags {
	if m1 == nil && m2 == nil {
		return nil
	}

	var l1, l2 map[string]string
	var a1, a2 map[string]string
	if m1 != nil {
		l1, a1 = m1.Labels, m1.Annotations
	}

	if m2 != nil {
		l2, a2 = m2.Labels, m2.Annotations
	}

	return &MetaTags{
		Labels:      utils.MergeMap(l1, l2),
		Annotations: utils.MergeMap(a1, a2),
	}
}

// Struct to hold labels and annotations for a resource
// +kubebuilder:object:generate=true
type ResourceMeta struct {
	// labels/annotations for the top-level CRD component
	// +optional
	Resource *MetaTags `json:"resource,omitempty"`

	// labels/annotations that will be applied to all components
	// created by the CRD
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`

	// labels/annotations for the pod components
	// +optional
	Pods *MetaTags `json:"pods,omitempty"`

	// labels/annotations for the service component
	Service *MetaTags `json:"service,omitempty"`
}
