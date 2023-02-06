package k8ssandra

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/stretchr/testify/assert"
)

var (
	analyticsWorkloadDc = &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "analytics",
		},
		ServerVersion: semver.MustParse("6.8.17"),
		DseWorkloads: &cassdcapi.DseWorkloads{
			AnalyticsEnabled: true,
		},
	}

	searchWorkloadDc = &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "search",
		},
		ServerVersion: semver.MustParse("6.8.17"),
		DseWorkloads: &cassdcapi.DseWorkloads{
			SearchEnabled: true,
		},
	}

	graphWorkloadDc = &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "graph",
		},
		ServerVersion: semver.MustParse("6.8.17"),
		DseWorkloads: &cassdcapi.DseWorkloads{
			GraphEnabled: true,
		},
	}

	mixedWorkloadDc = &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "mixed",
		},
		ServerVersion: semver.MustParse("6.8.17"),
		DseWorkloads: &cassdcapi.DseWorkloads{
			GraphEnabled:     true,
			AnalyticsEnabled: true,
		},
	}

	cassandraWorkloadDc = &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "cassandra",
		},
		ServerVersion: semver.MustParse("6.8.17"),
		DseWorkloads: &cassdcapi.DseWorkloads{
			GraphEnabled:     false,
			AnalyticsEnabled: false,
			SearchEnabled:    false,
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

	var datacenters []*cassandra.DatacenterConfig
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

	cassandraDc1 := &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "dc1",
		},
		ServerVersion: semver.MustParse("4.0.4"),
	}

	cassandraDc2 := &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "dc2",
		},
		ServerVersion: semver.MustParse("4.0.4"),
	}

	cassandraDc3 := &cassandra.DatacenterConfig{
		Meta: api.EmbeddedObjectMeta{
			Name: "dc3",
		},
		ServerVersion: semver.MustParse("4.0.4"),
	}

	var datacenters []*cassandra.DatacenterConfig
	datacenters = append(datacenters, cassandraDc1)
	datacenters = append(datacenters, cassandraDc2)
	datacenters = append(datacenters, cassandraDc3)
	sortedDatacenters := sortDatacentersByPriority(datacenters)
	assert.Equal(3, len(sortedDatacenters), "Expected 3 datacenters")
	assert.Equal("dc1", sortedDatacenters[0].Meta.Name, "Datacenter order should not change")
	assert.Equal("dc2", sortedDatacenters[1].Meta.Name, "Datacenter order should not change")
	assert.Equal("dc3", sortedDatacenters[2].Meta.Name, "Datacenter order should not change")
}
