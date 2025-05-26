package reaper

import (
	"github.com/adutra/goalesce"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	DatacenterAvailabilityEach = "EACH"
	DatacenterAvailabilityAll  = "ALL"
)

// DefaultResourceName generates a name for a new Reaper resource that is derived from the Cassandra cluster and DC
// names.
func DefaultResourceName(dc *cassdcapi.CassandraDatacenter) string {
	return cassdcapi.CleanupForKubernetes(dc.Spec.ClusterName + "-" + dc.DatacenterName() + "-reaper")
}

func NewReaper(
	reaperKey types.NamespacedName,
	kc *k8ssandraapi.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	reaperClusterTemplate *reaperapi.ReaperClusterTemplate,
	logger logr.Logger,
) (*reaperapi.Reaper, error) {
	labels := createResourceLabels(kc)
	var anns map[string]string
	if m := reaperClusterTemplate.ResourceMeta; m != nil {
		labels = utils.MergeMap(labels, m.Labels)
		anns = m.Annotations
	}

	desiredReaper := &reaperapi.Reaper{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   reaperKey.Namespace,
			Name:        reaperKey.Name,
			Annotations: anns,
			Labels:      labels,
		},
		Spec: reaperapi.ReaperSpec{
			ReaperTemplate:         reaperClusterTemplate.ReaperTemplate,
			DatacenterAvailability: computeReaperDcAvailability(kc),
			ClientEncryptionStores: kc.Spec.Cassandra.ClientEncryptionStores,
		},
	}
	if kc.Spec.Reaper != nil {
		if kc.Spec.Reaper.ReaperRef.Name == "" {
			// we only set the DatacenterRef if we do not have an external Reaper referred to via the ReaperRef
			desiredReaper.Spec.DatacenterRef = reaperapi.CassandraDatacenterRef{
				Name:      dc.Name,
				Namespace: dc.Namespace,
			}
		}
	}

	if kc.Spec.IsAuthEnabled() && !kc.Spec.UseExternalSecrets() {
		logger.Info("Auth is enabled, adding user secrets to Reaper spec")
		// if auth is enabled in this cluster, the k8ssandra controller will automatically create two secrets for
		// Reaper: one for CQL and JMX connections, one for the UI. Here we assume that these secrets exist. If the
		// secrets were specified by the user they should be already present in desiredReaper.Spec; otherwise, we assume
		// that the k8ssandra controller created two secrets with default names, and we need to manually fill in this
		// info in desiredReaper.Spec since it wasn't persisted in reaperTemplate.
		if desiredReaper.Spec.CassandraUserSecretRef.Name == "" {
			desiredReaper.Spec.CassandraUserSecretRef.Name = DefaultUserSecretName(kc.SanitizedName())
		}
		// Note: deliberately skip JmxUserSecretRef, which is deprecated.
		if kc.Spec.Reaper.UiUserSecretRef == nil && kc.Spec.IsAuthEnabled() {
			desiredReaper.Spec.UiUserSecretRef = &corev1.LocalObjectReference{Name: DefaultUiSecretName(kc.SanitizedName())}
		}

		if desiredReaper.Spec.ResourceMeta == nil {
			desiredReaper.Spec.ResourceMeta = &meta.ResourceMeta{}
		}

		err := secret.AddInjectionAnnotationReaperContainers(&desiredReaper.Spec.ResourceMeta.Pods, desiredReaper.Spec.CassandraUserSecretRef.Name)
		if err != nil {
			return desiredReaper, err
		}
		if desiredReaper.Spec.UiUserSecretRef != nil && desiredReaper.Spec.UiUserSecretRef.Name != "" {
			err = secret.AddInjectionAnnotationReaperContainers(&desiredReaper.Spec.ResourceMeta.Pods, desiredReaper.Spec.UiUserSecretRef.Name)
			if err != nil {
				return desiredReaper, err
			}
		}
	} else {
		logger.Info("Auth not enabled, no secrets added to Reaper spec")
	}
	// If the cluster is already initialized and some DCs are flagged as stopped, we cannot achieve QUORUM in the
	// cluster for Reaper's keyspace. In this case we simply skip schema migration, otherwise Reaper wouldn't be able to
	// start up.
	if kc.Status.GetConditionStatus(k8ssandraapi.CassandraInitialized) == corev1.ConditionTrue && kc.HasStoppedDatacenters() {
		desiredReaper.Spec.SkipSchemaMigration = true
	}

	// forward common metadata from k8ssandra cluster to reaper
	commonMeta := transformCassandraClusterMeta(kc)
	desiredReaper.Spec.ResourceMeta = goalesce.MustDeepMerge(desiredReaper.Spec.ResourceMeta, commonMeta)

	annotations.AddHashAnnotation(desiredReaper)
	return desiredReaper, nil
}

// See https://cassandra-reaper.io/docs/usage/multi_dc/.
// If we have more than one DC, and each DC has its own Reaper instance, use EACH; otherwise, use ALL.
func computeReaperDcAvailability(kc *k8ssandraapi.K8ssandraCluster) string {
	if kc.Spec.Reaper.DeploymentMode == reaperapi.DeploymentModeSingle || len(kc.Spec.Cassandra.Datacenters) == 1 {
		return DatacenterAvailabilityAll
	}
	return DatacenterAvailabilityEach
}

func transformCassandraClusterMeta(kc *k8ssandraapi.K8ssandraCluster) *meta.ResourceMeta {
	cassMeta := kc.Spec.Cassandra.Meta
	reaperMeta := &meta.ResourceMeta{}

	if cassMeta.Tags.Labels != nil {
		reaperMeta.Tags.Labels = cassMeta.Tags.Labels
	}
	if cassMeta.Tags.Annotations != nil {
		reaperMeta.Tags.Annotations = cassMeta.Tags.Annotations
	}

	if cassMeta.CommonLabels != nil {
		reaperMeta.CommonLabels = cassMeta.CommonLabels
	}
	// cassMeta.CommonAnnotation has nowhere to go

	if cassMeta.Pods.Labels != nil {
		reaperMeta.Pods.Labels = cassMeta.Pods.Labels
	}
	if cassMeta.Pods.Annotations != nil {
		reaperMeta.Pods.Annotations = cassMeta.Pods.Annotations
	}

	// while the CassandraClusterMeta has ServiceConfig field, none of the services therein really fits reaper
	// it seems more correct to forward the common L/A to the reaper service
	if cassMeta.CommonLabels != nil {
		reaperMeta.Service.Labels = cassMeta.CommonLabels
	}
	if cassMeta.CommonAnnotations != nil {
		reaperMeta.Service.Annotations = cassMeta.CommonAnnotations
	}

	return reaperMeta
}
