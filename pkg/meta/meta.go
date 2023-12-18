package meta

import cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"

// +kubebuilder:object:generate=true
type Tags struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Struct to hold labels and annotations for a resource
// +kubebuilder:object:generate=true
type ResourceMeta struct {
	// labels/annotations for the top-level CRD component
	// +optional
	Tags `json:",inline"`

	// labels/annotations that will be applied to all components
	// created by the CRD
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`

	// labels/annotations for the pod components
	// +optional
	Pods Tags `json:"pods,omitempty"`

	// labels/annotations for the service component
	Service Tags `json:"service,omitempty"`
}

// Struct to hold labels and annotations for the top-level Cassandra cluster definition.
// +kubebuilder:object:generate=true
type CassandraClusterMeta struct {
	// labels/annotations for the CassandraDatacenter component
	// +optional
	Tags `json:",inline"`

	// labels that will be applied to all components
	// created by the CRD
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`

	// annotations that will be applied to all components
	// created by the CRD
	// +optional
	CommonAnnotations map[string]string `json:"commonAnnotations,omitempty"`

	// labels/annotations for the pod components
	// +optional
	Pods Tags `json:"pods,omitempty"`

	// labels/annotations for all of the CassandraDatacenter service components
	ServiceConfig CassandraDatacenterServicesMeta `json:"services,omitempty"`
}

// CassandraDatacenterServicesMeta is very similar to cassdcapi.ServiceConfig and is passed to
// cass-operator in the AdditionalServiceConfig field of the CassandraDatacenter spec.
// +kubebuilder:object:generate=true
type CassandraDatacenterServicesMeta struct {
	DatacenterService     Tags `json:"dcService,omitempty"`
	SeedService           Tags `json:"seedService,omitempty"`
	AllPodsService        Tags `json:"allPodsService,omitempty"`
	AdditionalSeedService Tags `json:"additionalSeedService,omitempty"`
	NodePortService       Tags `json:"nodePortService,omitempty"`
}

func (in CassandraDatacenterServicesMeta) ToCassAdditionalServiceConfig() cassdcapi.ServiceConfig {
	return cassdcapi.ServiceConfig{
		DatacenterService: cassdcapi.ServiceConfigAdditions{
			Annotations: in.DatacenterService.Annotations,
			Labels:      in.DatacenterService.Labels,
		},
		SeedService: cassdcapi.ServiceConfigAdditions{
			Annotations: in.SeedService.Annotations,
			Labels:      in.SeedService.Labels,
		},
		AdditionalSeedService: cassdcapi.ServiceConfigAdditions{
			Annotations: in.AdditionalSeedService.Annotations,
			Labels:      in.AdditionalSeedService.Labels,
		},
		AllPodsService: cassdcapi.ServiceConfigAdditions{
			Annotations: in.AllPodsService.Annotations,
			Labels:      in.AllPodsService.Labels,
		},
		NodePortService: cassdcapi.ServiceConfigAdditions{
			Annotations: in.NodePortService.Annotations,
			Labels:      in.NodePortService.Labels,
		},
	}
}
