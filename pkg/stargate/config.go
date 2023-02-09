package stargate

import (
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandra "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CqlConfigName = "stargate-cql.yaml"

var CassandraYamlRetainedSettings = []string{"server_encryption_options"}
var CqlYamlRetainedSettings = []string{
	"rpc_keepalive",
	"native_transport_max_frame_size_in_mb",
	"native_transport_max_concurrent_connections",
	"native_transport_max_concurrent_connections_per_ip",
	"native_transport_flush_in_batches_legacy",
	"native_transport_allow_older_protocols",
	"native_transport_max_concurrent_requests_in_bytes_per_ip",
	"native_transport_max_concurrent_requests_in_bytes",
	"native_transport_idle_timeout_in_ms",
	"client_encryption_options",
}

func FilterConfig(config map[string]interface{}, retainedSettings []string) map[string]interface{} {
	filteredConfig := make(map[string]interface{})
	for k, v := range config {
		// check if the key is allowed
		if utils.SliceContains(retainedSettings, k) {
			filteredConfig[k] = v
		}
	}
	return filteredConfig
}

func CreateStargateConfigMap(namespace, cassandraYaml, stargateCqlYaml string, dc cassdcapi.CassandraDatacenter) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GeneratedConfigMapName(dc.Spec.ClusterName, dc.Name),
			Namespace: namespace,
			Labels: labels.MapOf(
				labels.StargateCommon,
				labels.CleanedUpByK8ssandraCluster(client.ObjectKey{
					Namespace: namespace,
					Name:      dc.Labels[k8ssandra.K8ssandraClusterNameLabel]}),
			),
		},
		Data: map[string]string{
			"cassandra.yaml": cassandraYaml,
			CqlConfigName:    stargateCqlYaml,
		},
	}
}

func MergeYamlString(userConfigMap string, generatedConfigMap string) string {
	if userConfigMap != "" {
		separator := "\n"
		if strings.HasSuffix(userConfigMap, "\n") {
			separator = ""
		}
		return userConfigMap + separator + generatedConfigMap
	}

	return generatedConfigMap
}
