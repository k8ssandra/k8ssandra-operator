package reaper

import (
	corev1 "k8s.io/api/core/v1"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/vector"
	"github.com/stretchr/testify/assert"
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
	configureVector(reaper, template, fakeDc, logger)

	assert.Equal(t, 1, len(template.Spec.Containers))
	assert.Equal(t, "reaper-vector-agent", template.Spec.Containers[0].Name)
	assert.Equal(t, resource.MustParse(vector.DefaultVectorCpuLimit), *template.Spec.Containers[0].Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse(vector.DefaultVectorCpuRequest), *template.Spec.Containers[0].Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse(vector.DefaultVectorMemoryLimit), *template.Spec.Containers[0].Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse(vector.DefaultVectorMemoryRequest), *template.Spec.Containers[0].Resources.Requests.Memory())
}
