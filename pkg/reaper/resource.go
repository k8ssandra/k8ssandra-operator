package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	DeploymentModeSingle = "SINGLE"
	DeploymentModePerDc  = "PER_DC"

	DatacenterAvailabilityEach = "EACH"
	DatacenterAvailabilityAll  = "ALL"
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
			ReaperTemplate: reaperTemplate.ReaperTemplate,
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
		if desiredReaper.Spec.UiUserSecretRef.Name == "" {
			desiredReaper.Spec.UiUserSecretRef.Name = DefaultUiSecretName(kc.Name)
		}
	}
	annotations.AddHashAnnotation(desiredReaper)
	return desiredReaper
}

// See https://cassandra-reaper.io/docs/usage/multi_dc/.
// If we have more than one DC, and each DC has its own Reaper instance, use EACH; otherwise, use ALL.
func computeReaperDcAvailability(kc *k8ssandraapi.K8ssandraCluster) string {
	if kc.Spec.Reaper.DeploymentMode == DeploymentModeSingle || len(kc.Spec.Cassandra.Datacenters) == 1 {
		return DatacenterAvailabilityAll
	}
	return DatacenterAvailabilityEach
}
