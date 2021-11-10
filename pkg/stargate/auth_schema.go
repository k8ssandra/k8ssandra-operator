package stargate

import (
	"fmt"
	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/httphelper"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"math"
	"strconv"
)

const (
	AuthKeyspace = "data_endpoint_auth"
	AuthTable    = "token"
)

const networkTopology = "org.apache.cassandra.locator.NetworkTopologyStrategy"

var authTableDefinition = &httphelper.TableDefinition{
	KeyspaceName: AuthKeyspace,
	TableName:    AuthTable,
	Columns: []*httphelper.ColumnDefinition{
		httphelper.NewPartitionKeyColumn("auth_token", "uuid", 0),
		httphelper.NewRegularColumn("username", "text"),
		httphelper.NewRegularColumn("created_timestamp", "int"),
	},
}

func ReconcileAuthSchema(
	kc *k8ssandraapi.K8ssandraCluster,
	dc *cassdcapi.CassandraDatacenter,
	managementApi cassandra.ManagementApiFacade,
	logger logr.Logger,
) (err error) {
	replication := desiredAuthSchemaReplication(kc.Spec.Cassandra.Datacenters...)
	if err = reconcileAuthKeyspace(replication, dc.ClusterName, managementApi, logger); err == nil {
		err = reconcileAuthTable(managementApi, logger)
	}
	return
}

func desiredAuthSchemaReplication(datacenters ...k8ssandraapi.CassandraDatacenterTemplate) map[string]int {
	desiredReplication := make(map[string]int, len(datacenters))
	for _, dcTemplate := range datacenters {
		replicationFactor := int(math.Min(3.0, float64(dcTemplate.Size)))
		desiredReplication[dcTemplate.Meta.Name] = replicationFactor
	}
	return desiredReplication
}

func reconcileAuthKeyspace(desiredReplication map[string]int, clusterName string, managementApi cassandra.ManagementApiFacade, logger logr.Logger) error {
	logger.Info(fmt.Sprintf("Ensuring that keyspace %s exists in cluster %v...", AuthKeyspace, clusterName))
	if keyspaces, err := managementApi.ListKeyspaces(AuthKeyspace); err != nil {
		return err
	} else if len(keyspaces) == 0 {
		logger.Info(fmt.Sprintf("keyspace %s does not exist in cluster %v, creating it", AuthKeyspace, clusterName))
		if err := managementApi.CreateKeyspaceIfNotExists(AuthKeyspace, desiredReplication); err != nil {
			return err
		} else {
			logger.Info(fmt.Sprintf("Keyspace %s successfully created", AuthKeyspace))
			return nil
		}
	} else {
		logger.Info(fmt.Sprintf("keyspace %s already exists in cluster %v", AuthKeyspace, clusterName))
		if actualReplication, err := managementApi.GetKeyspaceReplication(AuthKeyspace); err != nil {
			return err
		} else if compareReplications(actualReplication, desiredReplication) {
			logger.Info(fmt.Sprintf("Keyspace %s has desired replication", AuthKeyspace))
			return nil
		} else {
			logger.Info(fmt.Sprintf("keyspace %s already exists in cluster %v but has wrong replication, altering it", AuthKeyspace, clusterName))
			if err := managementApi.AlterKeyspace(AuthKeyspace, desiredReplication); err != nil {
				return err
			} else {
				logger.Info(fmt.Sprintf("Keyspace %s successfully altered", AuthKeyspace))
				return nil
			}
		}
	}
}

func reconcileAuthTable(managementApi cassandra.ManagementApiFacade, logger logr.Logger) error {
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
		// We do not currently check if the table definition matches
		logger.Info(fmt.Sprintf("table %s already exists in keyspace %v", AuthTable, AuthKeyspace))
		return nil
	}
}

func compareReplications(actualReplication map[string]string, desiredReplication map[string]int) bool {
	if len(actualReplication) == 0 {
		return false
	} else if class := actualReplication["class"]; class != networkTopology {
		return false
	} else if len(actualReplication) != len(desiredReplication)+1 {
		return false
	}
	for dcName, desiredRf := range desiredReplication {
		if actualRf, ok := actualReplication[dcName]; !ok {
			return false
		} else if rf, err := strconv.Atoi(actualRf); err != nil {
			return false
		} else if rf != desiredRf {
			return false
		}
	}
	return true
}
