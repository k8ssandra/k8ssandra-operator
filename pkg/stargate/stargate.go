package stargate

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
)

func ResourceName(kluster *api.K8ssandraCluster, dc *cassdcapi.CassandraDatacenter) string {
	return kluster.Name + "-" + dc.Name + "-stargate"
}

func ServiceName(dc *cassdcapi.CassandraDatacenter) string {
	return dc.Spec.ClusterName + "-" + dc.Name + "-stargate-service"
}

func DeploymentName(dc *cassdcapi.CassandraDatacenter, rack *cassdcapi.Rack) string {
	return dc.Spec.ClusterName + "-" + dc.Name + "-" + rack.Name + "-stargate-deployment"
}
