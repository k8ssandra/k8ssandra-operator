package medusa

import (
	"context"
	"errors"
	"fmt"
	"sort"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
)

// HostMapping is a type that reflects the mapping of source IP address from the Medusa backup to the target host name obtained from looking at the k8s
// statefulsets that comprise the Cassandra racks.
type HostMapping struct {
	Source string
	Target string
}
type HostMappingSlice []HostMapping

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
func getSourceRacksIPs(k8sRestore medusaapi.MedusaRestoreJob, client Client, ctx context.Context, dcName string) (map[NodeLocation][]string, error) {
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
	return getSourceRacks(namedBackup.Nodes, dcName), nil
}

// getSourceRacks groups backup nodes by rack and drops the nodes of other datacenters.
func getSourceRacks(nodes []*BackupNode, dcName string) map[NodeLocation][]string {
	out := make(map[NodeLocation][]string)
	for _, i := range nodes {
		if i.Datacenter == dcName {
			location := NodeLocation{
				Rack: i.Rack,
				DC:   i.Datacenter,
			}
			DNSOrIP := i.Host
			_, exists := out[location]
			if exists {
				out[location] = append(out[location], DNSOrIP)
			} else {
				out[location] = []string{DNSOrIP}
			}
		}
	}
	return out
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
		nodes := m[loc]
		sort.Strings(nodes)
		out = append(out,
			rack{
				Location: loc,
				Size:     len(m[loc]),
				Nodes:    nodes,
			},
		)
	}
	return out
}

// getClusterRackFQDNs gets a map of racks to FQDNs from the current K8ssandraCluster k8s object. The CassDC does not exist yet, so we cannot refer to it for names.
// We refer to the following code for how to calculate pod names: // https://github.com/k8ssandra/cass-operator/blob/master/pkg/reconciliation/construct_statefulset.go#L39
func getTargetRackFQDNs(dc *cassdcapi.CassandraDatacenter) map[NodeLocation][]string {
	racks := dc.GetRacks()
	rackNodeCounts := cassdcapi.SplitRacks(int(dc.Spec.Size), len(racks))
	out := make(map[NodeLocation][]string)
	for i := 0; i < len(racks); i++ {
		location := NodeLocation{
			DC:   dc.DatacenterName(),
			Rack: racks[i].Name,
		}
		podCount := rackNodeCounts[i]
		podNames := make([]string, 0, podCount)
		stsName := reconciliation.NewNamespacedNameForStatefulSet(dc, racks[i].Name)
		for j := 0; j < podCount; j++ {
			podNames = append(podNames, fmt.Sprintf("%s-%d", stsName.Name, j))
		}
		out[location] = podNames
	}

	return out
}

// GetHostMap gets the hostmap for a given CassandraBackup from IP or DNS name sources to DNS named targets from the K8ssandraCluster and the backups returned by the Medusa gRPC client.
func GetHostMap(cassDC *cassdcapi.CassandraDatacenter, k8sbackup medusaapi.MedusaRestoreJob, client Client, ctx context.Context) (HostMappingSlice, error) {
	sourceRacks, err := getSourceRacksIPs(k8sbackup, client, ctx, cassDC.DatacenterName())
	if err != nil {
		return nil, err
	}
	return buildHostMap(sourceRacks, cassDC)
}

// GetPitrHostMap maps each source node of a PITR restore to a pod of the target datacenter.
//
// nodeBackups comes from PreparePitrRestore: its keys are the source nodes and its values are the base backups
// chosen for them. Each node's rack is looked up in its backup, then nodes and pods are matched rack by rack.
func GetPitrHostMap(cassDC *cassdcapi.CassandraDatacenter, nodeBackups map[string]string, client Client, ctx context.Context) (HostMappingSlice, error) {
	backups, err := client.GetBackups(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetBackups failed: %w", err)
	}
	hostMap, err := buildPitrHostMap(cassDC, nodeBackups, backups)
	if err != nil {
		return nil, Terminal(err)
	}
	return hostMap, nil
}

func buildPitrHostMap(cassDC *cassdcapi.CassandraDatacenter, nodeBackups map[string]string, backups []*BackupSummary) (HostMappingSlice, error) {
	sourceNodeNames := make([]string, 0, len(nodeBackups))
	for node := range nodeBackups {
		sourceNodeNames = append(sourceNodeNames, node)
	}
	sort.Strings(sourceNodeNames)

	nodes := make([]*BackupNode, 0, len(nodeBackups))
	for _, node := range sourceNodeNames {
		backupName := nodeBackups[node]
		backup, err := filterBackupsByName(backupName, backups)
		if err != nil {
			return nil, fmt.Errorf("backup %q selected for node %q: %w", backupName, node, err)
		}
		backupNode := findBackupNode(backup, node)
		if backupNode == nil {
			return nil, fmt.Errorf("node %q not found in backup %q", node, backupName)
		}
		nodes = append(nodes, backupNode)
	}

	hostMap, err := buildHostMap(getSourceRacks(nodes, cassDC.DatacenterName()), cassDC)
	if err != nil {
		return nil, err
	}
	targetPods := 0
	for _, pods := range getTargetRackFQDNs(cassDC) {
		targetPods += len(pods)
	}
	if len(hostMap) != targetPods {
		return nil, fmt.Errorf("%d target pods in datacenter %s, but only %d have a source node", targetPods, cassDC.DatacenterName(), len(hostMap))
	}
	return hostMap, nil
}

func findBackupNode(backup *BackupSummary, host string) *BackupNode {
	for _, n := range backup.Nodes {
		if n.Host == host {
			return n
		}
	}
	return nil
}

// buildHostMap pairs the sorted source racks with the sorted racks of the target datacenter. Rack count and rack names
// must be equal. Node j of source rack i is paired with pod j of target rack i.
func buildHostMap(sourceRacks map[NodeLocation][]string, cassDC *cassdcapi.CassandraDatacenter) (HostMappingSlice, error) {
	destRacks := getTargetRackFQDNs(cassDC)
	sortedSource := sortLocations(sourceRacks)
	sortedDests := sortLocations(destRacks)
	if len(sortedDests) != len(sortedSource) {
		return nil, fmt.Errorf("number of racks in source != racks in destination. Source: %v, Dest: %v", sortedSource, sortedDests)
	}
	out := HostMappingSlice{}
	for i, sourceRack := range sortedSource {
		// We've already bounds checked sortedDest/sortedSource above, no need to do it again here.
		destRack := sortedDests[i]
		if sourceRack.Location.Rack != destRack.Location.Rack {
			return nil, fmt.Errorf("rack names in source does not match rack names in destination. Source: %v, Dest: %v", rackNames(sortedSource), rackNames(sortedDests))
		}
		for j, sourceNode := range sourceRack.Nodes {
			if j >= destRack.Size {
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

func rackNames(racks []rack) []string {
	names := make([]string, 0, len(racks))
	for _, r := range racks {
		names = append(names, r.Location.Rack)
	}
	return names
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
