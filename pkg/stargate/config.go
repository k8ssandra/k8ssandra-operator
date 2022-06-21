package stargate

import (
	"strings"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandra "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FilterYamlConfig(config map[string]interface{}) map[string]interface{} {
	allowedConfigSettings := []string{"server_encryption_options", "client_encryption_options"}
	filteredConfig := make(map[string]interface{})
	for k, v := range config {
		// check if the key is allowed
		if utils.SliceContains(allowedConfigSettings, k) {
			filteredConfig[k] = v
		}
	}
	return filteredConfig
}

func CreateStargateConfigMap(namespace string, configYaml string, dc cassdcapi.CassandraDatacenter) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GeneratedConfigMapName(dc.Spec.ClusterName, dc.Name),
			Namespace: namespace,
			Labels: map[string]string{
				k8ssandra.NameLabel:                      k8ssandra.NameLabelValue,
				k8ssandra.PartOfLabel:                    k8ssandra.PartOfLabelValue,
				k8ssandra.ComponentLabel:                 k8ssandra.ComponentLabelValueStargate,
				k8ssandra.CreatedByLabel:                 k8ssandra.CreatedByLabelValueK8ssandraClusterController,
				k8ssandra.K8ssandraClusterNameLabel:      dc.Labels[k8ssandra.K8ssandraClusterNameLabel],
				k8ssandra.K8ssandraClusterNamespaceLabel: namespace,
			},
		},
		Data: map[string]string{
			"cassandra.yaml": configYaml,
		},
	}
}

func MergeConfigMaps(userConfigMap string, generatedConfigMap string) string {
	if userConfigMap != "" {
		separator := "\n"
		if strings.HasSuffix(userConfigMap, "\n") {
			separator = ""
		}
		return userConfigMap + separator + generatedConfigMap
	}

	return generatedConfigMap
}
