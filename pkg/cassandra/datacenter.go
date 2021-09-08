package cassandra

import (
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// SystemReplication represents the replication factor of the system_auth, system_traces,
// and system_distributed keyspsces. This is applied to each datacenter. The replication
// should be configured per DC, but that is currently not supported. See
// https://github.com/k8ssandra/management-api-for-apache-cassandra/issues/124 and
// https://github.com/k8ssandra/k8ssandra-operator/issues/91 for details.
// Note that when we can configure the replication per DC, this can be changed to a
// map[string]int.
type SystemReplication struct {
	Datacenters       []string
	ReplicationFactor int
}

// DatacenterConfig provides the configuration to be applied to the CassandraDatacenter.
// A DatacenterConfig is essentially a coalescence of an api.CassandraClusterTemplate and
// an api.CassandraDatacenterTemplate. There are global, cluster-wide settings that need
// to be specified at the DC-level. Using a DatacenterConfig allows to keep the api types
// clean such that cluster-level settings won't leak into the dc-level settings.
type DatacenterConfig struct {
	Meta              api.EmbeddedObjectMeta
	Cluster           string
	ServerImage       string
	ServerVersion     string
	Size              int32
	Resources         *corev1.ResourceRequirements
	SystemReplication SystemReplication
	StorageConfig     *cassdcapi.StorageConfig
	Racks             []cassdcapi.Rack
	CassandraConfig   *api.CassandraConfig
	AdditionalSeeds   []string
	Networking        *cassdcapi.NetworkingConfig
}

func NewDatacenter(klusterKey types.NamespacedName, template *DatacenterConfig) (*cassdcapi.CassandraDatacenter, error) {
	namespace := template.Meta.Namespace
	if len(namespace) == 0 {
		namespace = klusterKey.Namespace
	}

	rawConfig, err := CreateJsonConfig(template.CassandraConfig, template.ServerVersion)
	if err != nil {
		return nil, err
	}

	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        template.Meta.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.NameLabel:             api.NameLabelValue,
				api.PartOfLabel:           api.PartOfLabelValue,
				api.ComponentLabel:        api.ComponentLabelValueCassandra,
				api.CreatedByLabel:        api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterLabel: klusterKey.Name,
			},
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:     template.Cluster,
			ServerImage:     template.ServerImage,
			Size:            template.Size,
			ServerType:      "cassandra",
			ServerVersion:   template.ServerVersion,
			Config:          rawConfig,
			Racks:           template.Racks,
			StorageConfig:   *template.StorageConfig,
			AdditionalSeeds: template.AdditionalSeeds,
			Networking:      template.Networking,
		},
	}

	if template.Resources != nil {
		dc.Spec.Resources = *template.Resources
	}

	return dc, nil
}

// Coalesce combines the cluster and dc templates with override semantics. If a property is
// defined in both templates, the dc-level property takes precedence.
func Coalesce(clusterTemplate *api.CassandraClusterTemplate, dcTemplate *api.CassandraDatacenterTemplate) *DatacenterConfig {
	dcConfig := &DatacenterConfig{}

	// Handler cluster-wide settings first
	dcConfig.Cluster = clusterTemplate.Cluster

	// DC-level settings
	dcConfig.Meta = dcTemplate.Meta
	dcConfig.Size = dcTemplate.Size

	if len(dcTemplate.ServerVersion) == 0 {
		dcConfig.ServerVersion = clusterTemplate.ServerVersion
	} else {
		dcConfig.ServerVersion = dcTemplate.ServerVersion
	}

	if len(dcTemplate.ServerImage) == 0 {
		dcConfig.ServerImage = clusterTemplate.ServerImage
	} else {
		dcConfig.ServerImage = dcTemplate.ServerImage
	}

	if len(dcTemplate.Racks) == 0 {
		dcConfig.Racks = clusterTemplate.Racks
	} else {
		dcConfig.Racks = dcTemplate.Racks
	}

	if dcTemplate.Resources == nil {
		dcConfig.Resources = clusterTemplate.Resources
	} else {
		dcConfig.Resources = dcTemplate.Resources
	}

	// TODO Add validation check to ensure StorageConfig is set at the cluster or DC level
	if dcTemplate.StorageConfig == nil {
		dcConfig.StorageConfig = clusterTemplate.StorageConfig
	} else {
		dcConfig.StorageConfig = dcTemplate.StorageConfig
	}

	if dcTemplate.Networking == nil {
		dcConfig.Networking = clusterTemplate.Networking
	} else {
		dcConfig.Networking = dcTemplate.Networking
	}

	// TODO Do we want merge vs override?
	if dcTemplate.CassandraConfig == nil {
		dcConfig.CassandraConfig = clusterTemplate.CassandraConfig
	} else {
		dcConfig.CassandraConfig = dcTemplate.CassandraConfig
	}

	return dcConfig
}
