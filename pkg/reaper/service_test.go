package reaper

import (
	"testing"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
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

	selectorLabels := map[string]string{
		k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		reaperapi.ReaperLabel:       reaper.Name,
	}

	serviceLabels := map[string]string{
		k8ssandraapi.NameLabel:      k8ssandraapi.NameLabelValue,
		k8ssandraapi.PartOfLabel:    k8ssandraapi.PartOfLabelValue,
		k8ssandraapi.ComponentLabel: k8ssandraapi.ComponentLabelValueReaper,
		k8ssandraapi.ManagedByLabel: k8ssandraapi.NameLabelValue,
		reaperapi.ReaperLabel:       reaper.Name,
		"common":                    "everywhere",
		"service-label":             "service-label-value",
		"override":                  "serviceLevel",
	}

	assert.Equal(t, serviceLabels, service.Labels)

	assert.Equal(t, selectorLabels, service.Spec.Selector)
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
