package stargate

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewService creates a Service object for the given Stargate and CassandraDatacenter
// resources.
func NewService(stargate *api.Stargate, dc *cassdcapi.CassandraDatacenter) *corev1.Service {
	serviceName := ServiceName(dc)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   stargate.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				utils.NameLabel:      utils.NameLabelValue,
				utils.PartOfLabel:    utils.PartOfLabelValue,
				utils.ComponentLabel: utils.ComponentLabelValueStargate,
				utils.CreatedByLabel: utils.CreatedByLabelValueStargateController,
				api.StargateLabel:    stargate.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{Port: 8080, Name: "graphql"},
				{Port: 8081, Name: "authorization"},
				{Port: 8082, Name: "rest"},
				{Port: 8084, Name: "health"},
				{Port: 8085, Name: "metrics"},
				{Port: 9042, Name: "cassandra"},
			},
			Selector: map[string]string{
				api.StargateLabel: stargate.Name,
			},
		},
	}

	klusterName, nameFound := stargate.Labels[utils.K8ssandraClusterNameLabel]
	klusterNamespace, namespaceFound := stargate.Labels[utils.K8ssandraClusterNamespaceLabel]

	if nameFound && namespaceFound {
		service.Labels[utils.K8ssandraClusterNameLabel] = klusterName
		service.Labels[utils.K8ssandraClusterNamespaceLabel] = klusterNamespace
	}
	service.Annotations[utils.ResourceHashAnnotation] = utils.DeepHashString(service)
	return service
}
