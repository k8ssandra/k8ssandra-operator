package reaper

import (
	"testing"

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
	assert.Equal(t, createServiceAndDeploymentLabels(reaper), service.Labels)

	assert.Equal(t, createServiceAndDeploymentLabels(reaper), service.Spec.Selector)
	assert.Len(t, service.Spec.Ports, 1)

	port := corev1.ServicePort{
		Name:     "app",
		Protocol: corev1.ProtocolTCP,
		Port:     8080,
		TargetPort: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "app",
		},
	}
	assert.Equal(t, port, service.Spec.Ports[0])
}
