package k8ssandra

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
)

var (
	analyticsWorkloadDc = api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				AnalyticsEnabled: true,
			},
		},
	}

	searchWorkloadDc = api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				SearchEnabled: true,
			},
		},
	}

	graphWorkloadDc = api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				GraphEnabled: true,
			},
		},
	}

	mixedWorkloadDc = api.CassandraDatacenterTemplate{
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "6.8.17",
			DseWorkloads: &cassdcapi.DseWorkloads{
				GraphEnabled:     true,
				AnalyticsEnabled: true,
			},
		},
	}

	cassandraWorkloadDc = api.CassandraDatacenterTemplate{
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

func datacentersTest(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
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
	assert.True(sortedDatacenters[0].DseWorkloads.AnalyticsEnabled, "Analytics workload DCs should be first")
	assert.True(sortedDatacenters[1].DseWorkloads.AnalyticsEnabled, "Analytics workload DCs should be first")
	// Second and third datacenters should be Cassandra and Graph only
	assert.False(sortedDatacenters[2].DseWorkloads.AnalyticsEnabled || sortedDatacenters[2].DseWorkloads.SearchEnabled, "Analytics workload DC should be first and search should be last")
	assert.False(sortedDatacenters[3].DseWorkloads.AnalyticsEnabled || sortedDatacenters[3].DseWorkloads.SearchEnabled, "Analytics workload DC should be first and search should be last")
	// Search comes last
	assert.Equal(searchWorkloadDc, sortedDatacenters[4], "Search workload DC should be last")
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
