package reaper

import (
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetServiceName(reaperName string) string {
	return reaperName + "-service"
}

func NewService(key types.NamespacedName, reaper *api.Reaper) *corev1.Service {
	commonLabels := createServiceAndDeploymentLabels(reaper)

	var serviceLabels, serviceAnnotations map[string]string
	if meta := reaper.Spec.ResourceMeta; meta != nil {
		serviceLabels = utils.MergeMap(commonLabels, meta.Service.Labels)
		serviceAnnotations = meta.Service.Annotations
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        key.Name,
			Namespace:   key.Namespace,
			Labels:      serviceLabels,
			Annotations: serviceAnnotations,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Port:     8080,
				Name:     "app",
				Protocol: corev1.ProtocolTCP,
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "app",
				},
			}, {
				Port:     8081,
				Name:     "admin",
				Protocol: corev1.ProtocolTCP,
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "admin",
				},
			}},
			Selector: getConstantLabels(reaper),
		},
	}
	annotations.AddHashAnnotation(service)
	return service
}
