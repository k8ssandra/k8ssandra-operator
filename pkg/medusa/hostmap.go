package medusa

import (
	"context"
	"errors"

	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"inet.af/netaddr"
)

type HostName string

// HostMapping is a type that reflects the mapping of source IP address from the Medusa backup to the target host name obtained from looking at the k8s
// statefulsets that comprise the Cassandra racks.
type HostMapping struct {
	Source netaddr.IP
	Target HostName
}
type HostMappingSlice []HostMapping

type mappable interface {
	ToSourceTargetMap() map[netaddr.IP]HostName
	ToTargetSourceMap() map[HostName]netaddr.IP
}

// Transform HostMappingSlide into a map with source IPs as keys and Target IPs as values.
func (m HostMappingSlice) ToSourceTargetMap() map[netaddr.IP]HostName {
	out := make(map[netaddr.IP]HostName)
	for _, i := range m {
		out[i.Source] = i.Target
	}
	return out
}

// Transform HostMappingSlide into a map with target IPs as keys and source IPs as values.
func (m HostMappingSlice) ToTargetSourceMap() map[HostName]netaddr.IP {
	out := make(map[HostName]netaddr.IP)
	for _, i := range m {
		out[i.Target] = i.Source
	}
	return out
}

// Transform map keyed by target IP with source IP values into HostMappingSlide
func FromTargetSourceMap(m map[HostName]netaddr.IP) HostMappingSlice {
	out := HostMappingSlice{}
	for k, v := range m {
		out = append(out, HostMapping{
			Source: v,
			Target: k,
		})
	}
	return out
}

// Transform map keyed by source IP with target IP values into HostMappingSlide
func FromSourceTargetMap(m map[netaddr.IP]HostName) HostMappingSlice {
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

type nodeLocation struct {
	Rack string
	DC   string
}

type backupGetter interface {
	GetBackups(ctx context.Context) ([]*BackupSummary, error)
}

// getBackupRackIPs gets a map of racks to IPs from a Medusa CassandraBackup k8s object.
func getBackupRackIPs(k8sbackup medusaapi.CassandraBackup, client backupGetter, ctx context.Context) (map[nodeLocation][]netaddr.IP, error) {
	backups, err := client.GetBackups(ctx)
	if err != nil {
		return nil, err
	}
	namedBackup, err := filterBackupsByName(k8sbackup.Spec.Name, backups)
	if err != nil {
		return nil, err
	}
	out := make(map[nodeLocation][]netaddr.IP)
	for _, i := range namedBackup.Nodes {
		location := nodeLocation{
			Rack: i.Rack,
			DC:   i.Datacenter,
		}
		IP, err := netaddr.ParseIP(i.Host)
		if err != nil {
			return nil, err
		}
		_, exists := out[location]
		if exists {
			out[location] = append(out[location], IP)
		} else {
			out[location] = []netaddr.IP{IP}
		}
	}
	return out, nil
}
