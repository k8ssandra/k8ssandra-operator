package stargate

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
)

func ResourceName(dc *cassdcapi.CassandraDatacenter) string {
	// FIXME sanitize name
	return dc.Spec.ClusterName + "-" + dc.Name + "-stargate"
}

func ServiceName(dc *cassdcapi.CassandraDatacenter) string {
	// FIXME sanitize name
	return dc.Spec.ClusterName + "-" + dc.Name + "-stargate-service"
}

func DeploymentName(dc *cassdcapi.CassandraDatacenter, rack *cassdcapi.Rack) string {
	// FIXME sanitize name
	return dc.Spec.ClusterName + "-" + dc.Name + "-" + rack.Name + "-stargate-deployment"
}
