package medusa

import (
	"context"
	"errors"
	"fmt"
	"sort"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
)

// HostMapping is a type that reflects the mapping of source IP address from the Medusa backup to the target host name obtained from looking at the k8s
// statefulsets that comprise the Cassandra racks.
type HostMapping struct {
	Source string
	Target string
}
type HostMappingSlice []HostMapping

type mappable interface {
	ToSourceTargetMap() map[string]string
	ToTargetSourceMap() map[string]string
}

// Transform HostMappingSlice into a map with source IPs as keys and Target IPs as values.
func (m HostMappingSlice) ToSourceTargetMap() map[string]string {
	out := make(map[string]string)
	for _, i := range m {
		out[i.Source] = i.Target
	}
	return out
}

// Transform HostMappingSlice into a map with target IPs as keys and source IPs as values.
func (m HostMappingSlice) ToTargetSourceMap() map[string]string {
	out := make(map[string]string)
	for _, i := range m {
		out[i.Target] = i.Source
	}
	return out
}

// Transform map keyed by target IP with source IP values into HostMappingSlice
func FromTargetSourceMap(m map[string]string) HostMappingSlice {
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
func FromSourceTargetMap(m map[string]string) HostMappingSlice {
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

// getSourceRacksIPs gets a map of racks to IPs or hostnames from a Medusa CassandraBackup k8s object.
func getSourceRacksIPs(k8sRestore medusaapi.MedusaRestoreJob, client Client, ctx context.Context) (map[NodeLocation][]string, error) {
	backups, err := client.GetBackups(ctx)
	if err != nil {
		return nil, err
	}
	if len(backups) == 0 {
		return nil, fmt.Errorf("no backups found")
	}
	namedBackup, err := filterBackupsByName(k8sRestore.Spec.Backup, backups)
	if err != nil {
		return nil, err
	}
	out := make(map[NodeLocation][]string)
	for _, i := range namedBackup.Nodes {
		if i.Datacenter == k8sRestore.Spec.CassandraDatacenter {
			location := NodeLocation{
				Rack: i.Rack,
				DC:   i.Datacenter,
			}
			DNSOrIP := string(i.Host)
			if err != nil {
				return nil, err
			}
			_, exists := out[location]
			if exists {
				out[location] = append(out[location], DNSOrIP)
			} else {
				out[location] = []string{DNSOrIP}
			}
		}
	}
	return out, nil
}

type rack struct {
	Location NodeLocation
	Size     int
	Nodes    []string
}

// sortLocations() sorts the map of nodelocations alphabetically.
// NetworkTopologyStrategy organises replicas according to the alphabetised rack names. To ensure that nodes are correctly placed, we need to match off
// racks in alphabetical order and check that they are of equal length between source and destination.
func sortLocations(m map[NodeLocation][]string) []rack {
	keys := make([]string, 0, len(m))
	DC := ""
	for k := range m {
		keys = append(keys, k.Rack)
		if DC == "" {
			DC = k.DC // Just get first DC. DCs should always be homogenous for a given backup source/target and this is checked elsewhere.
		}
	}
	sort.Strings(keys)
	out := []rack{}
	for _, k := range keys {
		loc := NodeLocation{
			Rack: k,
			DC:   DC,
		}
		out = append(out,
			rack{
				Location: loc,
				Size:     len(m[loc]),
				Nodes:    m[loc],
			},
		)
	}
	return out
}

// getClusterRackFQDNs gets a map of racks to FQDNs from the current K8ssandraCluster k8s object. The CassDC does not exist yet, so we cannot refer to it for names.
// We refer to the following code for how to calculate pod names: // https://github.com/k8ssandra/cass-operator/blob/master/pkg/reconciliation/construct_statefulset.go#L39
func getTargetRackFQDNs(cassDC *cassdcapi.CassandraDatacenter) (map[NodeLocation][]string, error) {
	racks := cassDC.GetRacks()
	out := make(map[NodeLocation][]string)
	for _, i := range racks {
		location := NodeLocation{
			DC:   cassDC.SanitizedName(),
			Rack: i.Name,
		}
		sizePerRack := int(cassDC.Spec.Size) / len(racks)
		out[location] = getPodNames(cassdcapi.CleanupForKubernetes(cassDC.Spec.ClusterName), cassDC.SanitizedName(), i.Name, sizePerRack)
	}
	return out, nil
}

func getPodNames(clusterName string, DCName string, rackName string, rackSize int) []string {
	out := []string{}
	for i := 0; i < rackSize; i++ {
		out = append(out, fmt.Sprintf("%s-%s-%s-sts-%s", clusterName, DCName, rackName, fmt.Sprint(i)))
	}
	return out
}

// GetHostMap gets the hostmap for a given CassandraBackup from IP or DNS name sources to DNS named targets from the K8ssandraCluster and the backups returned by the Medusa gRPC client.
func GetHostMap(cassDC *cassdcapi.CassandraDatacenter, k8sbackup medusaapi.MedusaRestoreJob, client Client, ctx context.Context) (HostMappingSlice, error) {
	sourceRacks, err := getSourceRacksIPs(k8sbackup, client, ctx)
	if err != nil {
		return nil, err
	}
	destRacks, err := getTargetRackFQDNs(cassDC)
	if err != nil {
		return nil, err
	}
	sortedSource := sortLocations(sourceRacks)
	sortedDests := sortLocations(destRacks)
	if len(sortedDests) != len(sortedSource) {
		return nil, errors.New("number of racks in source != racks in destination")
	}
	out := HostMappingSlice{}
	for i, sourceRack := range sortedSource {
		for j, sourceNode := range sourceRack.Nodes {
			// We've already bounds checked sortedDest/sortedSource above, no need to do it again here.
			destRack := sortedDests[i]
			if j > destRack.Size {
				return nil, fmt.Errorf("number of nodes in source rack %s greater than number of nodes in dest rack %s", sourceRack.Location.Rack, destRack.Location.Rack)
			}
			if destRack.Location.DC != sourceRack.Location.DC {
				return nil, fmt.Errorf("DCs do not match in source rack %s and dest rack %s", sourceRack.Location.DC, destRack.Location.DC)
			}
			out = append(out,
				HostMapping{
					Source: sourceNode,
					Target: destRack.Nodes[j],
				},
			)
		}
	}
	return out, nil
}

func (s HostMappingSlice) IsInPlace() (bool, error) {
	inPlace := true
	for _, i := range s {
		if i.Source == "" || i.Target == "" {
			return false, errors.New("source or target was undefined")
		}
		if i.Source != i.Target {
			inPlace = false
		}
	}
	return inPlace, nil
}

func (s HostMappingSlice) ToMedusaRestoreMapping() (medusaapi.MedusaRestoreMapping, error) {
	inPlace, err := s.IsInPlace()
	if err != nil {
		return medusaapi.MedusaRestoreMapping{}, err
	}
	out := medusaapi.MedusaRestoreMapping{
		InPlace: &inPlace,
		HostMap: make(map[string]medusaapi.MappingSource),
	}
	for _, i := range s {
		out.HostMap[i.Target] = medusaapi.MappingSource{
			Source: []string{i.Source},
			Seed:   false,
		}
	}
	return out, nil
}
