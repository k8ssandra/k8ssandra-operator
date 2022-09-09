package k8ssandra

import (
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/assert"
)

var (
	analyticsWorkloadDc = api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "analytics",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				AnalyticsEnabled: true,
			},
		},
	}

	searchWorkloadDc = api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "search",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				SearchEnabled: true,
			},
		},
	}

	graphWorkloadDc = api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "graph",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				GraphEnabled: true,
			},
		},
	}

	mixedWorkloadDc = api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "mixed",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				GraphEnabled:     true,
				AnalyticsEnabled: true,
			},
		},
	}

	cassandraWorkloadDc = api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "cassandra",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				GraphEnabled:     false,
				AnalyticsEnabled: false,
				SearchEnabled:    false,
			},
		},
	}
)

func TestDatacenters(t *testing.T) {
	t.Run("DcUpgradePriorityTest", dcUpgradePriorityTest)
	t.Run("SortDatacentersForUpgradeTest", sortDatacentersForUpgradeTest)
	t.Run("SortNoChangeTest", sortNoChangeTest)
}

func dcUpgradePriorityTest(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(3, dcUpgradePriority(analyticsWorkloadDc))
	assert.Equal(1, dcUpgradePriority(searchWorkloadDc))
	assert.Equal(2, dcUpgradePriority(graphWorkloadDc))
	assert.Equal(3, dcUpgradePriority(mixedWorkloadDc))
	assert.Equal(2, dcUpgradePriority(cassandraWorkloadDc))
}

func sortDatacentersForUpgradeTest(t *testing.T) {
	assert := assert.New(t)

	datacenters := []api.CassandraDatacenterTemplate{}
	datacenters = append(datacenters, cassandraWorkloadDc)
	datacenters = append(datacenters, analyticsWorkloadDc)
	datacenters = append(datacenters, searchWorkloadDc)
	datacenters = append(datacenters, graphWorkloadDc)
	datacenters = append(datacenters, mixedWorkloadDc)
	sortedDatacenters := sortDatacentersByPriority(datacenters)
	assert.Equal(5, len(sortedDatacenters), "Expected 5 datacenters")
	// The analytics enabled DCs should be first, which includes the mixed workload DC
	assert.Equal("analytics", sortedDatacenters[0].Meta.Name, "Analytics workload DCs should be first")
	assert.Equal("mixed", sortedDatacenters[1].Meta.Name, "Analytics workload DCs should be first")
	// Second and third datacenters should be Cassandra and Graph only
	assert.Equal("cassandra", sortedDatacenters[2].Meta.Name, "Analytics workload DC should be first and search should be last")
	assert.Equal("graph", sortedDatacenters[3].Meta.Name, "Analytics workload DC should be first and search should be last")
	// Search comes last
	assert.Equal("search", sortedDatacenters[4].Meta.Name, "Search workload DC should be last")
}

func sortNoChangeTest(t *testing.T) {
	assert := assert.New(t)

	cassandraDc1 := api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "dc1",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "4.0.0",
		},
	}

	cassandraDc2 := api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "dc2",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "4.0.0",
		},
	}

	cassandraDc3 := api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "dc3",
		},
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "4.0.0",
		},
	}

	datacenters := []api.CassandraDatacenterTemplate{}
	datacenters = append(datacenters, cassandraDc1)
	datacenters = append(datacenters, cassandraDc2)
	datacenters = append(datacenters, cassandraDc3)
	sortedDatacenters := sortDatacentersByPriority(datacenters)
	assert.Equal(3, len(sortedDatacenters), "Expected 3 datacenters")
	assert.Equal("dc1", sortedDatacenters[0].Meta.Name, "Datacenter order should not change")
	assert.Equal("dc2", sortedDatacenters[1].Meta.Name, "Datacenter order should not change")
	assert.Equal("dc3", sortedDatacenters[2].Meta.Name, "Datacenter order should not change")
}
