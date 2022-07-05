package cassandra

import (
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestInvalidTemplateMedusaContainer(t *testing.T) {
	dcTemplate := api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			AdditionalContainers: []corev1.Container{
				{
					Name: "medusa",
				},
			},
		},
	}

	assert.Error(t, ValidateDcTemplate(dcTemplate))
}

func TestInvalidTemplateMedusaInitContainer(t *testing.T) {
	dcTemplate := api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			AdditionalInitContainers: []corev1.Container{
				{
					Name: "medusa-restore",
				},
			},
		},
	}

	assert.Error(t, ValidateDcTemplate(dcTemplate))
}

func TestInvalidTemplateJmxCredsInitContainer(t *testing.T) {
	dcTemplate := api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			AdditionalInitContainers: []corev1.Container{
				{
					Name: "jmx-credentials",
				},
			},
		},
	}

	assert.Error(t, ValidateDcTemplate(dcTemplate))
}

func TestValidTemplate(t *testing.T) {
	dcTemplate := api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			AdditionalInitContainers: []corev1.Container{
				{
					Name: "custom-init-container1",
				},
				{
					Name: "custom-init-container2",
				},
			},
			AdditionalContainers: []corev1.Container{
				{
					Name: "custom-container1",
				},
				{
					Name: "custom-container2",
				},
			},
		},
	}

	assert.NoError(t, ValidateDcTemplate(dcTemplate))
}
