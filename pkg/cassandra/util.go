package cassandra

import (
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"time"
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
