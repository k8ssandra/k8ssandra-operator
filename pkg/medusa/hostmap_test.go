package medusa

import (
	"context"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	pkgtest "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockgRPCClient struct{}

func (client mockgRPCClient) GetBackups(ctx context.Context) ([]*BackupSummary, error) {
	return []*BackupSummary{
		{
			BackupName: "remote-backupname",
			Nodes: []*BackupNode{
				{
					Host:       "test-cluster-test-dc1-default-sts-0",
					Datacenter: "test-dc1",
					Rack:       "default",
				},
				{
					Host:       "test-cluster-test-dc1-default-sts-2",
					Datacenter: "test-dc1",
					Rack:       "default",
				},
				{
					Host:       "test-cluster-test-dc1-default-sts-1",
					Datacenter: "test-dc1",
					Rack:       "default",
				},
				{
					Host:       "test-cluster-test-dc2-test-rack1-sts-0",
					Datacenter: "test-dc2",
					Rack:       "test-rack1",
				},
				{
					Host:       "test-cluster-test-dc2-test-rack3-sts-0",
					Datacenter: "test-dc2",
					Rack:       "test-rack3",
				},
				{
					Host:       "test-cluster-test-dc2-test-rack2-sts-0",
					Datacenter: "test-dc2",
					Rack:       "test-rack2",
				},
			},
		},
	}, nil
}

func (c mockgRPCClient) Close() error {
	return nil
}

func (c mockgRPCClient) CreateBackup(ctx context.Context, name string, backupType string) (*BackupResponse, error) {
	return nil, nil
}

func (c mockgRPCClient) PurgeBackups(ctx context.Context) (*PurgeBackupsResponse, error) {
	return nil, nil
}

func (c mockgRPCClient) BackupStatus(ctx context.Context, name string) (*BackupStatusResponse, error) {
	return nil, nil
}

func (c mockgRPCClient) PrepareRestore(ctx context.Context, datacenter, backupName, restoreKey string) (*PrepareRestoreResponse, error) {
	return nil, nil
}

func TestGetSourceRacksIPs(t *testing.T) {
	mockgRPCClient := mockgRPCClient{}
	medusaBackup := pkgtest.NewMedusaRestore("default", "local-backupname", "remote-backupname", "test-dc2", "test-cluster")
	ctx := context.Background()
	sourceRacks, err := getSourceRacksIPs(*medusaBackup, mockgRPCClient, ctx, "test-dc2")
	assert.NoError(t, err, err)
	expectedSourceRacks := map[NodeLocation][]string{
		{Rack: "test-rack1", DC: "test-dc2"}: {"test-cluster-test-dc2-test-rack1-sts-0"},
		{Rack: "test-rack2", DC: "test-dc2"}: {"test-cluster-test-dc2-test-rack2-sts-0"},
		{Rack: "test-rack3", DC: "test-dc2"}: {"test-cluster-test-dc2-test-rack3-sts-0"},
	}
	assert.Equal(t, expectedSourceRacks, sourceRacks)
}

func TestGetTargetRackFQDNs(t *testing.T) {
	cassDc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc2",
			Namespace: "default",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:    "test-cluster",
			DatacenterName: "test-dc2",
			Size:           3,
		},
	}

	result, err := getTargetRackFQDNs(cassDc)
	assert.NoError(t, err, err)
	expectedSourceRacks := map[NodeLocation][]string{
		{Rack: "default", DC: "test-dc2"}: {"test-cluster-test-dc2-default-sts-0", "test-cluster-test-dc2-default-sts-1", "test-cluster-test-dc2-default-sts-2"},
	}
	assert.Equal(t, expectedSourceRacks, result)
	cassDc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	expectedSourceRacks = map[NodeLocation][]string{
		{Rack: "rack1", DC: "test-dc2"}: {"test-cluster-test-dc2-rack1-sts-0"},
		{Rack: "rack2", DC: "test-dc2"}: {"test-cluster-test-dc2-rack2-sts-0"},
		{Rack: "rack3", DC: "test-dc2"}: {"test-cluster-test-dc2-rack3-sts-0"},
	}
	result, err = getTargetRackFQDNs(cassDc)
	assert.NoError(t, err, err)
	assert.Equal(t, expectedSourceRacks, result)
}

func TestGetTargetRackFQDNsOverrides(t *testing.T) {
	cassDc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dc2",
			Namespace: "default",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName:    "Test Cluster",
			Size:           3,
			DatacenterName: "Test DC2",
		},
	}

	result, err := getTargetRackFQDNs(cassDc)
	assert.NoError(t, err, err)
	expectedSourceRacks := map[NodeLocation][]string{
		{Rack: "default", DC: "testdc2"}: {"testcluster-testdc2-default-sts-0", "testcluster-testdc2-default-sts-1", "testcluster-testdc2-default-sts-2"},
	}
	assert.Equal(t, expectedSourceRacks, result)
	cassDc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	expectedSourceRacks = map[NodeLocation][]string{
		{Rack: "rack1", DC: "testdc2"}: {"testcluster-testdc2-rack1-sts-0"},
		{Rack: "rack2", DC: "testdc2"}: {"testcluster-testdc2-rack2-sts-0"},
		{Rack: "rack3", DC: "testdc2"}: {"testcluster-testdc2-rack3-sts-0"},
	}
	result, err = getTargetRackFQDNs(cassDc)
	assert.NoError(t, err, err)
	assert.Equal(t, expectedSourceRacks, result)
}

func TestGetHostMap(t *testing.T) {
	// Fixtures
	mockgRPCClient := mockgRPCClient{}
	ctx := context.Background()

	cassDc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dc1",
			Namespace: "default",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName: "test-cluster",
			Size:        3,
		},
	}
	/////////////////////////////////////////////// Test with all nodes in one rack. /////////////////////////////////////////////////
	// Using DC = "test-dc-1" here because that's the DC we defined in the fixture above which has all nodes in a single rack.
	medusaBackup := pkgtest.NewMedusaRestore("default", "local-backupname", "remote-backupname", "test-dc1", "test-cluster")
	result, err := GetHostMap(cassDc, *medusaBackup, mockgRPCClient, ctx)
	assert.NoError(t, err, err)
	expected := HostMappingSlice{
		{
			Source: "test-cluster-test-dc1-default-sts-0",
			Target: "test-cluster-test-dc1-default-sts-0",
		},
		{
			Source: "test-cluster-test-dc1-default-sts-1",
			Target: "test-cluster-test-dc1-default-sts-1",
		},
		{
			Source: "test-cluster-test-dc1-default-sts-2",
			Target: "test-cluster-test-dc1-default-sts-2",
		},
	}
	assert.Equal(t, expected, result)
	///////////////////////////////////////////// Test with nodes split over three racks. ////////////////////////////////////////////
	// Define racks on DC
	cassDc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	// Make DC name = "test-dc2" which is our test DC which has racks.
	medusaBackup = pkgtest.NewMedusaRestore("default", "local-backupname", "remote-backupname", "test-dc2", "test-cluster")
	cassDc.ObjectMeta.Name = "test-dc2"
	expected = HostMappingSlice{
		{
			Source: "test-cluster-test-dc2-test-rack1-sts-0",
			Target: "test-cluster-test-dc2-rack1-sts-0",
		},
		{
			Source: "test-cluster-test-dc2-test-rack2-sts-0",
			Target: "test-cluster-test-dc2-rack2-sts-0",
		},
		{
			Source: "test-cluster-test-dc2-test-rack3-sts-0",
			Target: "test-cluster-test-dc2-rack3-sts-0",
		},
	}
	result, err = GetHostMap(cassDc, *medusaBackup, mockgRPCClient, ctx)
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
