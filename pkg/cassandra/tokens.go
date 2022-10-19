package cassandra

import (
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"strconv"
	"strings"
)

// ComputeInitialTokens computes initial tokens for each DC, assign those tokens to the first RF
// pods in the DC, and stores the result in the DC config for later retrieval. This is only done for
// clusters where all DCs have num_tokens < 16, and where the partitioner is either Murmur3 or
// Random. For all other cases, no initial tokens are assigned by the operator.
//
// In the unlikely case we get here with something invalid in the cassandra.yaml configuration, and
// the token computation cannot be carried out, this function simply skips token assignment but
// doesn't fail the reconciliation loop, since the configuration is already being validated
// elsewhere.
func ComputeInitialTokens(dcConfigs []*DatacenterConfig) {
	infos := collectTokenAllocationInfos(dcConfigs)
	if infos == nil {
		return
	}
	partitioner := checkPartitioner(infos)
	if partitioner == nil {
		return
	}
	var nodesPerDc []int
	for _, info := range infos {
		nodesPerDc = append(nodesPerDc, info.numTokens*info.rf)
	}
	allInitialTokens := utils.ComputeTokens(nodesPerDc, *partitioner)
	assignInitialTokens(dcConfigs, infos, allInitialTokens)
}

type tokenAllocationInfo struct {
	numTokens   int
	rf          int
	partitioner *utils.Partitioner
}

func collectTokenAllocationInfos(dcConfigs []*DatacenterConfig) []*tokenAllocationInfo {
	var infos []*tokenAllocationInfo
	for _, dcConfig := range dcConfigs {
		if dcConfig.PerNodeConfigMapRef.Name != "" {
			// We don't compute initial tokens when there is a user-provided per-node config map
			return nil
		}
		info := &tokenAllocationInfo{
			numTokens:   computeDcNumTokens(dcConfig),
			rf:          computeDcReplicationFactor(dcConfig),
			partitioner: computeDcPartitioner(dcConfig),
		}
		if info.numTokens == -1 || info.rf == -1 || info.partitioner == nil {
			// cannot compute initial tokens for this DC, so abort
			return nil
		}
		infos = append(infos, info)
	}
	return infos
}

func computeDcNumTokens(dc *DatacenterConfig) int {
	numTokens, hasNumTokens := dc.CassandraConfig.CassandraYaml["num_tokens"]
	if hasNumTokens {
		// convert interface{} -> string -> int to account for all possible numeric types
		numTokensInt, err := strconv.Atoi(fmt.Sprintf("%v", numTokens))
		if err != nil {
			return -1
		}
		// We only compute initial tokens for clusters where all DCs have num_tokens < 16
		if numTokensInt >= 16 {
			return -1
		}
		return numTokensInt
	}
	return 1 // default num_tokens
}

func computeDcReplicationFactor(dc *DatacenterConfig) int {
	rf := 3 // default RF
	allocateTokens, hasAllocateTokens := dc.CassandraConfig.CassandraYaml["allocate_tokens_for_local_replication_factor"]
	if hasAllocateTokens {
		// convert interface{} -> string -> int to account for all possible numeric types
		allocateTokensInt, err := strconv.Atoi(fmt.Sprintf("%v", allocateTokens))
		if err != nil {
			return -1
		}
		rf = allocateTokensInt
	}
	// We need at least RF nodes in the DC in order to correctly distribute the generated tokens
	if int(dc.Size) < rf {
		rf = int(dc.Size)
	}
	return rf
}

func computeDcPartitioner(dc *DatacenterConfig) *utils.Partitioner {
	var partitioner *utils.Partitioner
	partitionerName, hasPartitioner := dc.CassandraConfig.CassandraYaml["partitioner"]
	if hasPartitioner {
		partitionerNameStr, ok := partitionerName.(string)
		if ok {
			if strings.Contains(partitionerNameStr, "Murmur3Partitioner") {
				partitioner = &utils.Murmur3Partitioner
			} else if strings.Contains(partitionerNameStr, "RandomPartitioner") {
				partitioner = &utils.RandomPartitioner
			}
		}
	} else {
		partitioner = &utils.Murmur3Partitioner // default partitioner
	}
	return partitioner
}

func checkPartitioner(infos []*tokenAllocationInfo) *utils.Partitioner {
	partitioner := infos[0].partitioner
	for _, info := range infos {
		if info.partitioner != partitioner {
			return nil // partitioner mismatch
		}
	}
	return partitioner
}

func assignInitialTokens(dcConfigs []*DatacenterConfig, infos []*tokenAllocationInfo, allInitialTokens [][]string) {
	for dcIndex, dcConfig := range dcConfigs {

		racks := dcConfig.Racks
		if len(racks) == 0 {
			racks = []cassdcapi.Rack{{Name: "default"}}
		}

		// First, generate RF pod names since we need to assign tokens to the RF first nodes
		// only. Note: we know that RF < dc.Size, that has been checked before, so we will never
		// generate a pod name that doesn't exist.
		var podNames []string
		for i := 0; i < infos[dcIndex].rf; i++ {
			rackIndex := i % len(racks)
			podIndex := i / len(racks)
			podName := fmt.Sprintf("%s-%s-%s-sts-%d",
				cassdcapi.CleanupForKubernetes(dcConfig.Cluster),
				dcConfig.Meta.Name,
				cassdcapi.CleanupSubdomain(racks[rackIndex].Name),
				podIndex)
			podNames = append(podNames, podName)
		}

		// Next, distribute the generated tokens among the RF nodes. Note: we generated exactly
		// RF * num_token tokens, so we are guaranteed to have the exact amount of tokens
		// required to populate RF nodes.
		dcConfig.InitialTokensByPodName = make(map[string][]string)
		for i, initialToken := range allInitialTokens[dcIndex] {
			podName := podNames[i%len(podNames)]
			dcConfig.InitialTokensByPodName[podName] = append(dcConfig.InitialTokensByPodName[podName], initialToken)
		}
	}
}
