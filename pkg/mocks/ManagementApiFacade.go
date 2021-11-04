// Code generated by mockery 2.9.4. DO NOT EDIT.

package mocks

import (
	httphelper "github.com/k8ssandra/cass-operator/pkg/httphelper"
	mock "github.com/stretchr/testify/mock"
)

// ManagementApiFacade is an autogenerated mock type for the ManagementApiFacade type
type ManagementApiFacade struct {
	mock.Mock
}

// AlterKeyspace provides a mock function with given fields: keyspaceName, replicationSettings
func (_m *ManagementApiFacade) AlterKeyspace(keyspaceName string, replicationSettings map[string]int) error {
	ret := _m.Called(keyspaceName, replicationSettings)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, map[string]int) error); ok {
		r0 = rf(keyspaceName, replicationSettings)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateKeyspaceIfNotExists provides a mock function with given fields: keyspaceName, replication
func (_m *ManagementApiFacade) CreateKeyspaceIfNotExists(keyspaceName string, replication map[string]int) error {
	ret := _m.Called(keyspaceName, replication)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, map[string]int) error); ok {
		r0 = rf(keyspaceName, replication)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateTable provides a mock function with given fields: definition
func (_m *ManagementApiFacade) CreateTable(definition *httphelper.TableDefinition) error {
	ret := _m.Called(definition)

	var r0 error
	if rf, ok := ret.Get(0).(func(*httphelper.TableDefinition) error); ok {
		r0 = rf(definition)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// EnsureKeyspaceReplication provides a mock function with given fields: keyspaceName, replication
func (_m *ManagementApiFacade) EnsureKeyspaceReplication(keyspaceName string, replication map[string]int) error {
	ret := _m.Called(keyspaceName, replication)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, map[string]int) error); ok {
		r0 = rf(keyspaceName, replication)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetKeyspaceReplication provides a mock function with given fields: keyspaceName
func (_m *ManagementApiFacade) GetKeyspaceReplication(keyspaceName string) (map[string]string, error) {
	ret := _m.Called(keyspaceName)

	var r0 map[string]string
	if rf, ok := ret.Get(0).(func(string) map[string]string); ok {
		r0 = rf(keyspaceName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(keyspaceName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListKeyspaces provides a mock function with given fields: keyspaceName
func (_m *ManagementApiFacade) ListKeyspaces(keyspaceName string) ([]string, error) {
	ret := _m.Called(keyspaceName)

	var r0 []string
	if rf, ok := ret.Get(0).(func(string) []string); ok {
		r0 = rf(keyspaceName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(keyspaceName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListTables provides a mock function with given fields: keyspaceName
func (_m *ManagementApiFacade) ListTables(keyspaceName string) ([]string, error) {
	ret := _m.Called(keyspaceName)

	var r0 []string
	if rf, ok := ret.Get(0).(func(string) []string); ok {
		r0 = rf(keyspaceName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(keyspaceName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
