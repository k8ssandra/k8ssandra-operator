package cassandra

import (
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

func ComputeSystemReplication(kluster *api.K8ssandraCluster) SystemReplication {
	rf := 3.0
	for _, dc := range kluster.Spec.Cassandra.Datacenters {
		rf = math.Min(rf, float64(dc.Size))
	}

	dcNames := make([]string, 0, len(kluster.Spec.Cassandra.Datacenters))
	for _, dc := range kluster.Spec.Cassandra.Datacenters {
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

const networkTopology = "org.apache.cassandra.locator.NetworkTopologyStrategy"

func CompareReplications(actualReplication map[string]string, desiredReplication map[string]int) bool {
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
