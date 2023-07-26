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

// ComputeReplicationFromDatacenters is similar to ComputeReplication but takes dc templates as parameters along with potential external datacenters (unmanaged by the operator).
func ComputeReplicationFromDatacenters(maxReplicationPerDc int, externalDatacenters []string, datacenters ...api.CassandraDatacenterTemplate) map[string]int {
	desiredReplication := make(map[string]int, len(datacenters))
	for _, dcTemplate := range datacenters {
		replicationFactor := int(math.Min(float64(maxReplicationPerDc), float64(dcTemplate.Size)))
		desiredReplication[dcTemplate.CassDcName()] = replicationFactor
	}
	for _, dcName := range externalDatacenters {
		desiredReplication[dcName] = maxReplicationPerDc
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
