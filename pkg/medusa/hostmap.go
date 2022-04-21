package medusa

import (
	"context"
	"errors"
	"fmt"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cassandrapkg "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"k8s.io/apimachinery/pkg/types"
)

type HostName string

type HostDNSOrIP string

// HostMapping is a type that reflects the mapping of source IP address from the Medusa backup to the target host name obtained from looking at the k8s
// statefulsets that comprise the Cassandra racks.
type HostMapping struct {
	Source HostDNSOrIP
	Target HostName
}
type HostMappingSlice []HostMapping

type mappable interface {
	ToSourceTargetMap() map[HostDNSOrIP]HostName
	ToTargetSourceMap() map[HostName]HostDNSOrIP
}

// Transform HostMappingSlice into a map with source IPs as keys and Target IPs as values.
func (m HostMappingSlice) ToSourceTargetMap() map[HostDNSOrIP]HostName {
	out := make(map[HostDNSOrIP]HostName)
	for _, i := range m {
		out[i.Source] = i.Target
	}
	return out
}

// Transform HostMappingSlice into a map with target IPs as keys and source IPs as values.
func (m HostMappingSlice) ToTargetSourceMap() map[HostName]HostDNSOrIP {
	out := make(map[HostName]HostDNSOrIP)
	for _, i := range m {
		out[i.Target] = i.Source
	}
	return out
}

// Transform map keyed by target IP with source IP values into HostMappingSlice
func FromTargetSourceMap(m map[HostName]HostDNSOrIP) HostMappingSlice {
	out := HostMappingSlice{}
	for k, v := range m {
		out = append(out, HostMapping{
			Source: v,
			Target: k,
		})
	}
	return out
}

// Transform map keyed by source IP with target IP values into HostMappingSlice
func FromSourceTargetMap(m map[HostDNSOrIP]HostName) HostMappingSlice {
	out := HostMappingSlice{}
	for k, v := range m {
		out = append(out, HostMapping{
			Source: k,
			Target: v,
		})
	}
	return out
}

func filterBackupsByName(name string, backups []*BackupSummary) (*BackupSummary, error) {
	for _, i := range backups {
		if i.BackupName == name {
			return i, nil
		}
	}
	return nil, errors.New("could not find named backup")
}

type NodeLocation struct {
	Rack string
	DC   string
}

type backupGetter interface {
	GetBackups(ctx context.Context) ([]*BackupSummary, error)
}

// getSourceRacksIPs gets a map of racks to IPs or hostnames from a Medusa CassandraBackup k8s object.
func getSourceRacksIPs(k8sRestore medusaapi.CassandraRestore, client backupGetter, ctx context.Context) (map[NodeLocation][]HostDNSOrIP, error) {
	backups, err := client.GetBackups(ctx)
	if err != nil {
		return nil, err
	}
	namedBackup, err := filterBackupsByName(k8sRestore.Spec.Backup, backups)
	if err != nil {
		return nil, err
	}
	out := make(map[NodeLocation][]HostDNSOrIP)
	for _, i := range namedBackup.Nodes {
		if i.Datacenter == k8sRestore.Spec.CassandraDatacenter.Name {
			location := NodeLocation{
				Rack: i.Rack,
				DC:   i.Datacenter,
			}
			DNSOrIP := HostDNSOrIP(i.Host)
			if err != nil {
				return nil, err
			}
			_, exists := out[location]
			if exists {
				out[location] = append(out[location], DNSOrIP)
			} else {
				out[location] = []HostDNSOrIP{DNSOrIP}
			}
		}
	}
	return out, nil
}

// getClusterRackFQDNs gets a map of racks to FQDNs from the current K8ssandraCluster k8s object. The CassDC does not exist yet, so we cannot refer to it for names.
// We refer to the following code for how to calculate pod names: // https://github.com/k8ssandra/cass-operator/blob/master/pkg/reconciliation/construct_statefulset.go#L39
func getTargetRackFQDNs(Kluster k8ssandraapi.K8ssandraCluster, dcName string) (map[NodeLocation][]HostName, error) {
	cassDC, err := cassDCFromKluster(Kluster, dcName)
	if err != nil {
		return nil, err
	}
	racks := cassDC.GetRacks()
	out := make(map[NodeLocation][]HostName)
	for _, i := range racks {
		location := NodeLocation{
			DC:   cassDC.Name,
			Rack: i.Name,
		}
		sizePerRack := int(cassDC.Spec.Size) / len(racks)
		out[location] = getPodNames(Kluster.Name, cassDC.Name, i.Name, sizePerRack)
	}
	return out, nil
}

func getPodNames(clusterName string, DCName string, rackName string, rackSize int) []HostName {
	out := []HostName{}
	for i := 0; i < rackSize; i++ {
		out = append(out, HostName(fmt.Sprintf(clusterName, "-", DCName, "-", rackName, "-sts-", fmt.Sprint(i))))
	}
	return out
}

func cassDCFromKluster(Kluster k8ssandraapi.K8ssandraCluster, dcName string) (*cassdcapi.CassandraDatacenter, error) {
	thisDC := k8ssandraapi.CassandraDatacenterTemplate{}
	if len(Kluster.Spec.Cassandra.Datacenters) > 0 {
		for _, i := range Kluster.Spec.Cassandra.Datacenters {
			if i.Meta.Name == dcName {
				thisDC = i
			}
		}
	}
	DCConfig := cassandrapkg.Coalesce(Kluster.Name, Kluster.Spec.Cassandra, &thisDC)
	cassDC, err := cassandrapkg.NewDatacenter(
		types.NamespacedName{
			Namespace: Kluster.Namespace,
			Name:      Kluster.Name,
		},
		DCConfig)
	if err != nil {
		return nil, err
	}
	return cassDC, nil
}

// GetHostMap gets the hostmap for a given CassandraBackup from IP or DNS name sources to DNS named targets from the K8ssandraCluster and the backups returned by the Medusa gRPC client.
func GetHostMap(Kluster k8ssandraapi.K8ssandraCluster, k8sbackup medusaapi.CassandraRestore, client backupGetter, ctx context.Context) (HostMappingSlice, error) {
	sourceRacks, err := getSourceRacksIPs(k8sbackup, client, ctx)
	if err != nil {
		return nil, err
	}
	destRacks, err := getTargetRackFQDNs(Kluster, k8sbackup.Spec.CassandraDatacenter.Name)
	if err != nil {
		return nil, err
	}
	out := HostMappingSlice{}
	for sourceRackLocation, sourceRackNodes := range sourceRacks {
		targetRackHosts, ok := destRacks[sourceRackLocation]
		if !ok {
			return nil, errors.New(fmt.Sprint("could not find matching DC/rack location in destination for source", "source location", sourceRackLocation))
		}
		for index, node := range sourceRackNodes {
			if index > len(targetRackHosts) {
				return nil, errors.New(fmt.Sprint("attempting to map a source rack into a rack which is too small", "sourceRackNodes", sourceRackNodes, "targetRackHosts", targetRackHosts))
			}
			out = append(
				out,
				HostMapping{
					Source: node,
					Target: targetRackHosts[index],
				},
			)
		}
	}
	return out, nil

}
