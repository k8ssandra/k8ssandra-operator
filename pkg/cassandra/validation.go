package cassandra

import (
	"fmt"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
)

func ValidateDcTemplate(dcTemplate api.CassandraDatacenterTemplate) error {
	if dcTemplate.DatacenterOptions.AdditionalContainers != nil {
		for _, container := range dcTemplate.DatacenterOptions.AdditionalContainers {
			if container.Name == "medusa" {
				return fmt.Errorf("medusa container is not allowed in cassandra datacenter template")
			}
		}
	}

	if dcTemplate.DatacenterOptions.AdditionalInitContainers != nil {
		for _, container := range dcTemplate.DatacenterOptions.AdditionalInitContainers {
			if container.Name == "jmx-credentials" {
				return fmt.Errorf("jmx-credentials container is not allowed in cassandra datacenter template")
			}
			if container.Name == "medusa-restore" {
				return fmt.Errorf("medusa-restore container is not allowed in cassandra datacenter template")
			}
		}
	}
	return nil
}
