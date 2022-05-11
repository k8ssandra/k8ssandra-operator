package medusa

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	pkgtest "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
)

type mockgRPCClient struct{}

func (client mockgRPCClient) GetBackups(ctx context.Context) ([]*BackupSummary, error) {
	return []*BackupSummary{
		{
			BackupName: "remote-backupname",
			Nodes: []*BackupNode{
				{
					Host:       "192.168.1.2",
					Datacenter: "test-dc1",
					Rack:       "default",
				},
				{
					Host:       "192.168.1.3",
					Datacenter: "test-dc1",
					Rack:       "default",
				},
				{
					Host:       "192.168.1.4",
					Datacenter: "test-dc1",
					Rack:       "default",
				},
				{
					Host:       "192.168.1.5",
					Datacenter: "test-dc2",
					Rack:       "test-rack1",
				},
				{
					Host:       "192.168.1.6",
					Datacenter: "test-dc2",
					Rack:       "test-rack2",
				},
				{
					Host:       "192.168.1.7",
					Datacenter: "test-dc2",
					Rack:       "test-rack3",
				},
			},
		},
	}, nil
}

func TestGetSourceRacksIPs(t *testing.T) {
	mockgRPCClient := mockgRPCClient{}
	medusaBackup := pkgtest.NewMedusaRestore("default", "local-backupname", "remote-backupname", "test-dc2", "test-cluster")
	ctx := context.Background()
	sourceRacks, err := getSourceRacksIPs(*medusaBackup, mockgRPCClient, ctx)
	assert.NoError(t, err, err)
	expectedSourceRacks := map[NodeLocation][]string{
		{Rack: "test-rack1", DC: "test-dc2"}: {"192.168.1.5"},
		{Rack: "test-rack2", DC: "test-dc2"}: {"192.168.1.6"},
		{Rack: "test-rack3", DC: "test-dc2"}: {"192.168.1.7"},
	}
	assert.Equal(t, expectedSourceRacks, sourceRacks)
}

func TestGetTargetRackFQDNs(t *testing.T) {
	kluster := pkgtest.NewK8ssandraCluster("test-cluster", "default")
	kluster.Spec.Cassandra.Datacenters = []k8ssandraapi.CassandraDatacenterTemplate{
		{
			Meta: k8ssandraapi.EmbeddedObjectMeta{
				Name:      "test-dc2",
				Namespace: "default",
			},
			Size: 3,
		},
	}
	result, err := getTargetRackFQDNs(kluster, "test-dc2")
	assert.NoError(t, err, err)
	expectedSourceRacks := map[NodeLocation][]string{
		{Rack: "default", DC: "test-dc2"}: {"test-cluster-test-dc2-default-sts-0", "test-cluster-test-dc2-default-sts-1", "test-cluster-test-dc2-default-sts-2"},
	}
	assert.Equal(t, expectedSourceRacks, result)
	kluster.Spec.Cassandra.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	expectedSourceRacks = map[NodeLocation][]string{
		{Rack: "rack1", DC: "test-dc2"}: {"test-cluster-test-dc2-rack1-sts-0"},
		{Rack: "rack2", DC: "test-dc2"}: {"test-cluster-test-dc2-rack2-sts-0"},
		{Rack: "rack3", DC: "test-dc2"}: {"test-cluster-test-dc2-rack3-sts-0"},
	}
	result, err = getTargetRackFQDNs(kluster, "test-dc2")
	assert.NoError(t, err, err)
	assert.Equal(t, expectedSourceRacks, result)
}

func TestGetHostMap(t *testing.T) {
	// Fixtures
	mockgRPCClient := mockgRPCClient{}
	ctx := context.Background()
	kluster := pkgtest.NewK8ssandraCluster("test-cluster", "default")
	kluster.Spec.Cassandra.Datacenters = []k8ssandraapi.CassandraDatacenterTemplate{
		{
			Meta: k8ssandraapi.EmbeddedObjectMeta{
				Name:      "test-dc1",
				Namespace: "default",
			},
			Size: 3,
		},
	}
	/////////////////////////////////////////////// Test with all nodes in one rack. /////////////////////////////////////////////////
	// Using DC = "test-dc-1" here because that's the DC we defined in the fixture above which has all nodes in a single rack.
	medusaBackup := pkgtest.NewMedusaRestore("default", "local-backupname", "remote-backupname", "test-dc1", "test-cluster")
	result, err := GetHostMap(kluster, *medusaBackup, mockgRPCClient, ctx)
	assert.NoError(t, err, err)
	expected := HostMappingSlice{
		{
			Source: "192.168.1.2",
			Target: "test-cluster-test-dc1-default-sts-0",
		},
		{
			Source: "192.168.1.3",
			Target: "test-cluster-test-dc1-default-sts-1",
		},
		{
			Source: "192.168.1.4",
			Target: "test-cluster-test-dc1-default-sts-2",
		},
	}
	assert.Equal(t, expected, result)
	///////////////////////////////////////////// Test with nodes split over three racks. ////////////////////////////////////////////
	// Define racks on DC
	kluster.Spec.Cassandra.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	// Make DC name = "test-dc2" which is our test DC which has racks.
	kluster.Spec.Cassandra.Datacenters[0].Meta.Name = "test-dc2"
	expected = HostMappingSlice{
		{
			Source: "192.168.1.5",
			Target: "test-cluster-test-dc2-rack1-sts-0",
		},
		{
			Source: "192.168.1.6",
			Target: "test-cluster-test-dc2-rack2-sts-0",
		},
		{
			Source: "192.168.1.7",
			Target: "test-cluster-test-dc2-rack3-sts-0",
		},
	}
	result, err = GetHostMap(kluster, *medusaBackup, mockgRPCClient, ctx)
	assert.NoError(t, err, err)
	assert.Equal(t, expected, result)
}

func Test_HostMappingSlice_IsInPlace(t *testing.T) {
	s := HostMappingSlice{
		{Source: "1", Target: "1"},
		{Source: "2", Target: "2"},
		{Source: "3", Target: "3"},
		{Source: "4", Target: "4"},
	}
	res, err := s.IsInPlace()
	assert.NoError(t, err, err)
	assert.Equal(t, true, res)
	s = HostMappingSlice{
		{Source: "1", Target: ""},
	}
	res, err = s.IsInPlace()
	assert.Error(t, err, err)
}
