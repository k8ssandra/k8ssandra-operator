package reaper

import (
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestNewService(t *testing.T) {
	reaper := newTestReaper()
	key := types.NamespacedName{Namespace: reaper.Namespace, Name: GetServiceName(reaper.Name)}

	service := NewService(key, reaper)

	assert.Equal(t, key.Name, service.Name)
	assert.Equal(t, key.Namespace, service.Namespace)

	commonLabels := createServiceAndDeploymentLabels(reaper)
	labels := utils.MergeMap(reaper.Spec.ReaperTemplate.ResourceMeta.Service.Labels, commonLabels)

	assert.Equal(t, labels, service.Labels)

	assert.Equal(t, commonLabels, service.Spec.Selector)
	assert.Len(t, service.Spec.Ports, 2)

	appPort := corev1.ServicePort{
		Name:     "app",
		Protocol: corev1.ProtocolTCP,
		Port:     8080,
		TargetPort: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "app",
		},
	}
	adminPort := corev1.ServicePort{
		Name:     "admin",
		Protocol: corev1.ProtocolTCP,
		Port:     8081,
		TargetPort: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "admin",
		},
	}
	assert.Contains(t, service.Spec.Ports, appPort)
	assert.Contains(t, service.Spec.Ports, adminPort)
}
