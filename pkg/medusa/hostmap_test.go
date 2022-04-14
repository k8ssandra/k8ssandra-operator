package medusa

import (
	"context"
	"testing"

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
