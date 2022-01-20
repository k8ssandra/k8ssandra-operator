package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	DatacenterAvailabilityLocal = "LOCAL"
	DatacenterAvailabilityEach  = "EACH"
)

// DefaultResourceName generates a name for a new Reaper resource that is derived from the Cassandra cluster and DC
// names.
func DefaultResourceName(dc *cassdcapi.CassandraDatacenter) string {
	// FIXME sanitize name
	return dc.Spec.ClusterName + "-" + dc.Name + "-reaper"
}

func NewReaper(
	reaperKey types.NamespacedName,
	kc *k8ssandraapi.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	reaperTemplate *reaperapi.ReaperClusterTemplate,
) *reaperapi.Reaper {
	labels := createResourceLabels(kc)
	desiredReaper := &reaperapi.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   reaperKey.Namespace,
			Name:        reaperKey.Name,
			Annotations: map[string]string{},
			Labels:      labels,
		},
		Spec: reaperapi.ReaperSpec{
			ReaperClusterTemplate: *reaperTemplate,
			DatacenterRef: reaperapi.CassandraDatacenterRef{
				Name:      dc.Name,
				Namespace: dc.Namespace,
			},
			DatacenterAvailability: computeReaperDcAvailability(kc),
		},
	}
	if kc.Spec.IsAuthEnabled() {
		// if auth is enabled in this cluster, the k8ssandra controller will automatically create two secrets for
		// Reaper: one for CQL connections, one for JMX connections. Here we assume that these secrets exist. If the
		// secrets were specified by the user they should be already present in desiredReaper.Spec; otherwise, we assume
		// that the k8ssandra controller created two secrets with default names, and we need to manually fill in this
		// info in desiredReaper.Spec since it wasn't persisted in reaperTemplate.
		if desiredReaper.Spec.CassandraUserSecretRef.Name == "" {
			desiredReaper.Spec.CassandraUserSecretRef.Name = DefaultUserSecretName(kc.Name)
		}
		if desiredReaper.Spec.JmxUserSecretRef.Name == "" {
			desiredReaper.Spec.JmxUserSecretRef.Name = DefaultJmxUserSecretName(kc.Name)
		}
		if desiredReaper.Spec.ReaperUiSecretRef.Name == "" {
			desiredReaper.Spec.ReaperUiSecretRef.Name = DefaultUiSecretName(kc.Name)
		}
	}
	annotations.AddHashAnnotation(desiredReaper)
	return desiredReaper
}

// See https://cassandra-reaper.io/docs/usage/multi_dc/.
// If each DC has its own Reaper instance, use EACH, otherwise use LOCAL.
func computeReaperDcAvailability(kc *k8ssandraapi.K8ssandraCluster) string {
	if kc.Spec.Reaper != nil {
		return DatacenterAvailabilityEach
	}
	reapersCount := 0
	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		if dcTemplate.Reaper != nil {
			reapersCount++
		}
	}
	if reapersCount == len(kc.Spec.Cassandra.Datacenters) {
		return DatacenterAvailabilityEach
	}
	return DatacenterAvailabilityLocal
}

// Coalesce combines the cluster and dc templates with override semantics. If a property is
// defined in both templates, the dc-level property takes precedence.
func Coalesce(clusterTemplate *api.ReaperClusterTemplate, dcTemplate *api.ReaperDatacenterTemplate) *api.ReaperClusterTemplate {

	if clusterTemplate == nil && dcTemplate == nil {
		return nil
	}

	coalesced := &api.ReaperClusterTemplate{}

	if dcTemplate != nil && dcTemplate.ContainerImage != nil {
		coalesced.ContainerImage = dcTemplate.ContainerImage
	} else if clusterTemplate != nil && clusterTemplate.ContainerImage != nil {
		coalesced.ContainerImage = clusterTemplate.ContainerImage
	}

	if dcTemplate != nil && dcTemplate.InitContainerImage != nil {
		coalesced.InitContainerImage = dcTemplate.InitContainerImage
	} else if clusterTemplate != nil && clusterTemplate.InitContainerImage != nil {
		coalesced.InitContainerImage = clusterTemplate.InitContainerImage
	}

	if dcTemplate != nil && len(dcTemplate.ServiceAccountName) != 0 {
		coalesced.ServiceAccountName = dcTemplate.ServiceAccountName
	} else if clusterTemplate != nil && len(clusterTemplate.ServiceAccountName) != 0 {
		coalesced.ServiceAccountName = clusterTemplate.ServiceAccountName
	}

	if clusterTemplate != nil {
		coalesced.Keyspace = clusterTemplate.Keyspace
		coalesced.CassandraUserSecretRef = clusterTemplate.CassandraUserSecretRef
		coalesced.JmxUserSecretRef = clusterTemplate.JmxUserSecretRef
		coalesced.ReaperUiSecretRef = clusterTemplate.ReaperUiSecretRef
	}

	// FIXME do we want to drill down on auto scheduling properties?
	if dcTemplate != nil {
		coalesced.AutoScheduling = dcTemplate.AutoScheduling
	} else if clusterTemplate != nil {
		coalesced.AutoScheduling = clusterTemplate.AutoScheduling
	}

	if dcTemplate != nil && dcTemplate.ReadinessProbe != nil {
		coalesced.ReadinessProbe = dcTemplate.ReadinessProbe
	} else if clusterTemplate != nil && clusterTemplate.ReadinessProbe != nil {
		coalesced.ReadinessProbe = clusterTemplate.ReadinessProbe
	}

	if dcTemplate != nil && dcTemplate.LivenessProbe != nil {
		coalesced.LivenessProbe = dcTemplate.LivenessProbe
	} else if clusterTemplate != nil && clusterTemplate.LivenessProbe != nil {
		coalesced.LivenessProbe = clusterTemplate.LivenessProbe
	}

	if dcTemplate != nil && dcTemplate.Affinity != nil {
		coalesced.Affinity = dcTemplate.Affinity
	} else if clusterTemplate != nil && clusterTemplate.Affinity != nil {
		coalesced.Affinity = clusterTemplate.Affinity
	}

	if dcTemplate != nil && dcTemplate.Tolerations != nil {
		coalesced.Tolerations = dcTemplate.Tolerations
	} else if clusterTemplate != nil && clusterTemplate.Tolerations != nil {
		coalesced.Tolerations = clusterTemplate.Tolerations
	}

	if dcTemplate != nil && dcTemplate.PodSecurityContext != nil {
		coalesced.PodSecurityContext = dcTemplate.PodSecurityContext
	} else if clusterTemplate != nil && clusterTemplate.PodSecurityContext != nil {
		coalesced.PodSecurityContext = clusterTemplate.PodSecurityContext
	}

	if dcTemplate != nil && dcTemplate.SecurityContext != nil {
		coalesced.SecurityContext = dcTemplate.SecurityContext
	} else if clusterTemplate != nil && clusterTemplate.SecurityContext != nil {
		coalesced.SecurityContext = clusterTemplate.SecurityContext
	}

	if dcTemplate != nil && dcTemplate.InitContainerSecurityContext != nil {
		coalesced.InitContainerSecurityContext = dcTemplate.InitContainerSecurityContext
	} else if clusterTemplate != nil && clusterTemplate.InitContainerSecurityContext != nil {
		coalesced.InitContainerSecurityContext = clusterTemplate.InitContainerSecurityContext
	}

	return coalesced
}
