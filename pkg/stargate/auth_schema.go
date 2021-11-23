package stargate

import (
	"fmt"
	"github.com/go-logr/logr"
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

// ReconcileAuthTable ensures that the Stargate auth schema contains the appropriate tables. Note that we don't do much
// here and currently, we only check if the token table exists, without checking if the table definition matches the
// expected one. If the auth schema becomes more complex in the future, we'd need to find a more robust solution, maybe
// à la Reaper.
func ReconcileAuthTable(managementApi cassandra.ManagementApiFacade, logger logr.Logger) error {
	if tables, err := managementApi.ListTables(AuthKeyspace); err != nil {
		return err
	} else if len(tables) == 0 {
		logger.Info(fmt.Sprintf("table %s does not exist in keyspace %v, creating it", AuthTable, AuthKeyspace))
		if err := managementApi.CreateTable(authTableDefinition); err != nil {
			return err
		} else {
			logger.Info(fmt.Sprintf("Table %s successfully created", AuthTable))
			return nil
		}
	} else {
		logger.Info(fmt.Sprintf("table %s already exists in keyspace %v", AuthTable, AuthKeyspace))
		return nil
	}
}
