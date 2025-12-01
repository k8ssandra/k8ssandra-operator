package stargate

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	cassimages "github.com/k8ssandra/cass-operator/pkg/images"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestConfigureVector(t *testing.T) {
	telemetrySpec := &telemetryapi.TelemetrySpec{Vector: &telemetryapi.VectorSpec{Enabled: ptr.To(true)}}
	stargate := &api.Stargate{}
	stargate.Spec.Telemetry = telemetrySpec

	deployment := &v1.Deployment{}
	fakeDc := &cassdcapi.CassandraDatacenter{}

	logger := testlogr.NewTestLogger(t)
	configureVector(stargate, deployment, fakeDc, logger, getTestImageRegistry())

	assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Containers))
	assert.Equal(t, "stargate-vector-agent", deployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorCpuLimit), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorCpuRequest), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorMemoryLimit), *deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse(telemetry.DefaultVectorMemoryRequest), *deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory())
}

var (
	regOnce           sync.Once
	imageRegistryTest cassimages.ImageRegistry
)

func getTestImageRegistry() cassimages.ImageRegistry {
	regOnce.Do(func() {
		p := filepath.Clean("../../test/testdata/imageconfig/image_config_test.yaml")
		data, err := os.ReadFile(p)
		if err == nil {
			if r, e := cassimages.NewImageRegistryV2(data); e == nil {
				imageRegistryTest = r
			}
		}
	})
	return imageRegistryTest
}
