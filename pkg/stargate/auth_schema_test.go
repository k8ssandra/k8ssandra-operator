package stargate

import (
	"errors"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
)

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
			err := ReconcileAuthTable(tt.managementApi(), log.NullLogger{})
			assert.Equal(t, tt.err, err)
		})
	}
}
