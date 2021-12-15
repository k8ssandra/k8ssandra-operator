package reaper

import (
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
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
	labels := createServiceAndDeploymentLabels(reaper)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    labels,
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
			}},
			Selector: labels,
		},
	}
	utils.AddHashAnnotation(service)
	return service
}
