package cassandra

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	goalesceutils "github.com/k8ssandra/k8ssandra-operator/pkg/goalesce"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	VectorContainerName = "vector-agent"
)

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
	Meta                      api.EmbeddedObjectMeta
	K8sContext                string
	Cluster                   string
	SuperuserSecretRef        corev1.LocalObjectReference
	ServerImage               string
	ServerVersion             *semver.Version
	ServerType                api.ServerDistribution
	JmxInitContainerImage     *images.Image
	Size                      int32
	Stopped                   bool
	Resources                 *corev1.ResourceRequirements
	StorageConfig             *cassdcapi.StorageConfig
	Racks                     []cassdcapi.Rack
	CassandraConfig           api.CassandraConfig
	AdditionalSeeds           []string
	Networking                *cassdcapi.NetworkingConfig
	Users                     []cassdcapi.CassandraUser
	PodTemplateSpec           corev1.PodTemplateSpec
	MgmtAPIHeap               *resource.Quantity
	SoftPodAntiAffinity       *bool
	Tolerations               []corev1.Toleration
	ServerEncryptionStores    *encryption.Stores
	ClientEncryptionStores    *encryption.Stores
	ClientKeystorePassword    string
	ClientTruststorePassword  string
	ServerKeystorePassword    string
	ServerTruststorePassword  string
	CDC                       *cassdcapi.CDCConfiguration
	DseWorkloads              *cassdcapi.DseWorkloads
	ManagementApiAuth         *cassdcapi.ManagementApiAuthConfig
	PerNodeConfigMapRef       corev1.LocalObjectReference
	PerNodeInitContainerImage string
	ServiceAccount            string
	ExternalSecrets           bool
	McacEnabled               bool

	// InitialTokensByPodName is a list of initial tokens for the RF first pods in the cluster. It
	// is only populated when num_tokens < 16 in the whole cluster. Used for generating default
	// per-node configurations; not transferred directly to the CassandraDatacenter CRD but its
	// presence affects the PodTemplateSpec.
	InitialTokensByPodName map[string][]string
}

const (
	mgmtApiHeapSizeEnvVar = "MANAGEMENT_API_HEAP_SIZE"
	mcacDisabledEnvVar    = "MGMT_API_DISABLE_MCAC"
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
			PodTemplateSpec:     &template.PodTemplateSpec,
			CDC:                 template.CDC,
			DseWorkloads:        template.DseWorkloads,
			ServiceAccount:      template.ServiceAccount,
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

	if position, found := FindInitContainer(&template.PodTemplateSpec, reconciliation.ServerConfigContainerName); found {
		configBuilderResources := template.PodTemplateSpec.Spec.InitContainers[position].Resources
		if configBuilderResources.Limits != nil || configBuilderResources.Requests != nil {
			dc.Spec.ConfigBuilderResources = configBuilderResources
		}
	}

	if template.ManagementApiAuth != nil {
		dc.Spec.ManagementApiAuth = *template.ManagementApiAuth
	}

	m := template.Meta.Metadata
	dc.ObjectMeta.Labels = utils.MergeMap(dc.ObjectMeta.Labels, m.Labels)
	dc.ObjectMeta.Annotations = utils.MergeMap(dc.ObjectMeta.Annotations, m.Annotations)

	if m.CommonLabels != nil {
		dc.Spec.AdditionalLabels = m.CommonLabels
	}

	dc.Spec.AdditionalServiceConfig = m.ServiceConfig.ToCassAdditionalServiceConfig()

	dc.Spec.Tolerations = template.Tolerations

	if !template.McacEnabled {
		// MCAC needs to be disabled
		setMcacDisabled(dc, template)
	}

	return dc, nil
}

// setMgmtAPIHeap sets the management API heap size on a CassandraDatacenter
func setMgmtAPIHeap(dc *cassdcapi.CassandraDatacenter, heapSize *resource.Quantity) {
	UpdateCassandraContainer(dc.Spec.PodTemplateSpec, func(c *corev1.Container) {
		heapSizeInBytes := heapSize.Value()
		c.Env = append(c.Env, corev1.EnvVar{Name: mgmtApiHeapSizeEnvVar, Value: fmt.Sprintf("%v", heapSizeInBytes)})
	})
}

func setMcacDisabled(dc *cassdcapi.CassandraDatacenter, template *DatacenterConfig) {
	UpdateCassandraContainer(dc.Spec.PodTemplateSpec, func(c *corev1.Container) {
		c.Env = append(
			c.Env,
			corev1.EnvVar{
				Name:  mcacDisabledEnvVar,
				Value: "true",
			},
		)
	})
}

// UpdateCassandraContainer finds the cassandra container, passes it to f, and then adds it
// back to the PodTemplateSpec. The Container object is created if necessary before calling
// f. Only the Name field is initialized.
func UpdateCassandraContainer(p *corev1.PodTemplateSpec, f func(c *corev1.Container)) {
	UpdateContainer(p, reconciliation.CassandraContainerName, f)
}

// UpdateVectorContainer finds the vector container, passes it to f, and then adds it
// back to the PodTemplateSpec. The Container object is created if necessary before calling
// f. Only the Name field is initialized.
func UpdateVectorContainer(p *corev1.PodTemplateSpec, f func(c *corev1.Container)) {
	UpdateContainer(p, VectorContainerName, f)
}

// UpdateContainer finds the container with the given name, passes it to f, and then adds it
// back to the PodTemplateSpec. The Container object is created if necessary before calling
// f. Only the Name field is initialized.
func UpdateContainer(p *corev1.PodTemplateSpec, name string, f func(c *corev1.Container)) {
	idx, found := FindContainer(p, name)
	var container *corev1.Container

	if !found {
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

// Coalesce merges the cluster and dc templates. If a property is defined in both templates, the
// dc-level property takes precedence.
func Coalesce(clusterName string, clusterTemplate *api.CassandraClusterTemplate, dcTemplate *api.CassandraDatacenterTemplate) *DatacenterConfig {
	dcConfig := &DatacenterConfig{}

	// Handle cluster-wide settings first
	dcConfig.Cluster = clusterName
	dcConfig.SuperuserSecretRef = clusterTemplate.SuperuserSecretRef
	dcConfig.ServerType = clusterTemplate.ServerType
	dcConfig.ServerEncryptionStores = clusterTemplate.ServerEncryptionStores
	dcConfig.ClientEncryptionStores = clusterTemplate.ClientEncryptionStores
	dcConfig.AdditionalSeeds = clusterTemplate.AdditionalSeeds

	// DC-level settings
	dcConfig.K8sContext = dcTemplate.K8sContext // can be empty
	dcConfig.Meta = dcTemplate.Meta
	dcConfig.Size = dcTemplate.Size
	dcConfig.Stopped = dcTemplate.Stopped
	dcConfig.PerNodeConfigMapRef = dcTemplate.PerNodeConfigMapRef
	dcConfig.CDC = dcTemplate.CDC

	mergedOptions := goalesceutils.MergeCRs(clusterTemplate.DatacenterOptions, dcTemplate.DatacenterOptions)

	if len(mergedOptions.ServerVersion) > 0 {
		dcConfig.ServerVersion = semver.MustParse(mergedOptions.ServerVersion)
	}
	dcConfig.ServerImage = mergedOptions.ServerImage
	dcConfig.JmxInitContainerImage = mergedOptions.JmxInitContainerImage
	dcConfig.Racks = mergedOptions.Racks
	dcConfig.Resources = mergedOptions.Resources
	dcConfig.StorageConfig = mergedOptions.StorageConfig
	dcConfig.Networking = mergedOptions.Networking.ToCassNetworkingConfig()
	if mergedOptions.CassandraConfig != nil {
		dcConfig.CassandraConfig = *mergedOptions.CassandraConfig
	}
	dcConfig.MgmtAPIHeap = mergedOptions.MgmtAPIHeap
	dcConfig.SoftPodAntiAffinity = mergedOptions.SoftPodAntiAffinity
	dcConfig.Tolerations = mergedOptions.Tolerations
	dcConfig.DseWorkloads = mergedOptions.DseWorkloads
	dcConfig.ManagementApiAuth = mergedOptions.ManagementApiAuth
	dcConfig.PodTemplateSpec.Spec.SecurityContext = mergedOptions.PodSecurityContext
	dcConfig.PerNodeInitContainerImage = mergedOptions.PerNodeConfigInitContainerImage
	dcConfig.ServiceAccount = mergedOptions.ServiceAccount

	dcConfig.Meta.Metadata = goalesceutils.MergeCRs(clusterTemplate.Meta, dcTemplate.Meta.Metadata)

	AddPodTemplateSpecMeta(dcConfig, dcConfig.Meta.Metadata)

	if len(mergedOptions.Containers) > 0 {
		AddContainersToPodTemplateSpec(dcConfig, mergedOptions.Containers...)
	}

	if len(mergedOptions.InitContainers) > 0 {
		AddInitContainersToPodTemplateSpec(dcConfig, mergedOptions.InitContainers...)
	}

	if mergedOptions.ExtraVolumes != nil {
		AddK8ssandraVolumesToPodTemplateSpec(dcConfig, *mergedOptions.ExtraVolumes)
	}

	// we need to declare at least one container, otherwise the PodTemplateSpec struct will be invalid
	UpdateCassandraContainer(&dcConfig.PodTemplateSpec, func(c *corev1.Container) {})

	dcConfig.McacEnabled = mergedOptions.Telemetry.IsMcacEnabled()

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

func AddContainersToPodTemplateSpec(dcConfig *DatacenterConfig, containers ...corev1.Container) {
	dcConfig.PodTemplateSpec.Spec.Containers = append(dcConfig.PodTemplateSpec.Spec.Containers, containers...)
}

func AddInitContainersToPodTemplateSpec(dcConfig *DatacenterConfig, initContainers ...corev1.Container) {
	dcConfig.PodTemplateSpec.Spec.InitContainers = append(dcConfig.PodTemplateSpec.Spec.InitContainers, initContainers...)
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
		AddVolumesToPodTemplateSpec(&dcConfig.PodTemplateSpec, volume)
	}
}

func AddVolumesToPodTemplateSpec(podTemplateSpec *corev1.PodTemplateSpec, volume corev1.Volume) {
	podTemplateSpec.Spec.Volumes = append(podTemplateSpec.Spec.Volumes, volume)
}

func AddPodTemplateSpecMeta(dcConfig *DatacenterConfig, m meta.CassandraDatacenterMeta) {
	// We don't need to overlay m.CommonLabels as this will be done by cass-operator itself
	dcConfig.PodTemplateSpec.ObjectMeta = metav1.ObjectMeta{
		Annotations: m.Pods.Annotations,
		Labels:      m.Pods.Labels,
	}

}

func FindContainer(dcPodTemplateSpec *corev1.PodTemplateSpec, containerName string) (int, bool) {
	if dcPodTemplateSpec != nil {
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
	for i, volume := range dcConfig.StorageConfig.AdditionalVolumes {
		if volume.Name == volumeName {
			return i, true
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
