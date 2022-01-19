package stargate

import (
	"fmt"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
)

const (
	AuthKeyspace = "data_endpoint_auth"
	AuthTable    = "token"
)

var authTableDefinition = &httphelper.TableDefinition{
	KeyspaceName: AuthKeyspace,
	TableName:    AuthTable,
	Columns: []*httphelper.ColumnDefinition{
		httphelper.NewPartitionKeyColumn("auth_token", "uuid", 0),
		httphelper.NewRegularColumn("username", "text"),
		httphelper.NewRegularColumn("created_timestamp", "int"),
	},
}

// ReconcileAuthKeyspace ensures that the Stargate auth schema exists, has the appropriate replication, and contains the
// appropriate tables.
func ReconcileAuthKeyspace(dcs []*cassdcapi.CassandraDatacenter, managementApi cassandra.ManagementApiFacade, logger logr.Logger) error {
	replication := cassandra.ComputeReplication(3, dcs...)
	logger.Info(fmt.Sprintf("Reconciling Stargate auth keyspace %v", AuthKeyspace))
	if err := managementApi.EnsureKeyspaceReplication(AuthKeyspace, replication); err != nil {
		logger.Error(err, "Failed to ensure keyspace replication")
		return err
	}
	return ReconcileAuthTable(managementApi, logger)
}

// ReconcileAuthTable ensures that the Stargate auth schema contains the appropriate tables. Note that we don't do much
// here and currently, we only check if the token table exists, without checking if the table definition matches the
// expected one. If the auth schema becomes more complex in the future, we'd need to find a more robust solution, maybe
// à la Reaper.
func ReconcileAuthTable(managementApi cassandra.ManagementApiFacade, logger logr.Logger) error {
	logger.Info(fmt.Sprintf("Reconciling Stargate auth table %v.%v", AuthKeyspace, AuthTable))
	if tables, err := managementApi.ListTables(AuthKeyspace); err != nil {
		return err
	} else if len(tables) == 0 {
		logger.Info(fmt.Sprintf("Table %s does not exist in keyspace %v, creating it", AuthTable, AuthKeyspace))
		if err := managementApi.CreateTable(authTableDefinition); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to create table %v", AuthTable))
			return err
		} else {
			logger.Info(fmt.Sprintf("Table %s successfully created", AuthTable))
			return nil
		}
	} else {
		logger.Info(fmt.Sprintf("Table %v.%s already exists", AuthKeyspace, AuthTable))
		return nil
	}
}
