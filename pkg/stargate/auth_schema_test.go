package stargate

import (
	"errors"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
)

func TestReconcileAuthKeyspace(t *testing.T) {
	dummyError := errors.New("failure")
	desiredReplication := map[string]int{"dc1": 1}
	goodReplication := map[string]string{
		"class": networkTopology,
		"dc1":   "1",
	}
	badReplication := map[string]string{
		"class": networkTopology,
		"dc1":   "3",
	}
	tests := []struct {
		name          string
		replication   map[string]int
		managementApi func() cassandra.ManagementApiFacade
		err           error
	}{
		{
			"list keyspace failed",
			desiredReplication,
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListKeyspaces", AuthKeyspace).Return(nil, dummyError)
				return m
			},
			dummyError,
		},
		{
			"create keyspace failed",
			desiredReplication,
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListKeyspaces", AuthKeyspace).Return([]string{}, nil)
				m.On("CreateKeyspaceIfNotExists", AuthKeyspace, desiredReplication).Return(dummyError)
				return m
			},
			dummyError,
		},
		{
			"create keyspace OK",
			desiredReplication,
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListKeyspaces", AuthKeyspace).Return([]string{}, nil)
				m.On("CreateKeyspaceIfNotExists", AuthKeyspace, desiredReplication).Return(nil)
				return m
			},
			nil,
		},
		{
			"get keyspace replication failed",
			desiredReplication,
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListKeyspaces", AuthKeyspace).Return([]string{AuthKeyspace}, nil)
				m.On("GetKeyspaceReplication", AuthKeyspace).Return(nil, dummyError)
				return m
			},
			dummyError,
		},
		{
			"get keyspace replication OK",
			desiredReplication,
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListKeyspaces", AuthKeyspace).Return([]string{AuthKeyspace}, nil)
				m.On("GetKeyspaceReplication", AuthKeyspace).Return(goodReplication, nil)
				return m
			},
			nil,
		},
		{
			"get keyspace replication wrong, alter OK",
			desiredReplication,
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListKeyspaces", AuthKeyspace).Return([]string{AuthKeyspace}, nil)
				m.On("GetKeyspaceReplication", AuthKeyspace).Return(badReplication, nil)
				m.On("AlterKeyspace", AuthKeyspace, desiredReplication).Return(nil)
				return m
			},
			nil,
		},
		{
			"get keyspace replication wrong, alter failed",
			desiredReplication,
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListKeyspaces", AuthKeyspace).Return([]string{AuthKeyspace}, nil)
				m.On("GetKeyspaceReplication", AuthKeyspace).Return(badReplication, nil)
				m.On("AlterKeyspace", AuthKeyspace, desiredReplication).Return(dummyError)
				return m
			},
			dummyError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reconcileAuthKeyspace(tt.replication, "test", tt.managementApi(), log.NullLogger{})
			assert.Equal(t, tt.err, err)
		})
	}
}

func TestReconcileAuthTable(t *testing.T) {
	dummyError := errors.New("failure")
	tests := []struct {
		name          string
		managementApi func() cassandra.ManagementApiFacade
		err           error
	}{
		{
			"list tables failed",
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListTables", AuthKeyspace).Return(nil, dummyError)
				return m
			},
			dummyError,
		},
		{
			"table exists",
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListTables", AuthKeyspace).Return([]string{AuthTable}, nil)
				return m
			},
			nil,
		},
		{
			"table creation OK",
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListTables", AuthKeyspace).Return([]string{}, nil)
				m.On("CreateTable", mock.Anything).Return(nil)
				return m
			},
			nil,
		},
		{
			"table creation failed",
			func() cassandra.ManagementApiFacade {
				m := new(mocks.ManagementApiFacade)
				m.On("ListTables", AuthKeyspace).Return([]string{}, nil)
				m.On("CreateTable", mock.Anything).Return(dummyError)
				return m
			},
			dummyError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reconcileAuthTable(tt.managementApi(), log.NullLogger{})
			assert.Equal(t, tt.err, err)
		})
	}
}

func TestDesiredAuthSchemaReplication(t *testing.T) {
	tests := []struct {
		name     string
		dcs      []k8ssandraapi.CassandraDatacenterTemplate
		expected map[string]int
	}{
		{"one dc", []k8ssandraapi.CassandraDatacenterTemplate{
			{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}, Size: 3},
		}, map[string]int{"dc1": 3}},
		{"small dc", []k8ssandraapi.CassandraDatacenterTemplate{
			{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}, Size: 1},
		}, map[string]int{"dc1": 1}},
		{"large dc", []k8ssandraapi.CassandraDatacenterTemplate{
			{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}, Size: 10},
		}, map[string]int{"dc1": 3}},
		{"many dcs", []k8ssandraapi.CassandraDatacenterTemplate{
			{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc1"}, Size: 3},
			{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc2"}, Size: 1},
			{Meta: k8ssandraapi.EmbeddedObjectMeta{Name: "dc3"}, Size: 10},
		}, map[string]int{"dc1": 3, "dc2": 1, "dc3": 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := desiredAuthSchemaReplication(tt.dcs...)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestCompareReplications(t *testing.T) {
	tests := []struct {
		name     string
		actual   map[string]string
		desired  map[string]int
		expected bool
	}{
		{"nil", nil, map[string]int{"dc1": 3}, false},
		{"empty", map[string]string{}, map[string]int{"dc1": 3}, false},
		{"wrong class", map[string]string{"class": "wrong"}, map[string]int{"dc1": 3}, false},
		{"wrong length", map[string]string{"class": networkTopology, "dc1": "3", "dc2": "3"}, map[string]int{"dc1": 3}, false},
		{"missing dc", map[string]string{"class": networkTopology, "dc2": "3"}, map[string]int{"dc1": 3}, false},
		{"invalid rf", map[string]string{"class": networkTopology, "dc1": "not a number"}, map[string]int{"dc1": 3}, false},
		{"wrong rf", map[string]string{"class": networkTopology, "dc1": "1"}, map[string]int{"dc1": 3}, false},
		{"success", map[string]string{"class": networkTopology, "dc1": "1", "dc2": "3"}, map[string]int{"dc1": 1, "dc2": 3}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareReplications(tt.actual, tt.desired)
			assert.Equal(t, tt.expected, result)
		})
	}
}
