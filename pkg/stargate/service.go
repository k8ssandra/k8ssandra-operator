package stargate

import (
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewService creates a Service object for the given Stargate and CassandraDatacenter
// resources.
func NewService(stargate *api.Stargate, dc *cassdcapi.CassandraDatacenter) *corev1.Service {
	serviceName := dc.Spec.ClusterName + "-" + dc.Name + "-stargate-service"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   stargate.Namespace,
			Annotations: map[string]string{},
			Labels: map[string]string{
				api.NameLabel:      api.NameLabelValue,
				api.PartOfLabel:    api.PartOfLabelValue,
				api.ComponentLabel: api.ComponentLabelValueStargate,
				api.CreatedByLabel: api.CreatedByLabelValueStargateController,
				api.StargateLabel:  stargate.Name,
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
	service.Annotations[api.ResourceHashAnnotation] = utils.DeepHashString(service)
	return service
}
