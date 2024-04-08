package nodeconfig

import (
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"strings"

	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NewDefaultPerNodeConfigMap generates a ConfigMap that contains default per-node configuration
// files for this DC. It returns nil if this DC does not require any per-node configuration.
func NewDefaultPerNodeConfigMap(kcKey types.NamespacedName, kc *k8ssandraapi.K8ssandraCluster, dcConfig *cassandra.DatacenterConfig) *corev1.ConfigMap {

	configKey := NewDefaultPerNodeConfigMapKey(kc, dcConfig)
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

func NewDefaultPerNodeConfigMapKey(kc *k8ssandraapi.K8ssandraCluster, dcConfig *cassandra.DatacenterConfig) types.NamespacedName {
	return types.NamespacedName{
		Name:      cassdcapi.CleanupForKubernetes(kc.CassClusterName() + "-" + dcConfig.CassDcName() + "-per-node-config"),
		Namespace: utils.FirstNonEmptyString(dcConfig.Meta.Namespace, kc.Namespace),
	}
}

func newPerNodeConfigMap(kcKey, configKey types.NamespacedName) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configKey.Name,
			Namespace: configKey.Namespace,
			Labels: utils.MergeMap(
				map[string]string{
					k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
					k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
					k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueCassandra,
				},
				labels.CleanedUpByLabels(kcKey)),
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
