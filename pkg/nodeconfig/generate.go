package nodeconfig

import (
	"fmt"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"strings"

	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NewDefaultPerNodeConfigMap generates a ConfigMap that contains default per-node configuration
// files for this DC. It returns nil if this DC does not require any per-node configuration.
func NewDefaultPerNodeConfigMap(kcKey types.NamespacedName, dcConfig *cassandra.DatacenterConfig) *corev1.ConfigMap {

	configKey := NewDefaultPerNodeConfigMapKey(kcKey, dcConfig)
	perNodeConfig := newPerNodeConfigMap(kcKey, configKey)

	// append all the default per-node configuration to the ConfigMap;
	// currently, this is just the initial_token option, but we may add more in the future.

	addInitialTokens(dcConfig, perNodeConfig)

	// if any data has been added to the ConfigMap, return it, otherwise, return nil since no
	// per-node configuration is required in this DC.

	if len(perNodeConfig.Data) > 0 {
		return perNodeConfig
	}

	return nil
}

func NewDefaultPerNodeConfigMapKey(kcKey types.NamespacedName, dcConfig *cassandra.DatacenterConfig) types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s-per-node-config", kcKey.Name, dcConfig.Meta.Name),
		Namespace: utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kcKey.Namespace),
	}
}

func newPerNodeConfigMap(kcKey, configKey types.NamespacedName) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configKey.Name,
			Namespace: configKey.Namespace,
			Labels: labels.MapOf(
				labels.CassandraCommon,
				labels.CleanedUpByK8ssandraCluster(kcKey),
			),
		},
	}
}

func addInitialTokens(dcConfig *cassandra.DatacenterConfig, perNodeConfig *corev1.ConfigMap) {
	for podName, initialTokens := range dcConfig.InitialTokensByPodName {
		entryKey := podName + "_cassandra.yaml"
		entryValue := fmt.Sprintf("initial_token: %s", strings.Join(initialTokens, ","))
		if perNodeConfig.Data == nil {
			perNodeConfig.Data = make(map[string]string)
		}
		perNodeConfig.Data[entryKey] = perNodeConfig.Data[entryKey] + "\n" + entryValue
	}
}
