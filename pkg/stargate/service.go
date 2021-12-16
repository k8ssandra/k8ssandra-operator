package stargate

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
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
				v1alpha1.NameLabel:      v1alpha1.NameLabelValue,
				v1alpha1.PartOfLabel:    v1alpha1.PartOfLabelValue,
				v1alpha1.ComponentLabel: v1alpha1.ComponentLabelValueStargate,
				v1alpha1.CreatedByLabel: v1alpha1.CreatedByLabelValueStargateController,
				api.StargateLabel:       stargate.Name,
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

	klusterName, nameFound := stargate.Labels[v1alpha1.K8ssandraClusterNameLabel]
	klusterNamespace, namespaceFound := stargate.Labels[v1alpha1.K8ssandraClusterNamespaceLabel]

	if nameFound && namespaceFound {
		service.Labels[v1alpha1.K8ssandraClusterNameLabel] = klusterName
		service.Labels[v1alpha1.K8ssandraClusterNamespaceLabel] = klusterNamespace
	}
	service.Annotations[v1alpha1.ResourceHashAnnotation] = utils.DeepHashString(service)
	return service
}
