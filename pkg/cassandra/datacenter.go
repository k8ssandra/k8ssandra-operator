package cassandra

import (
	"fmt"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var DefaultJmxInitImage = images.Image{
	Registry:   images.DefaultRegistry,
	Repository: images.DockerOfficialRepository,
	Name:       "busybox",
	Tag:        "1.34.1",
	// When changing the default version above, please also change the kubebuilder marker in
	// apis/reaper/v1alpha1/reaper_types.go accordingly.
}

// SystemReplication represents the replication factor of the system_auth, system_traces,
// and system_distributed keyspaces. This is applied to each datacenter. The replication
// should be configured per DC, but that is currently not supported. See
// https://github.com/k8ssandra/management-api-for-apache-cassandra/issues/124 and
// https://github.com/k8ssandra/k8ssandra-operator/issues/91 for details.
// Note that when we can configure the replication per DC, this can be changed to a
// map[string]int.
type SystemReplication struct {
	Datacenters       []string `json:"datacenters"`
	ReplicationFactor int      `json:"replicationFactor"`
}

// Replication provides a mapping of DCs to a mapping of keyspaces and their
// replica counts. NetworkTopologyStrategy is assumed for all keyspaces.
type Replication struct {
	datacenters map[string]keyspacesReplication
}

type keyspacesReplication map[string]int

// EachDcContainsKeyspaces if every DC contains all the keyspaces.
func (r *Replication) EachDcContainsKeyspaces(keyspaces ...string) bool {
	for _, ksMap := range r.datacenters {
		for _, ks := range keyspaces {
			if _, found := ksMap[ks]; !found {
				return false
			}
		}
	}
	return true
}

// ForDcs returns a new Replication that contains only the specifics dcs.
func (r *Replication) ForDcs(dcs ...string) *Replication {
	replication := &Replication{datacenters: map[string]keyspacesReplication{}}

	for dc, ksReplication := range r.datacenters {
		if utils.SliceContains(dcs, dc) {
			ksMap := map[string]int{}
			for ks, val := range ksReplication {
				ksMap[ks] = val
			}
			replication.datacenters[dc] = ksMap
		}
	}

	return replication
}

func (r *Replication) ReplicationFactor(dc, ks string) int {
	if ksMap, found := r.datacenters[dc]; found {
		if rf, found := ksMap[ks]; found {
			return rf
		}
	}
	return 0
}

// DatacenterConfig provides the configuration to be applied to the CassandraDatacenter.
// A DatacenterConfig is essentially a coalescence of an api.CassandraClusterTemplate and
// an api.CassandraDatacenterTemplate. There are global, cluster-wide settings that need
// to be specified at the DC-level. Using a DatacenterConfig allows to keep the api types
// clean such that cluster-level settings won't leak into the dc-level settings.
type DatacenterConfig struct {
	Meta                  api.EmbeddedObjectMeta
	Cluster               string
	SuperuserSecretRef    corev1.LocalObjectReference
	ServerImage           string
	ServerVersion         string
	JmxInitContainerImage *images.Image
	Size                  int32
	Resources             *corev1.ResourceRequirements
	SystemReplication     SystemReplication
	StorageConfig         *cassdcapi.StorageConfig
	Racks                 []cassdcapi.Rack
	CassandraConfig       api.CassandraConfig
	AdditionalSeeds       []string
	Networking            *cassdcapi.NetworkingConfig
	Users                 []cassdcapi.CassandraUser
	PodTemplateSpec       *corev1.PodTemplateSpec
	MgmtAPIHeap           *resource.Quantity
	SoftPodAntiAffinity   *bool
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
			SuperuserSecretName: template.SuperuserSecretRef.Name,
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

	if template.SoftPodAntiAffinity != nil {
		dc.Spec.AllowMultipleNodesPerWorker = *template.SoftPodAntiAffinity
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
	UpdateContainer(p, reconciliation.CassandraContainerName, f)
}

// UpdateContainer finds the container with the given name, passes it to f, and then adds it
// back to the PodTemplateSpec. The Container object is created if necessary before calling
// f. Only the Name field is initialized.
func UpdateContainer(p *corev1.PodTemplateSpec, name string, f func(c *corev1.Container)) {
	idx := -1
	var container *corev1.Container
	for i, c := range p.Spec.Containers {
		if c.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		idx = len(p.Spec.Containers)
		container = &corev1.Container{Name: name}
		p.Spec.Containers = append(p.Spec.Containers, *container)
	} else {
		container = &p.Spec.Containers[idx]
	}

	f(container)
	p.Spec.Containers[idx] = *container
}

// UpdateInitContainer finds the init container with the given name, passes it to f, and then adds it
// back to the PodTemplateSpec. The Container object is created if necessary before calling
// f. Only the Name field is initialized.
func UpdateInitContainer(p *corev1.PodTemplateSpec, name string, f func(c *corev1.Container)) {
	idx := -1
	var container *corev1.Container
	for i, c := range p.Spec.InitContainers {
		if c.Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		idx = len(p.Spec.InitContainers)
		container = &corev1.Container{Name: name}
		p.Spec.InitContainers = append(p.Spec.InitContainers, *container)
	} else {
		container = &p.Spec.InitContainers[idx]
	}

	f(container)
	p.Spec.InitContainers[idx] = *container
}

// Coalesce combines the cluster and dc templates with override semantics. If a property is
// defined in both templates, the dc-level property takes precedence.
func Coalesce(clusterName string, clusterTemplate *api.CassandraClusterTemplate, dcTemplate *api.CassandraDatacenterTemplate) *DatacenterConfig {
	dcConfig := &DatacenterConfig{}

	// Handler cluster-wide settings first
	dcConfig.Cluster = clusterName
	dcConfig.SuperuserSecretRef = clusterTemplate.SuperuserSecretRef

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

	if dcTemplate.JmxInitContainerImage != nil {
		dcConfig.JmxInitContainerImage = dcTemplate.JmxInitContainerImage
	} else {
		dcConfig.JmxInitContainerImage = clusterTemplate.JmxInitContainerImage
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
	if dcTemplate.CassandraConfig != nil {
		dcConfig.CassandraConfig = *dcTemplate.CassandraConfig
	} else if clusterTemplate.CassandraConfig != nil {
		dcConfig.CassandraConfig = *clusterTemplate.CassandraConfig
	}

	if dcTemplate.MgmtAPIHeap == nil {
		dcConfig.MgmtAPIHeap = clusterTemplate.MgmtAPIHeap
	} else {
		dcConfig.MgmtAPIHeap = dcTemplate.MgmtAPIHeap
	}

	if dcTemplate.SoftPodAntiAffinity == nil {
		dcConfig.SoftPodAntiAffinity = clusterTemplate.SoftPodAntiAffinity
	} else {
		dcConfig.SoftPodAntiAffinity = dcTemplate.SoftPodAntiAffinity
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

func FindAdditionalVolume(dcConfig *DatacenterConfig, volumeName string) (int, bool) {
	if dcConfig.StorageConfig.AdditionalVolumes != nil {
		for i, volume := range dcConfig.StorageConfig.AdditionalVolumes {
			if volume.Name == volumeName {
				return i, true
			}
		}
	}

	return -1, false
}

func ValidateConfig(desiredDc, actualDc *cassdcapi.CassandraDatacenter) error {
	desiredConfig, err := utils.UnmarshalToMap(desiredDc.Spec.Config)
	if err != nil {
		return err
	}
	actualConfig, err := utils.UnmarshalToMap(actualDc.Spec.Config)
	if err != nil {
		return err
	}

	actualCassYaml, foundActualYaml := actualConfig["cassandra-yaml"].(map[string]interface{})
	desiredCassYaml, foundDesiredYaml := desiredConfig["cassandra-yaml"].(map[string]interface{})

	if (foundActualYaml && foundDesiredYaml) && actualCassYaml["num_tokens"] != desiredCassYaml["num_tokens"] {
		return fmt.Errorf("tried to change num_tokens in an existing datacenter")
	}

	return nil
}
