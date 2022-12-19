package stargate

import (
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func ResourceName(dc *cassdcapi.CassandraDatacenter) string {
	return cassdcapi.CleanupForKubernetes(dc.Spec.ClusterName + "-" + dc.Name + "-stargate")
}

func ServiceName(dc *cassdcapi.CassandraDatacenter) string {
	return cassdcapi.CleanupForKubernetes(dc.Spec.ClusterName + "-" + dc.Name + "-stargate-service")
}

func DeploymentName(dc *cassdcapi.CassandraDatacenter, rack *cassdcapi.Rack) string {
	return cassdcapi.CleanupForKubernetes(dc.Spec.ClusterName + "-" + dc.Name + "-" + rack.Name + "-stargate-deployment")
}

func NewStargate(
	stargateKey types.NamespacedName,
	kc *api.K8ssandraCluster,
	stargateTemplate *stargateapi.StargateDatacenterTemplate,
	actualDc *cassdcapi.CassandraDatacenter,
	dcTemplate api.CassandraDatacenterTemplate,
	logger logr.Logger,
) *stargateapi.Stargate {

	cassandraEncryption := stargateapi.CassandraEncryption{}
	dcConfig := cassandra.Coalesce(kc.SanitizedName(), kc.Spec.Cassandra, &dcTemplate)

	if cassandra.ClientEncryptionEnabled(dcConfig) && !stargateTemplate.UseExternalSecrets() {
		logger.Info("Client encryption enabled, setting it up in Stargate")
		cassandraEncryption.ClientEncryptionStores = kc.Spec.Cassandra.ClientEncryptionStores
	}

	if cassandra.ServerEncryptionEnabled(dcConfig) && !stargateTemplate.UseExternalSecrets() {
		logger.Info("Server encryption enabled, setting it up in Stargate")
		cassandraEncryption.ServerEncryptionStores = kc.Spec.Cassandra.ServerEncryptionStores
	}

	labels := map[string]string{
		api.NameLabel:                      api.NameLabelValue,
		api.PartOfLabel:                    api.PartOfLabelValue,
		api.ComponentLabel:                 api.ComponentLabelValueStargate,
		api.CreatedByLabel:                 api.CreatedByLabelValueK8ssandraClusterController,
		api.K8ssandraClusterNameLabel:      kc.Name,
		api.K8ssandraClusterNamespaceLabel: kc.Namespace,
	}

	var annotations map[string]string
	if m := stargateTemplate.ResourceMeta; m != nil {
		labels = utils.MergeMap(labels, m.Resource.Labels)
		annotations = m.Resource.Annotations
	}

	desiredStargate := &stargateapi.Stargate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   stargateKey.Namespace,
			Name:        stargateKey.Name,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: stargateapi.StargateSpec{
			StargateDatacenterTemplate: *stargateTemplate,
			DatacenterRef:              corev1.LocalObjectReference{Name: actualDc.Name},
			Auth:                       kc.Spec.Auth,
			CassandraEncryption:        &cassandraEncryption,
		},
	}

	return desiredStargate
}
