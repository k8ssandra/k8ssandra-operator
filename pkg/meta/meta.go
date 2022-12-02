package meta

// +kubebuilder:object:generate=true
type MetaTags struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Struct to hold labels and annotations for a the core componenets of
// an application on kubernetes
// +kubebuilder:object:generate=true
type ResourceMeta struct {
	// metaTags for the orchestrator component (e.g. Statefulet, Deployment)
	OrchestrationTags *MetaTags `json:"orchestrationTags,omitempty"`
	// metaTags for the child components (Pods)
	ChildTags *MetaTags `json:"childTags,omitempty"`
	// metaTags for the Service component
	ServiceTags *MetaTags `json:"serviceTags,omitempty"`
}
