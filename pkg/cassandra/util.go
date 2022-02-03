package cassandra

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func DatacenterUpdatedAfter(t time.Time, dc *cassdcapi.CassandraDatacenter) bool {
	updateCondition, found := dc.GetCondition(cassdcapi.DatacenterUpdating)
	if !found || updateCondition.Status != corev1.ConditionFalse {
		return false
	}
	return updateCondition.LastTransitionTime.After(t)
}

func DatacenterReady(dc *cassdcapi.CassandraDatacenter) bool {
	return dc.GetConditionStatus(cassdcapi.DatacenterReady) == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
}

func DatacenterStopped(dc *cassdcapi.CassandraDatacenter) bool {
	return dc.GetConditionStatus(cassdcapi.DatacenterStopped) == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
}

func DatacenterStopping(dc *cassdcapi.CassandraDatacenter) bool {
	return dc.GetConditionStatus(cassdcapi.DatacenterStopped) == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressUpdating
}

// GetDatacentersForReplication determines the DCs that should be included for replication. This function works for both
// system and non-system keyspaces. Replication for system keyspaces is initially done before the cluster has been
// initialized, and is set through the management-api, not CQL. This allows us to specify non-existent DCs for
// replication even though Cassandra 4 does not allow that. Once the cluster is initialized however, replication is done
// through CQL, and we cannot include non-existing DCs anymore; this is going to be the case: 1) when updating system
// keyspaces; 2) when configuring keyspaces for Stargate and Reaper; and 3) when updating user keyspaces during a DC
// rebuild.
func GetDatacentersForReplication(kc *api.K8ssandraCluster) []api.CassandraDatacenterTemplate {
	var datacenters []api.CassandraDatacenterTemplate
	if initialized := kc.Status.GetConditionStatus(api.CassandraInitialized) == corev1.ConditionTrue; initialized {
		// The cluster is already initialized: we can't bypass CQL anymore, so we must include only dcs that really
		// exist, otherwise Cassandra 4 is going to reject the replication. DCs can be in ready or stopped state.
		for _, dc := range kc.Spec.Cassandra.Datacenters {
			if status, found := kc.Status.Datacenters[dc.Meta.Name]; found {
				ready := status.Cassandra.GetConditionStatus(cassdcapi.DatacenterReady) == corev1.ConditionTrue
				stopped := status.Cassandra.GetConditionStatus(cassdcapi.DatacenterStopped) == corev1.ConditionTrue
				if ready || stopped {
					datacenters = append(datacenters, dc)
				}
			}
		}
	} else {
		// The cluster hasn't been initialized yet: include all dcs, even those that don't exist yet.
		// The replication is going to be set bypassing CQL, which is feasible even for Cassandra 4.
		datacenters = kc.Spec.Cassandra.Datacenters
	}
	return datacenters
}

func ComputeInitialSystemReplication(kc *api.K8ssandraCluster) SystemReplication {
	rf := 3.0

	datacenters := GetDatacentersForReplication(kc)

	for _, dc := range datacenters {
		rf = math.Min(rf, float64(dc.Size))
	}

	dcNames := make([]string, 0, len(datacenters))
	for _, dc := range datacenters {
		dcNames = append(dcNames, dc.Meta.Name)
	}

	return SystemReplication{Datacenters: dcNames, ReplicationFactor: int(rf)}
}

// ComputeReplication computes the desired replication for each dc, taking into account the desired maximum replication
// per dc.
func ComputeReplication(maxReplicationPerDc int, datacenters ...*cassdcapi.CassandraDatacenter) map[string]int {
	desiredReplication := make(map[string]int, len(datacenters))
	for _, dcTemplate := range datacenters {
		replicationFactor := int(math.Min(float64(maxReplicationPerDc), float64(dcTemplate.Spec.Size)))
		desiredReplication[dcTemplate.Name] = replicationFactor
	}
	return desiredReplication
}

// ComputeReplicationFromDcTemplates is similar to ComputeReplication but takes dc templates as parameters.
func ComputeReplicationFromDcTemplates(maxReplicationPerDc int, datacenters ...api.CassandraDatacenterTemplate) map[string]int {
	desiredReplication := make(map[string]int, len(datacenters))
	for _, dcTemplate := range datacenters {
		replicationFactor := int(math.Min(float64(maxReplicationPerDc), float64(dcTemplate.Size)))
		desiredReplication[dcTemplate.Meta.Name] = replicationFactor
	}
	return desiredReplication
}

const NetworkTopology = "org.apache.cassandra.locator.NetworkTopologyStrategy"

func CompareReplications(actualReplication map[string]string, desiredReplication map[string]int) bool {
	if len(actualReplication) == 0 {
		return false
	} else if class := actualReplication["class"]; class != NetworkTopology {
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

func ParseReplication(val []byte) (*Replication, error) {
	var result map[string]interface{}

	if err := json.Unmarshal(val, &result); err != nil {
		return nil, err
	}

	dcsReplication := Replication{datacenters: map[string]keyspacesReplication{}}

	for k, v := range result {
		ksMap, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to parse replication")
		}
		ksReplication := keyspacesReplication{}
		for keyspace, replicasVal := range ksMap {
			freplicas, ok := replicasVal.(float64)
			if !ok {
				return nil, fmt.Errorf("failed to parse replication")
			}
			replicas := int(freplicas)
			if replicas < 0 {
				return nil, fmt.Errorf("invalid replication")
			}
			ksReplication[keyspace] = replicas
		}
		dcsReplication.datacenters[k] = ksReplication
	}

	return &dcsReplication, nil
}
