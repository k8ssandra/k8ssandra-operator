package reaper

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	reaperclient "github.com/k8ssandra/reaper-client-go/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type clientMock struct {
	mock.Mock
}

func (c *clientMock) IsReaperUp(ctx context.Context) (bool, error) {
	args := c.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (c *clientMock) GetClusterNames(ctx context.Context) ([]string, error) {
	args := c.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (c *clientMock) GetCluster(ctx context.Context, name string) (*reaperclient.Cluster, error) {
	args := c.Called(ctx, name)
	return args.Get(0).(*reaperclient.Cluster), args.Error(1)
}

func (c *clientMock) GetClusters(ctx context.Context) <-chan reaperclient.GetClusterResult {
	args := c.Called(ctx)
	return args.Get(0).(<-chan reaperclient.GetClusterResult)
}

func (c *clientMock) GetClustersSync(ctx context.Context) ([]*reaperclient.Cluster, error) {
	args := c.Called(ctx)
	return args.Get(0).([]*reaperclient.Cluster), args.Error(1)
}

func (c *clientMock) AddCluster(ctx context.Context, cluster string, seed string) error {
	args := c.Called(ctx, cluster, seed)
	return args.Error(0)
}

func (c *clientMock) DeleteCluster(ctx context.Context, cluster string) error {
	args := c.Called(ctx, cluster)
	return args.Error(0)
}
func (c *clientMock) RepairRuns(ctx context.Context, searchOptions *reaperclient.RepairRunSearchOptions) (map[uuid.UUID]*reaperclient.RepairRun, error) {
	args := c.Called(ctx, searchOptions)
	return args.Get(0).(map[uuid.UUID]*reaperclient.RepairRun), args.Error(1)
}

func (c *clientMock) RepairRun(ctx context.Context, repairRunId uuid.UUID) (*reaperclient.RepairRun, error) {
	args := c.Called(ctx, repairRunId)
	return args.Get(0).(*reaperclient.RepairRun), args.Error(1)
}

func (c *clientMock) CreateRepairRun(ctx context.Context, cluster string, keyspace string, owner string, options *reaperclient.RepairRunCreateOptions) (uuid.UUID, error) {
	args := c.Called(ctx, cluster, keyspace, owner, options)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (c *clientMock) UpdateRepairRun(ctx context.Context, repairRunId uuid.UUID, newIntensity reaperclient.Intensity) error {
	args := c.Called(ctx, repairRunId, newIntensity)
	return args.Error(0)
}

func (c *clientMock) StartRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	args := c.Called(ctx, repairRunId)
	return args.Error(0)
}

func (c *clientMock) PauseRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	args := c.Called(ctx, repairRunId)
	return args.Error(0)
}

func (c *clientMock) ResumeRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	args := c.Called(ctx, repairRunId)
	return args.Error(0)
}

func (c *clientMock) AbortRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	args := c.Called(ctx, repairRunId)
	return args.Error(0)
}

func (c *clientMock) RepairRunSegments(ctx context.Context, repairRunId uuid.UUID) (map[uuid.UUID]*reaperclient.RepairSegment, error) {
	args := c.Called(ctx, repairRunId)
	return args.Get(0).(map[uuid.UUID]*reaperclient.RepairSegment), args.Error(1)
}

func (c *clientMock) AbortRepairRunSegment(ctx context.Context, repairRunId uuid.UUID, segmentId uuid.UUID) error {
	args := c.Called(ctx, repairRunId, segmentId)
	return args.Error(0)
}

func (c *clientMock) DeleteRepairRun(ctx context.Context, repairRunId uuid.UUID, owner string) error {
	args := c.Called(ctx, repairRunId, owner)
	return args.Error(0)
}

func (c *clientMock) PurgeRepairRuns(ctx context.Context) (int, error) {
	args := c.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (c *clientMock) RepairSchedules(ctx context.Context) ([]reaperclient.RepairSchedule, error) {
	args := c.Called(ctx)
	return args.Get(0).([]reaperclient.RepairSchedule), args.Error(1)
}

func (c *clientMock) RepairSchedulesForCluster(ctx context.Context, clusterName string) ([]reaperclient.RepairSchedule, error) {
	args := c.Called(ctx, clusterName)
	return args.Get(0).([]reaperclient.RepairSchedule), args.Error(1)
}

func (c *clientMock) Login(ctx context.Context, username string, password string) error {
	args := c.Called(ctx, username, password)
	return args.Error(0)
}

func TestAddClusterToReaper(t *testing.T) {
	tests := []struct {
		name                string
		clusterName         string
		expectedClusterName string
		mockError           error
		expectError         bool
	}{
		{
			name:                "successfully adds cluster with underscores",
			clusterName:         "test_cluster",
			expectedClusterName: "test_cluster",
			mockError:           nil,
			expectError:         false,
		},
		{
			name:                "successfully adds cluster with dashes",
			clusterName:         "test-cluster",
			expectedClusterName: "test-cluster",
			mockError:           nil,
			expectError:         false,
		},
		{
			name:                "returns error when AddCluster fails",
			clusterName:         "test-cluster",
			expectedClusterName: "test-cluster",
			mockError:           errors.New("internal reaper error"),
			expectError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			expectedSeed := "test-cluster-seed-service.test-ns"
			client := &clientMock{}
			client.On("AddCluster", ctx, tt.expectedClusterName, expectedSeed).Once().Return(tt.mockError)
			manager := restReaperManager{reaperClient: client}
			inputDC := &cassdcapi.CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dc",
					Namespace: "test-ns",
				},
				Spec: cassdcapi.CassandraDatacenterSpec{
					ClusterName: tt.clusterName,
				},
			}

			err := manager.AddClusterToReaper(ctx, inputDC)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			client.AssertExpectations(t)
		})
	}
}

func TestVerifyClusterIsConfigured(t *testing.T) {
	tests := []struct {
		name           string
		clusterName    string
		mockClusters   []string
		mockError      error
		expectedResult bool
		expectError    bool
	}{
		{
			name:           "cluster is configured with dashes",
			clusterName:    "test-cluster",
			mockClusters:   []string{"test-cluster"},
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "cluster is configured with underscores",
			clusterName:    "test_cluster",
			mockClusters:   []string{"test_cluster"},
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "cluster is not configured",
			clusterName:    "test-cluster",
			mockClusters:   []string{"other-cluster", "another-cluster"},
			mockError:      nil,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "GetClusterNames returns error",
			clusterName:    "test-cluster",
			mockClusters:   nil,
			mockError:      errors.New("failed to get cluster names"),
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := &clientMock{}
			client.On("GetClusterNames", ctx).Once().Return(tt.mockClusters, tt.mockError)
			manager := restReaperManager{reaperClient: client}
			inputDC := &cassdcapi.CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dc",
					Namespace: "test-ns",
				},
				Spec: cassdcapi.CassandraDatacenterSpec{
					ClusterName: tt.clusterName,
				},
			}

			result, err := manager.VerifyClusterIsConfigured(ctx, inputDC)

			if tt.expectError {
				assert.Error(t, err)
				assert.False(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
			client.AssertExpectations(t)
		})
	}
}
