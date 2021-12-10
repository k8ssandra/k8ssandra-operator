package cassandra

import (
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
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
	Datacenters       []string `json:"datacenters"`
	ReplicationFactor int      `json:"replicationFactor"`
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

const (
	mgmtApiHeapSizeEnvVar = "MANAGEMENT_API_HEAP_SIZE"
)

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
				api.NameLabel:                      api.NameLabelValue,
				api.PartOfLabel:                    api.PartOfLabelValue,
				api.ComponentLabel:                 api.ComponentLabelValueCassandra,
				api.CreatedByLabel:                 api.CreatedByLabelValueK8ssandraClusterController,
				api.K8ssandraClusterNameLabel:      klusterKey.Name,
				api.K8ssandraClusterNamespaceLabel: klusterKey.Namespace,
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
		setMgmtAPIHeap(dc, template.MgmtAPIHeap)
	}

	return dc, nil
}

// setMgmtAPIHeap sets the management API heap size on a CassandraDatacenter
func setMgmtAPIHeap(dc *cassdcapi.CassandraDatacenter, heapSize *resource.Quantity) {
	if dc.Spec.PodTemplateSpec == nil {
		dc.Spec.PodTemplateSpec = &corev1.PodTemplateSpec{}
	}

	UpdateCassandraContainer(dc.Spec.PodTemplateSpec, func(c *corev1.Container) {
		heapSizeInBytes := heapSize.Value()
		c.Env = append(c.Env, corev1.EnvVar{Name: mgmtApiHeapSizeEnvVar, Value: fmt.Sprintf("%v", heapSizeInBytes)})
	})
}

// UpdateCassandraContainer finds the cassandra container, passes it to f, and then adds it
// back to the PodTemplateSpec. The Container object is created if necessary before calling
// f. Only the Name field is initialized.
func UpdateCassandraContainer(p *corev1.PodTemplateSpec, f func(c *corev1.Container)) {
	idx := -1
	container := &corev1.Container{}

	for i, c := range p.Spec.Containers {
		if c.Name == reconciliation.CassandraContainerName {
			idx = i
			break
		}
	}

	if idx == -1 {
		idx = 0
		container.Name = reconciliation.CassandraContainerName
		p.Spec.Containers = make([]corev1.Container, 1)
	} else {
		container = &p.Spec.Containers[idx]
	}

	f(container)
	p.Spec.Containers[idx] = *container
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

func FindContainer(dcPodTemplateSpec *corev1.PodTemplateSpec, containerName string) (int, bool) {
	if dcPodTemplateSpec.Spec.Containers != nil {
		for i, container := range dcPodTemplateSpec.Spec.Containers {
			if container.Name == containerName {
				return i, true
			}
		}
	}

	return -1, false
}

func FindInitContainer(dcPodTemplateSpec *corev1.PodTemplateSpec, containerName string) (int, bool) {
	if dcPodTemplateSpec != nil {
		for i, container := range dcPodTemplateSpec.Spec.InitContainers {
			if container.Name == containerName {
				return i, true
			}
		}
	}

	return -1, false
}

func FindVolume(dcPodTemplateSpec *corev1.PodTemplateSpec, volumeName string) (int, bool) {
	if dcPodTemplateSpec != nil {
		for i, volume := range dcPodTemplateSpec.Spec.Volumes {
			if volume.Name == volumeName {
				return i, true
			}
		}
	}

	return -1, false
}
