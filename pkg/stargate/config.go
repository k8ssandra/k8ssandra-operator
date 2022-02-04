package stargate

import (
	"strings"

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

func CreateStargateConfigMap(namespace, configYaml, clusterName, dcName string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GeneratedConfigMapName(clusterName, dcName),
			Namespace: namespace,
			Labels: map[string]string{
				k8ssandra.NameLabel:                      k8ssandra.NameLabelValue,
				k8ssandra.PartOfLabel:                    k8ssandra.PartOfLabelValue,
				k8ssandra.ComponentLabel:                 k8ssandra.ComponentLabelValueStargate,
				k8ssandra.CreatedByLabel:                 k8ssandra.CreatedByLabelValueK8ssandraClusterController,
				k8ssandra.K8ssandraClusterNameLabel:      clusterName,
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
