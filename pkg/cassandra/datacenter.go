package cassandra

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
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
// is configured per DC.
type SystemReplication map[string]int

// // Replication provides a mapping of DCs to a mapping of keyspaces and their
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
	Meta                     api.EmbeddedObjectMeta
	K8sContext               string
	Cluster                  string
	SuperuserSecretRef       corev1.LocalObjectReference
	ServerImage              string
	ServerVersion            *semver.Version
	ServerType               api.ServerDistribution
	JmxInitContainerImage    *images.Image
	Size                     int32
	Stopped                  bool
	Resources                *corev1.ResourceRequirements
	SystemReplication        SystemReplication
	StorageConfig            *cassdcapi.StorageConfig
	Racks                    []cassdcapi.Rack
	CassandraConfig          api.CassandraConfig
	AdditionalSeeds          []string
	Networking               *cassdcapi.NetworkingConfig
	Users                    []cassdcapi.CassandraUser
	PodTemplateSpec          *corev1.PodTemplateSpec
	MgmtAPIHeap              *resource.Quantity
	SoftPodAntiAffinity      *bool
	Tolerations              []corev1.Toleration
	ServerEncryptionStores   *encryption.Stores
	ClientEncryptionStores   *encryption.Stores
	ClientKeystorePassword   string
	ClientTruststorePassword string
	ServerKeystorePassword   string
	ServerTruststorePassword string
	CDC                      *cassdcapi.CDCConfiguration
	DseWorkloads             *cassdcapi.DseWorkloads
	ConfigBuilderResources   *corev1.ResourceRequirements
	ManagementApiAuth        *cassdcapi.ManagementApiAuthConfig
	PerNodeConfigMapRef      corev1.LocalObjectReference
	ExternalSecrets          bool

	// InitialTokensByPodName is a list of initial tokens for the RF first pods in the cluster. It
	// is only populated when num_tokens < 16 in the whole cluster. Used for generating default
	// per-node configurations; not transferred directly to the CassandraDatacenter CRD but its
	// presence affects the PodTemplateSpec.
	InitialTokensByPodName map[string][]string
}

const (
	mgmtApiHeapSizeEnvVar = "MANAGEMENT_API_HEAP_SIZE"
)

func NewDatacenter(klusterKey types.NamespacedName, template *DatacenterConfig) (*cassdcapi.CassandraDatacenter, error) {
	namespace := utils.FirstNonEmptyString(template.Meta.Namespace, klusterKey.Namespace)

	rawConfig, err := createJsonConfig(template.CassandraConfig, template.ServerVersion, template.ServerType)
	if err != nil {
		return nil, err
	}

	// if using external secrets, make sure superUserSecretName is empty
	superUserSecretName := template.SuperuserSecretRef.Name
	if template.ExternalSecrets {
		superUserSecretName = ""
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
			Stopped:             template.Stopped,
			ServerVersion:       template.ServerVersion.String(),
			ServerImage:         template.ServerImage,
			ServerType:          string(template.ServerType),
			Config:              rawConfig,
			Racks:               template.Racks,
			StorageConfig:       *template.StorageConfig,
			ClusterName:         template.Cluster,
			SuperuserSecretName: superUserSecretName,
			Users:               template.Users,
			Networking:          template.Networking,
			PodTemplateSpec:     template.PodTemplateSpec,
			CDC:                 template.CDC,
			DseWorkloads:        template.DseWorkloads,
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

	if template.ConfigBuilderResources != nil {
		dc.Spec.ConfigBuilderResources = *template.ConfigBuilderResources
	}

	if template.ManagementApiAuth != nil {
		dc.Spec.ManagementApiAuth = *template.ManagementApiAuth
	}

	dc.Spec.Tolerations = template.Tolerations

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
	dcConfig.ServerType = clusterTemplate.ServerType

	// DC-level settings
	dcConfig.K8sContext = dcTemplate.K8sContext // can be empty
	dcConfig.Meta = dcTemplate.Meta
	dcConfig.Size = dcTemplate.Size
	dcConfig.Stopped = dcTemplate.Stopped
	dcConfig.PerNodeConfigMapRef = dcTemplate.PerNodeConfigMapRef

	if len(dcTemplate.DatacenterOptions.ServerVersion) > 0 {
		dcConfig.ServerVersion = semver.MustParse(dcTemplate.DatacenterOptions.ServerVersion)
	} else if len(clusterTemplate.DatacenterOptions.ServerVersion) > 0 {
		dcConfig.ServerVersion = semver.MustParse(clusterTemplate.DatacenterOptions.ServerVersion)
	}

	if len(dcTemplate.DatacenterOptions.ServerImage) == 0 {
		dcConfig.ServerImage = clusterTemplate.DatacenterOptions.ServerImage
	} else {
		dcConfig.ServerImage = dcTemplate.DatacenterOptions.ServerImage
	}

	if dcTemplate.DatacenterOptions.JmxInitContainerImage != nil {
		dcConfig.JmxInitContainerImage = dcTemplate.DatacenterOptions.JmxInitContainerImage
	} else {
		dcConfig.JmxInitContainerImage = clusterTemplate.DatacenterOptions.JmxInitContainerImage
	}

	if len(dcTemplate.DatacenterOptions.Racks) == 0 {
		dcConfig.Racks = clusterTemplate.DatacenterOptions.Racks
	} else {
		dcConfig.Racks = dcTemplate.DatacenterOptions.Racks
	}

	if dcTemplate.DatacenterOptions.Resources == nil {
		dcConfig.Resources = clusterTemplate.DatacenterOptions.Resources
	} else {
		dcConfig.Resources = dcTemplate.DatacenterOptions.Resources
	}

	// TODO Add validation check to ensure StorageConfig is set at the cluster or DC level
	if dcTemplate.DatacenterOptions.StorageConfig == nil {
		dcConfig.StorageConfig = clusterTemplate.DatacenterOptions.StorageConfig
	} else {
		dcConfig.StorageConfig = dcTemplate.DatacenterOptions.StorageConfig
	}

	if dcTemplate.DatacenterOptions.Networking == nil {
		dcConfig.Networking = clusterTemplate.DatacenterOptions.Networking
	} else {
		dcConfig.Networking = dcTemplate.DatacenterOptions.Networking
	}

	// TODO Do we want merge vs override?
	if dcTemplate.DatacenterOptions.CassandraConfig != nil {
		dcConfig.CassandraConfig = *dcTemplate.DatacenterOptions.CassandraConfig
	} else if clusterTemplate.DatacenterOptions.CassandraConfig != nil {
		dcConfig.CassandraConfig = *clusterTemplate.DatacenterOptions.CassandraConfig
	}

	if dcTemplate.DatacenterOptions.MgmtAPIHeap == nil {
		dcConfig.MgmtAPIHeap = clusterTemplate.DatacenterOptions.MgmtAPIHeap
	} else {
		dcConfig.MgmtAPIHeap = dcTemplate.DatacenterOptions.MgmtAPIHeap
	}

	if dcTemplate.CDC != nil {
		dcConfig.CDC = dcTemplate.CDC
	}

	if dcTemplate.DatacenterOptions.SoftPodAntiAffinity == nil {
		dcConfig.SoftPodAntiAffinity = clusterTemplate.DatacenterOptions.SoftPodAntiAffinity
	} else {
		dcConfig.SoftPodAntiAffinity = dcTemplate.DatacenterOptions.SoftPodAntiAffinity
	}

	if len(dcTemplate.DatacenterOptions.Tolerations) == 0 {
		dcConfig.Tolerations = clusterTemplate.DatacenterOptions.Tolerations
	} else {
		dcConfig.Tolerations = dcTemplate.DatacenterOptions.Tolerations
	}

	// Client/Server Encryption stores are only defined at the cluster level
	dcConfig.ServerEncryptionStores = clusterTemplate.ServerEncryptionStores
	dcConfig.ClientEncryptionStores = clusterTemplate.ClientEncryptionStores
	dcConfig.AdditionalSeeds = clusterTemplate.AdditionalSeeds

	if dcTemplate.DatacenterOptions.DseWorkloads != nil {
		dcConfig.DseWorkloads = dcTemplate.DatacenterOptions.DseWorkloads
	} else if clusterTemplate.DatacenterOptions.DseWorkloads != nil {
		dcConfig.DseWorkloads = clusterTemplate.DatacenterOptions.DseWorkloads
	}

	if dcTemplate.DatacenterOptions.ManagementApiAuth != nil {
		dcConfig.ManagementApiAuth = dcTemplate.DatacenterOptions.ManagementApiAuth
	} else if clusterTemplate.DatacenterOptions.ManagementApiAuth != nil {
		dcConfig.ManagementApiAuth = clusterTemplate.DatacenterOptions.ManagementApiAuth
	}

	// Ensure we have a valid PodTemplateSpec before proceeding to modify it.
	// FIXME if we are doing this, then we should remove the pointer
	dcConfig.PodTemplateSpec = &corev1.PodTemplateSpec{}

	if dcTemplate.AdditionalPodAnnotations != nil && clusterTemplate.AdditionalPodAnnotations != nil {
		podAnnotations := clusterTemplate.AdditionalPodAnnotations
		for k, v := range dcTemplate.AdditionalPodAnnotations {
			podAnnotations[k] = v
		}
		dcConfig.PodTemplateSpec.Annotations = podAnnotations
	} else if clusterTemplate.AdditionalPodAnnotations != nil {
		dcConfig.PodTemplateSpec.Annotations = clusterTemplate.AdditionalPodAnnotations
	}

	var podSecurityContext *corev1.PodSecurityContext
	if dcTemplate.DatacenterOptions.PodSecurityContext != nil {
		podSecurityContext = dcTemplate.DatacenterOptions.PodSecurityContext
	} else if clusterTemplate.DatacenterOptions.PodSecurityContext != nil {
		podSecurityContext = clusterTemplate.DatacenterOptions.PodSecurityContext
	}
	// Add the SecurityContext to the PodTemplateSpec if it's defined
	if podSecurityContext != nil {
		dcConfig.PodTemplateSpec.Spec.SecurityContext = podSecurityContext
	}

	// Create additional containers if requested
	var containers []corev1.Container
	if len(dcTemplate.DatacenterOptions.Containers) > 0 {
		containers = dcTemplate.DatacenterOptions.Containers
	} else if len(clusterTemplate.DatacenterOptions.Containers) > 0 {
		containers = clusterTemplate.DatacenterOptions.Containers
	}
	if len(containers) > 0 {
		_ = AddContainersToPodTemplateSpec(dcConfig, containers...)
	}

	// Create additional init containers if requested
	var initContainers []corev1.Container
	if len(dcTemplate.DatacenterOptions.InitContainers) > 0 {
		initContainers = dcTemplate.DatacenterOptions.InitContainers
	} else if len(clusterTemplate.DatacenterOptions.InitContainers) > 0 {
		initContainers = clusterTemplate.DatacenterOptions.InitContainers
	}
	if len(initContainers) > 0 {
		_ = AddInitContainersToPodTemplateSpec(dcConfig, initContainers...)
	}

	// Create additional volumes if requested
	var extraVolumes *api.K8ssandraVolumes
	if dcTemplate.DatacenterOptions.ExtraVolumes != nil {
		extraVolumes = dcTemplate.DatacenterOptions.ExtraVolumes
	} else if clusterTemplate.DatacenterOptions.ExtraVolumes != nil {
		extraVolumes = clusterTemplate.DatacenterOptions.ExtraVolumes
	}
	if extraVolumes != nil {
		AddK8ssandraVolumesToPodTemplateSpec(dcConfig, *extraVolumes)
	}

	// we need to declare at least one container, otherwise the PodTemplateSpec struct will be invalid
	UpdateCassandraContainer(dcConfig.PodTemplateSpec, func(c *corev1.Container) {})

	return dcConfig
}

// ValidateDatacenterConfig checks the coalesced DC config for missing fields and mandatory options,
// and then validates the cassandra.yaml file.
func ValidateDatacenterConfig(dcConfig *DatacenterConfig) error {
	if dcConfig.ServerVersion == nil {
		return DCConfigIncomplete{"template.ServerVersion"}
	}
	if dcConfig.ServerType == "" {
		return DCConfigIncomplete{"template.ServerType"}
	}
	if dcConfig.StorageConfig == nil {
		return DCConfigIncomplete{"template.StorageConfig"}
	}
	if err := validateCassandraYaml(dcConfig.CassandraConfig.CassandraYaml); err != nil {
		return err
	}
	return nil
}

func AddContainersToPodTemplateSpec(dcConfig *DatacenterConfig, containers ...corev1.Container) error {
	if dcConfig.PodTemplateSpec == nil {
		return errors.New("PodTemplateSpec was nil, cannot add containers")
	} else {
		dcConfig.PodTemplateSpec.Spec.Containers = append(dcConfig.PodTemplateSpec.Spec.Containers, containers...)
		return nil
	}
}

func AddInitContainersToPodTemplateSpec(dcConfig *DatacenterConfig, initContainers ...corev1.Container) error {
	if dcConfig.PodTemplateSpec == nil {
		return errors.New("PodTemplateSpec was nil, cannot add init containers")
	} else {
		dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, initContainers...)
		position, found := FindInitContainer(dcConfig.PodTemplateSpec, reconciliation.ServerConfigContainerName)
		if found && (dcConfig.PodTemplateSpec.Spec.InitContainers[position].Resources.Limits != nil || dcConfig.PodTemplateSpec.Spec.InitContainers[position].Resources.Requests != nil) {
			dcConfig.ConfigBuilderResources = &dcConfig.PodTemplateSpec.Spec.InitContainers[position].Resources
		}
		return nil
	}
}

func AddK8ssandraVolumesToPodTemplateSpec(dcConfig *DatacenterConfig, extraVolumes api.K8ssandraVolumes) {
	// Add and mount additional volumes that need to be managed by the statefulset
	if len(extraVolumes.PVCs) > 0 {
		if dcConfig.StorageConfig == nil {
			dcConfig.StorageConfig = &cassdcapi.StorageConfig{
				AdditionalVolumes: extraVolumes.PVCs,
			}
		} else {
			dcConfig.StorageConfig.AdditionalVolumes = append(dcConfig.StorageConfig.AdditionalVolumes, extraVolumes.PVCs...)
		}
	}

	// Add extra volumes that do not need to be managed by the statefulset
	for _, volume := range extraVolumes.Volumes {
		AddVolumesToPodTemplateSpec(dcConfig, volume)
	}
}

func AddVolumesToPodTemplateSpec(dcConfig *DatacenterConfig, volume corev1.Volume) {
	if dcConfig.PodTemplateSpec == nil {
		dcConfig.PodTemplateSpec = &corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{volume},
			},
		}
	} else {
		dcConfig.PodTemplateSpec.Spec.Volumes = append(dcConfig.PodTemplateSpec.Spec.Volumes, volume)
	}
}

func FindContainer(dcPodTemplateSpec *corev1.PodTemplateSpec, containerName string) (int, bool) {
	if dcPodTemplateSpec != nil && dcPodTemplateSpec.Spec.Containers != nil {
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

func FindVolumeMount(container *corev1.Container, name string) *corev1.VolumeMount {
	for _, v := range container.VolumeMounts {
		if v.Name == name {
			return &v
		}
	}
	return nil
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

func AddOrUpdateVolume(dcConfig *DatacenterConfig, volume *corev1.Volume, volumeIndex int, found bool) {
	if !found {
		// volume doesn't exist, we need to add it
		dcConfig.PodTemplateSpec.Spec.Volumes = append(dcConfig.PodTemplateSpec.Spec.Volumes, *volume)
	} else {
		// Overwrite existing volume
		dcConfig.PodTemplateSpec.Spec.Volumes[volumeIndex] = *volume
	}
}

func AddOrUpdateAdditionalVolume(dcConfig *DatacenterConfig, volume *v1beta1.AdditionalVolumes, volumeIndex int, found bool) {
	if dcConfig.StorageConfig.AdditionalVolumes == nil {
		dcConfig.StorageConfig.AdditionalVolumes = make(v1beta1.AdditionalVolumesSlice, 0)
	}
	if !found {
		// volume doesn't exist, we need to add it
		dcConfig.StorageConfig.AdditionalVolumes = append(dcConfig.StorageConfig.AdditionalVolumes, *volume)
	} else {
		// Overwrite existing volume
		dcConfig.StorageConfig.AdditionalVolumes[volumeIndex] = *volume
	}
}

func AddOrUpdateVolumeMount(container *corev1.Container, volume *corev1.Volume, mountPath string) {
	newVolumeMount := corev1.VolumeMount{
		Name:      volume.Name,
		MountPath: mountPath,
	}
	for i, volumeMount := range container.VolumeMounts {
		if volumeMount.Name == volume.Name {
			container.VolumeMounts[i] = newVolumeMount
			return
		}
	}

	// Volume mount doesn't exist yet, we'll create it.
	container.VolumeMounts = append(container.VolumeMounts, newVolumeMount)
}
