package test

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/mocks"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

type ManagementApiFactoryAdapter func(
	ctx context.Context,
	datacenter *cassdcapi.CassandraDatacenter,
	client client.Client,
	logger logr.Logger) (cassandra.ManagementApiFacade, error)

var defaultAdapter ManagementApiFactoryAdapter = func(
	ctx context.Context,
	datacenter *cassdcapi.CassandraDatacenter,
	client client.Client,
	logger logr.Logger) (cassandra.ManagementApiFacade, error) {

	m := new(mocks.ManagementApiFacade)
	m.On(EnsureKeyspaceReplication, mock.Anything, mock.Anything).Return(nil)
	m.On(ListTables, stargate.AuthKeyspace).Return([]string{"token"}, nil)
	m.On(CreateTable, mock.MatchedBy(func(def *httphelper.TableDefinition) bool {
		return def.KeyspaceName == stargate.AuthKeyspace && def.TableName == stargate.AuthTable
	})).Return(nil)
	m.On(ListKeyspaces, "").Return([]string{}, nil)
	m.On(GetSchemaVersions).Return(map[string][]string{"fake": {"test"}}, nil)
	return m, nil
}

type FakeManagementApiFactory struct {
	t *testing.T

	adapter ManagementApiFactoryAdapter
}

func (f *FakeManagementApiFactory) SetT(t *testing.T) {
	f.t = t
}

func (f *FakeManagementApiFactory) UseDefaultAdapter() {
	f.adapter = defaultAdapter
}

func (f *FakeManagementApiFactory) SetAdapter(a ManagementApiFactoryAdapter) {
	f.adapter = a
}

func (f *FakeManagementApiFactory) NewManagementApiFacade(
	ctx context.Context,
	dc *cassdcapi.CassandraDatacenter,
	client client.Client,
	logger logr.Logger) (cassandra.ManagementApiFacade, error) {

	if f.t == nil {
		return nil, fmt.Errorf("testing.T instance not set")
	}

	if f.adapter == nil {
		return nil, fmt.Errorf("adapter not set")
	}

	var mgmtApi cassandra.ManagementApiFacade
	var err error

	mgmtApi, err = f.adapter(ctx, dc, client, logger)
	if err != nil {
		return nil, err
	}

	if testable, ok := mgmtApi.(Testable); ok {
		testable.Test(f.t)
	}

	return mgmtApi, nil
}

type ManagementApiMethod string

const (
	EnsureKeyspaceReplication = "EnsureKeyspaceReplication"
	GetKeyspaceReplication    = "GetKeyspaceReplication"
	CreateKeyspaceIfNotExists = "CreateKeyspaceIfNotExists"
	AlterKeyspace             = "AlterKeyspace"
	ListKeyspaces             = "ListKeyspaces"
	CreateTable               = "CreateTable"
	ListTables                = "ListTables"
	GetSchemaVersions         = "GetSchemaVersions"
)

type FakeManagementApiFacade struct {
	*mocks.ManagementApiFacade
}

type Testable interface {
	Test(t mock.TestingT)
}

func NewFakeManagementApiFacade() *FakeManagementApiFacade {
	m := new(mocks.ManagementApiFacade)
	return &FakeManagementApiFacade{ManagementApiFacade: m}
}

func (f *FakeManagementApiFacade) GetLastCall(method ManagementApiMethod, args ...interface{}) int {
	idx := -1

	calls := make([]mock.Call, 0)
	for _, call := range f.Calls {
		if call.Method == string(method) {
			calls = append(calls, call)
		}
	}

	for i, call := range calls {
		if _, count := call.Arguments.Diff(args); count == 0 {
			idx = i
		}
	}

	return idx
}

func (f *FakeManagementApiFacade) GetFirstCall(method ManagementApiMethod, args ...interface{}) int {
	calls := make([]mock.Call, 0)
	for _, call := range f.Calls {
		if call.Method == string(method) {
			calls = append(calls, call)
		}
	}

	for i, call := range calls {
		if _, count := call.Arguments.Diff(args); count == 0 {
			return i
		}
	}

	return -1
}

func (f *FakeManagementApiFacade) getCallsForMethod(method ManagementApiMethod) []mock.Call {
	calls := make([]mock.Call, 0)
	for _, call := range f.Calls {
		if call.Method == string(method) {
			calls = append(calls, call)
		}
	}
	return calls
}
