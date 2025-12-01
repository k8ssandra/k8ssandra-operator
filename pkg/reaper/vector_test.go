package reaper

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestConfigureVector(t *testing.T) {
	telemetrySpec := &telemetryapi.TelemetrySpec{Vector: &telemetryapi.VectorSpec{Enabled: ptr.To(true)}}
	reaper := &api.Reaper{}
	reaper.Spec.Telemetry = telemetrySpec

	template := &corev1.PodTemplateSpec{}
	fakeDc := &cassdcapi.CassandraDatacenter{}

	logger := testlogr.NewTestLogger(t)
	configureVector(reaper, template, fakeDc, logger, getTestImageRegistry(t))

	assert.Equal(t, 1, len(template.Spec.Containers))
	assert.Equal(t, "reaper-vector-agent", template.Spec.Containers[0].Name)
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorCpuLimit), *template.Spec.Containers[0].Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorCpuRequest), *template.Spec.Containers[0].Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorMemoryLimit), *template.Spec.Containers[0].Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorMemoryRequest), *template.Spec.Containers[0].Resources.Requests.Memory())

	// Verify security context settings
	var vectorContainer *corev1.Container
	for i := range template.Spec.Containers {
		if template.Spec.Containers[i].Name == "reaper-vector-agent" {
			vectorContainer = &template.Spec.Containers[i]
			break
		}
	}
	require.NotNil(t, vectorContainer, "vector container not found")
	securityContext := vectorContainer.SecurityContext
	require.NotNil(t, securityContext)
	assert.True(t, *securityContext.RunAsNonRoot)
	assert.Equal(t, int64(1000), *securityContext.RunAsUser)
	assert.True(t, *securityContext.ReadOnlyRootFilesystem)
	assert.False(t, *securityContext.AllowPrivilegeEscalation)
	assert.Equal(t, []corev1.Capability{"ALL"}, securityContext.Capabilities.Drop)
}
