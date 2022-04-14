package medusa

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	pkgtest "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	"inet.af/netaddr"
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
					Rack:       "test-rack1",
				},
				{
					Host:       "192.168.1.3",
					Datacenter: "test-dc1",
					Rack:       "test-rack2",
				},
				{
					Host:       "192.168.1.4",
					Datacenter: "test-dc1",
					Rack:       "test-rack3",
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
	expectedSourceRacks := map[NodeLocation][]netaddr.IP{
		{Rack: "test-rack1", DC: "test-dc2"}: {netaddr.MustParseIP("192.168.1.5")},
		{Rack: "test-rack2", DC: "test-dc2"}: {netaddr.MustParseIP("192.168.1.6")},
		{Rack: "test-rack3", DC: "test-dc2"}: {netaddr.MustParseIP("192.168.1.7")},
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
	expectedSourceRacks := map[NodeLocation][]HostName{
		{Rack: "default", DC: "test-dc2"}: {HostName("test-cluster-test-dc2-default-sts-0"), HostName("test-cluster-test-dc2-default-sts-1"), HostName("test-cluster-test-dc2-default-sts-2")},
	}
	assert.Equal(t, expectedSourceRacks, result)
	kluster.Spec.Cassandra.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	expectedSourceRacks = map[NodeLocation][]HostName{
		{Rack: "rack1", DC: "test-dc2"}: {HostName("test-cluster-test-dc2-rack1-sts-0")},
		{Rack: "rack2", DC: "test-dc2"}: {HostName("test-cluster-test-dc2-rack2-sts-0")},
		{Rack: "rack3", DC: "test-dc2"}: {HostName("test-cluster-test-dc2-rack3-sts-0")},
	}
	result, err = getTargetRackFQDNs(kluster, "test-dc2")
	assert.NoError(t, err, err)
	assert.Equal(t, expectedSourceRacks, result)
}

// func TestGetHostMap(t *testing.T) {

// }
