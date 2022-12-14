package reaper

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
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
	return cassdcapi.CleanupForKubernetes(dc.Spec.ClusterName + "-" + dc.Name + "-reaper")
}

func NewReaper(
	reaperKey types.NamespacedName,
	kc *k8ssandraapi.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	reaperTemplate *reaperapi.ReaperClusterTemplate,
) *reaperapi.Reaper {
	labels := createResourceLabels(kc)
	var anns map[string]string
	if m := reaperTemplate.ResourceMeta; m != nil && m.Resource != nil {
		labels = utils.MergeMap(labels, m.Resource.Labels)
		anns = m.Resource.Annotations
	}

	desiredReaper := &reaperapi.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   reaperKey.Namespace,
			Name:        reaperKey.Name,
			Annotations: anns,
			Labels:      labels,
		},
		Spec: reaperapi.ReaperSpec{
			ReaperTemplate: reaperTemplate.ReaperTemplate,
			DatacenterRef: reaperapi.CassandraDatacenterRef{
				Name:      dc.Name,
				Namespace: dc.Namespace,
			},
			DatacenterAvailability: computeReaperDcAvailability(kc),
			ClientEncryptionStores: kc.Spec.Cassandra.ClientEncryptionStores,
		},
	}
	if kc.Spec.IsAuthEnabled() && !kc.Spec.UseExternalSecrets() {
		// if auth is enabled in this cluster, the k8ssandra controller will automatically create two secrets for
		// Reaper: one for CQL and JMX connections, one for the UI. Here we assume that these secrets exist. If the
		// secrets were specified by the user they should be already present in desiredReaper.Spec; otherwise, we assume
		// that the k8ssandra controller created two secrets with default names, and we need to manually fill in this
		// info in desiredReaper.Spec since it wasn't persisted in reaperTemplate.
		if desiredReaper.Spec.CassandraUserSecretRef.Name == "" {
			desiredReaper.Spec.CassandraUserSecretRef.Name = DefaultUserSecretName(kc.SanitizedName())
		}
		// Note: deliberately skip JmxUserSecretRef, which is deprecated.
		if desiredReaper.Spec.UiUserSecretRef.Name == "" {
			desiredReaper.Spec.UiUserSecretRef.Name = DefaultUiSecretName(kc.SanitizedName())
		}
	}
	// If the cluster is already initialized and some DCs are flagged as stopped, we cannot achieve QUORUM in the
	// cluster for Reaper's keyspace. In this case we simply skip schema migration, otherwise Reaper wouldn't be able to
	// start up.
	if kc.Status.GetConditionStatus(k8ssandraapi.CassandraInitialized) == corev1.ConditionTrue && kc.HasStoppedDatacenters() {
		desiredReaper.Spec.SkipSchemaMigration = true
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
