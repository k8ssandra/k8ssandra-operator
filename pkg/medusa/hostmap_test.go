package medusa

import (
	"context"
	"errors"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	pkgtest "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

func (c mockgRPCClient) PreparePitrRestore(ctx context.Context, datacenter, targetTimestamp string) (*PreparePitrRestoreResponse, error) {
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

	result := getTargetRackFQDNs(cassDc)
	expectedSourceRacks := map[NodeLocation][]string{
		{Rack: "default", DC: "test-dc2"}: {"test-cluster-dc2-default-sts-0", "test-cluster-dc2-default-sts-1", "test-cluster-dc2-default-sts-2"},
	}
	assert.Equal(t, expectedSourceRacks, result)
	cassDc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	expectedSourceRacks = map[NodeLocation][]string{
		{Rack: "rack1", DC: "test-dc2"}: {"test-cluster-dc2-rack1-sts-0"},
		{Rack: "rack2", DC: "test-dc2"}: {"test-cluster-dc2-rack2-sts-0"},
		{Rack: "rack3", DC: "test-dc2"}: {"test-cluster-dc2-rack3-sts-0"},
	}
	result = getTargetRackFQDNs(cassDc)
	assert.Equal(t, expectedSourceRacks, result)
}

func TestGetTargetRackFQDNsExistingOverride(t *testing.T) {
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
		Status: cassdcapi.CassandraDatacenterStatus{
			DatacenterName: ptr.To[string]("test-dc2"),
		},
	}

	result := getTargetRackFQDNs(cassDc)
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
	result = getTargetRackFQDNs(cassDc)
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

	result := getTargetRackFQDNs(cassDc)
	expectedSourceRacks := map[NodeLocation][]string{
		{Rack: "default", DC: "Test DC2"}: {"testcluster-test-dc2-default-sts-0", "testcluster-test-dc2-default-sts-1", "testcluster-test-dc2-default-sts-2"},
	}
	assert.Equal(t, expectedSourceRacks, result)
	cassDc.Spec.Racks = []cassdcapi.Rack{
		{Name: "rack1"},
		{Name: "rack2"},
		{Name: "rack3"},
	}
	expectedSourceRacks = map[NodeLocation][]string{
		{Rack: "rack1", DC: "Test DC2"}: {"testcluster-test-dc2-rack1-sts-0"},
		{Rack: "rack2", DC: "Test DC2"}: {"testcluster-test-dc2-rack2-sts-0"},
		{Rack: "rack3", DC: "Test DC2"}: {"testcluster-test-dc2-rack3-sts-0"},
	}
	result = getTargetRackFQDNs(cassDc)
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
	// Define racks on DC. The rack names must match the rack names of the source backup.
	cassDc.Spec.Racks = []cassdcapi.Rack{
		{Name: "test-rack1"},
		{Name: "test-rack2"},
		{Name: "test-rack3"},
	}
	// Make DC name = "test-dc2" which is our test DC which has racks.
	medusaBackup = pkgtest.NewMedusaRestore("default", "local-backupname", "remote-backupname", "test-dc2", "test-cluster")
	cassDc.Name = "test-dc2"
	expected = HostMappingSlice{
		{
			Source: "test-cluster-test-dc2-test-rack1-sts-0",
			Target: "test-cluster-test-dc2-test-rack1-sts-0",
		},
		{
			Source: "test-cluster-test-dc2-test-rack2-sts-0",
			Target: "test-cluster-test-dc2-test-rack2-sts-0",
		},
		{
			Source: "test-cluster-test-dc2-test-rack3-sts-0",
			Target: "test-cluster-test-dc2-test-rack3-sts-0",
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
	_, err = s.IsInPlace()
	assert.Error(t, err, err)
}

// configurableMockClient is a mockgRPCClient whose GetBackups result can be set per test.
type configurableMockClient struct {
	mockgRPCClient
	backups []*BackupSummary
	err     error
}

func (c configurableMockClient) GetBackups(ctx context.Context) ([]*BackupSummary, error) {
	return c.backups, c.err
}

func newTestDC(clusterName string, size int32, racks ...string) *cassdcapi.CassandraDatacenter {
	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "default"},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName: clusterName,
			Size:        size,
		},
	}
	for _, r := range racks {
		dc.Spec.Racks = append(dc.Spec.Racks, cassdcapi.Rack{Name: r})
	}
	return dc
}

func backupNode(host, dc, rack string) *BackupNode {
	return &BackupNode{Host: host, Datacenter: dc, Rack: rack}
}

// pitrFixture is a source DC "dc1" of cluster "src" with racks r1 and r2 with two nodes each. Node 0 of each rack is
// part of backup b1 and node 1 of each rack is part of backup b2, so that base backups differ per node.
func pitrFixture() (backups []*BackupSummary, nodeBackups map[string]string) {
	backups = []*BackupSummary{
		{BackupName: "b1", Nodes: []*BackupNode{
			backupNode("src-dc1-r1-sts-0", "dc1", "r1"),
			backupNode("src-dc1-r2-sts-0", "dc1", "r2"),
		}},
		{BackupName: "b2", Nodes: []*BackupNode{
			backupNode("src-dc1-r1-sts-1", "dc1", "r1"),
			backupNode("src-dc1-r2-sts-1", "dc1", "r2"),
		}},
	}
	nodeBackups = map[string]string{
		"src-dc1-r1-sts-0": "b1",
		"src-dc1-r2-sts-0": "b1",
		"src-dc1-r1-sts-1": "b2",
		"src-dc1-r2-sts-1": "b2",
	}
	return
}

func requireTerminal(t *testing.T, err error) {
	t.Helper()
	assert.Error(t, err)
	var terminal *TerminalError
	assert.True(t, errors.As(err, &terminal), "expected a TerminalError, got %v", err)
}

func TestGetPitrHostMapSameNames(t *testing.T) {
	backups, nodeBackups := pitrFixture()
	dc := newTestDC("src", 4, "r1", "r2")

	result, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, context.Background())
	assert.NoError(t, err)
	assert.Equal(t, HostMappingSlice{
		{Source: "src-dc1-r1-sts-0", Target: "src-dc1-r1-sts-0"},
		{Source: "src-dc1-r1-sts-1", Target: "src-dc1-r1-sts-1"},
		{Source: "src-dc1-r2-sts-0", Target: "src-dc1-r2-sts-0"},
		{Source: "src-dc1-r2-sts-1", Target: "src-dc1-r2-sts-1"},
	}, result)
	inPlace, err := result.IsInPlace()
	assert.NoError(t, err)
	assert.True(t, inPlace)
}

func TestGetPitrHostMapDifferentNames(t *testing.T) {
	backups, nodeBackups := pitrFixture()
	// Same layout, different cluster name, so different pod names.
	dc := newTestDC("tgt", 4, "r1", "r2")

	result, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, context.Background())
	assert.NoError(t, err)
	assert.Equal(t, HostMappingSlice{
		{Source: "src-dc1-r1-sts-0", Target: "tgt-dc1-r1-sts-0"},
		{Source: "src-dc1-r1-sts-1", Target: "tgt-dc1-r1-sts-1"},
		{Source: "src-dc1-r2-sts-0", Target: "tgt-dc1-r2-sts-0"},
		{Source: "src-dc1-r2-sts-1", Target: "tgt-dc1-r2-sts-1"},
	}, result)
	inPlace, err := result.IsInPlace()
	assert.NoError(t, err)
	assert.False(t, inPlace)

	mapping, err := result.ToMedusaRestoreMapping()
	assert.NoError(t, err)
	assert.False(t, *mapping.InPlace)
	assert.Equal(t, []string{"src-dc1-r2-sts-1"}, mapping.HostMap["tgt-dc1-r2-sts-1"].Source)
}

func TestGetPitrHostMapIgnoresOtherDC(t *testing.T) {
	backups, nodeBackups := pitrFixture()
	backups[0].Nodes = append(backups[0].Nodes, backupNode("src-other-zzz-sts-0", "other", "zzz"))
	nodeBackups["src-other-zzz-sts-0"] = "b1"
	dc := newTestDC("src", 4, "r1", "r2")

	result, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, context.Background())
	assert.NoError(t, err)
	assert.Len(t, result, 4)
}

func TestGetPitrHostMapSingleDefaultRack(t *testing.T) {
	backups := []*BackupSummary{{BackupName: "b1", Nodes: []*BackupNode{
		backupNode("src-dc1-default-sts-0", "dc1", "default"),
		backupNode("src-dc1-default-sts-1", "dc1", "default"),
	}}}
	nodeBackups := map[string]string{"src-dc1-default-sts-0": "b1", "src-dc1-default-sts-1": "b1"}

	result, err := GetPitrHostMap(newTestDC("src", 2), nodeBackups, configurableMockClient{backups: backups}, context.Background())
	assert.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGetPitrHostMapTopologyErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("rack count mismatch", func(t *testing.T) {
		backups, nodeBackups := pitrFixture()
		dc := newTestDC("src", 4, "r1", "r2", "r3")
		_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, ctx)
		requireTerminal(t, err)
		assert.Contains(t, err.Error(), "number of racks")
	})

	t.Run("rack names differ", func(t *testing.T) {
		backups, nodeBackups := pitrFixture()
		dc := newTestDC("src", 4, "r1", "other")
		_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, ctx)
		requireTerminal(t, err)
		assert.Contains(t, err.Error(), "rack names")
	})

	t.Run("source rack larger than target rack", func(t *testing.T) {
		backups := []*BackupSummary{{BackupName: "b1", Nodes: []*BackupNode{
			backupNode("src-dc1-r1-sts-0", "dc1", "r1"),
			backupNode("src-dc1-r1-sts-1", "dc1", "r1"),
			backupNode("src-dc1-r1-sts-2", "dc1", "r1"),
			backupNode("src-dc1-r2-sts-0", "dc1", "r2"),
		}}}
		nodeBackups := map[string]string{
			"src-dc1-r1-sts-0": "b1", "src-dc1-r1-sts-1": "b1", "src-dc1-r1-sts-2": "b1", "src-dc1-r2-sts-0": "b1",
		}
		dc := newTestDC("src", 4, "r1", "r2") // two pods per rack
		assert.NotPanics(t, func() {
			_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, ctx)
			requireTerminal(t, err)
			assert.Contains(t, err.Error(), "greater than")
		})
	})

	t.Run("target pod without a source", func(t *testing.T) {
		backups, nodeBackups := pitrFixture()
		dc := newTestDC("src", 6, "r1", "r2") // three pods per rack, two sources per rack
		_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, ctx)
		requireTerminal(t, err)
		assert.Contains(t, err.Error(), "have a source node")
	})

	t.Run("backup not found", func(t *testing.T) {
		backups, nodeBackups := pitrFixture()
		nodeBackups["src-dc1-r1-sts-0"] = "missing"
		dc := newTestDC("src", 4, "r1", "r2")
		_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, ctx)
		requireTerminal(t, err)
		assert.Contains(t, err.Error(), "missing")
	})

	t.Run("node not in its backup", func(t *testing.T) {
		backups, nodeBackups := pitrFixture()
		nodeBackups["src-dc1-r1-sts-0"] = "b2"
		dc := newTestDC("src", 4, "r1", "r2")
		_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{backups: backups}, ctx)
		requireTerminal(t, err)
		assert.Contains(t, err.Error(), "src-dc1-r1-sts-0")
	})

	t.Run("no backups at all", func(t *testing.T) {
		_, nodeBackups := pitrFixture()
		dc := newTestDC("src", 4, "r1", "r2")
		_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{}, ctx)
		requireTerminal(t, err)
	})
}

func TestGetPitrHostMapGetBackupsFailureIsNotTerminal(t *testing.T) {
	_, nodeBackups := pitrFixture()
	dc := newTestDC("src", 4, "r1", "r2")
	rootErr := errors.New("connection refused")

	_, err := GetPitrHostMap(dc, nodeBackups, configurableMockClient{err: rootErr}, context.Background())
	assert.Error(t, err)
	assert.True(t, errors.Is(err, rootErr))
	var terminal *TerminalError
	assert.False(t, errors.As(err, &terminal))
}

func TestGetHostMapTopologyErrors(t *testing.T) {
	ctx := context.Background()
	restore := pkgtest.NewMedusaRestore("default", "local", "b1", "dc1", "src")

	t.Run("rack names differ", func(t *testing.T) {
		backups, _ := pitrFixture()
		backups[0].BackupName = "b1"
		backups[0].Nodes = append(backups[0].Nodes, backups[1].Nodes...)
		dc := newTestDC("src", 4, "r1", "other")
		_, err := GetHostMap(dc, *restore, configurableMockClient{backups: backups[:1]}, ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rack names")
		var terminal *TerminalError
		assert.False(t, errors.As(err, &terminal))
	})

	t.Run("source rack larger than target rack", func(t *testing.T) {
		backups := []*BackupSummary{{BackupName: "b1", Nodes: []*BackupNode{
			backupNode("src-dc1-r1-sts-0", "dc1", "r1"),
			backupNode("src-dc1-r1-sts-1", "dc1", "r1"),
			backupNode("src-dc1-r1-sts-2", "dc1", "r1"),
			backupNode("src-dc1-r2-sts-0", "dc1", "r2"),
		}}}
		dc := newTestDC("src", 4, "r1", "r2")
		assert.NotPanics(t, func() {
			_, err := GetHostMap(dc, *restore, configurableMockClient{backups: backups}, ctx)
			assert.Error(t, err)
			var terminal *TerminalError
			assert.False(t, errors.As(err, &terminal))
		})
	})
}
