package cassandra

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	Meta                api.EmbeddedObjectMeta
	Cluster             string
	SuperUserSecretName string
	ServerImage         string
	ServerVersion       string
	Size                int32
	Resources           *corev1.ResourceRequirements
	SystemReplication   SystemReplication
	StorageConfig       *cassdcapi.StorageConfig
	Racks               []cassdcapi.Rack
	CassandraConfig     *api.CassandraConfig
	AdditionalSeeds     []string
	Networking          *cassdcapi.NetworkingConfig
	Users               []cassdcapi.CassandraUser
	PodTemplateSpec     *corev1.PodTemplateSpec
	MgmtAPIHeap         *resource.Quantity
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

	if template.StorageConfig == nil {
		return nil, DCConfigIncomplete{"template.StorageConfig"}
	}

	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        template.Meta.Name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.NameLabel:                 api.NameLabelValue,
				api.PartOfLabel:               api.PartOfLabelValue,
				api.ComponentLabel:            api.ComponentLabelValueCassandra,
				api.CreatedByLabel:            api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterNameLabel: klusterKey.Name,
			},
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			Size:                template.Size,
			ServerVersion:       template.ServerVersion,
			ServerImage:         template.ServerImage,
			ServerType:          "cassandra",
			Config:              rawConfig,
			Racks:               template.Racks,
			StorageConfig:       *template.StorageConfig,
			ClusterName:         template.Cluster,
			SuperuserSecretName: template.SuperUserSecretName,
			Users:               template.Users,
			Networking:          template.Networking,
			AdditionalSeeds:     template.AdditionalSeeds,
			PodTemplateSpec:     template.PodTemplateSpec,
		},
	}

	if template.Resources != nil {
		dc.Spec.Resources = *template.Resources
	}

	if template.MgmtAPIHeap != nil {
		SetMgmtAPIHeap(dc, template.MgmtAPIHeap)
	}

	return dc, nil
}

// SetMgmtAPIHeap sets the management API heap size on a CassandraDatacenter
func SetMgmtAPIHeap(dc *cassdcapi.CassandraDatacenter, heapSize *resource.Quantity) {
	if dc.Spec.PodTemplateSpec == nil {
		dc.Spec.PodTemplateSpec = &corev1.PodTemplateSpec{}
	}
	if len(dc.Spec.PodTemplateSpec.Spec.Containers) == 0 {
		dc.Spec.PodTemplateSpec.Spec.Containers = []corev1.Container{{Name: "cassandra"}}
	}
	var cassIndex int
	for i, container := range dc.Spec.PodTemplateSpec.Spec.Containers {
		if container.Name == "cassandra" {
			cassIndex = i
			break
		}
	}
	heapSize.Format = resource.Format(resource.DecimalSI)
	dc.Spec.PodTemplateSpec.Spec.Containers[cassIndex].Env = append(
		dc.Spec.PodTemplateSpec.Spec.Containers[cassIndex].Env,
		corev1.EnvVar{
			Name:  "MGMT_API_HEAP_SIZE",
			Value: string(heapSize.String()),
		},
	)
}

// Coalesce combines the cluster and dc templates with override semantics. If a property is
// defined in both templates, the dc-level property takes precedence.
func Coalesce(clusterTemplate *api.CassandraClusterTemplate, dcTemplate *api.CassandraDatacenterTemplate) *DatacenterConfig {
	dcConfig := &DatacenterConfig{}

	// Handler cluster-wide settings first
	dcConfig.Cluster = clusterTemplate.Cluster
	dcConfig.SuperUserSecretName = clusterTemplate.SuperuserSecretName

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

	if dcTemplate.MgmtAPIHeap == nil {
		dcConfig.MgmtAPIHeap = clusterTemplate.MgmtAPIHeap
	} else if dcTemplate.MgmtAPIHeap != nil {
		dcConfig.MgmtAPIHeap = dcTemplate.MgmtAPIHeap
	}

	return dcConfig
}
