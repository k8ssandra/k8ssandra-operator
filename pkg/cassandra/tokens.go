package cassandra

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
)

// ComputeInitialTokens computes initial tokens for each DC, assign those tokens to the first RF
// pods in the DC, and stores the result in the DC config for later retrieval.
//
// If any DC has num_tokens >= 16, or if the partitioner is not Murmur3 nor Random, or if there is
// something invalid in the cassandra.yaml configuration, then this function skips computing initial
// tokens, does not modify the DC configs, and returns an error.
func ComputeInitialTokens(dcConfigs []*DatacenterConfig) error {
	infos, err := collectTokenAllocationInfos(dcConfigs)
	if err != nil {
		return err
	}
	partitioner, err := checkPartitioner(infos)
	if err != nil {
		return err
	}
	var nodesPerDc []int
	for _, info := range infos {
		nodesPerDc = append(nodesPerDc, info.numTokens*info.rf)
	}
	allInitialTokens := utils.ComputeTokens(nodesPerDc, *partitioner)
	assignInitialTokens(dcConfigs, infos, allInitialTokens)
	return nil
}

var (
	ErrInitialTokensDCHasUserProvidedConfig = errors.New("cannot compute initial tokens: at least one DC has a user-provided per-node config")
	ErrInitialTokensInvalidNumTokens        = errors.New("cannot compute initial tokens: invalid num_tokens")
	ErrInitialTokensNumTokensTooHigh        = errors.New("cannot compute initial tokens: at least one DC has num_tokens >= 16")
	ErrInitialTokensInvalidAllocateTokens   = errors.New("cannot compute initial tokens: invalid allocate_tokens_for_local_replication_factor")
	ErrInitialTokensInvalidPartitioner      = errors.New("cannot compute initial tokens: invalid or unsupported partitioner")
	ErrInitialTokensPartitionerMismatch     = errors.New("cannot compute initial tokens: partitioner mismatch")
)

type tokenAllocationInfo struct {
	numTokens   int
	rf          int
	partitioner *utils.Partitioner
}

func collectTokenAllocationInfos(dcConfigs []*DatacenterConfig) ([]*tokenAllocationInfo, error) {
	var infos []*tokenAllocationInfo
	for _, dcConfig := range dcConfigs {
		if dcConfig.PerNodeConfigMapRef.Name != "" {
			return nil, ErrInitialTokensDCHasUserProvidedConfig
		}
		numTokens, err := computeDcNumTokens(dcConfig)
		if err != nil {
			return nil, err
		}
		rf, err := computeDcReplicationFactor(dcConfig)
		if err != nil {
			return nil, err
		}
		partitioner, err := computeDcPartitioner(dcConfig)
		if err != nil {
			return nil, err
		}
		info := &tokenAllocationInfo{
			numTokens:   numTokens,
			rf:          rf,
			partitioner: partitioner,
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func computeDcNumTokens(dc *DatacenterConfig) (int, error) {
	numTokens, hasNumTokens := dc.CassandraConfig.CassandraYaml["num_tokens"]
	if hasNumTokens {
		// convert interface{} -> string -> int to account for all possible numeric types
		numTokensInt, err := strconv.Atoi(fmt.Sprintf("%v", numTokens))
		if err != nil {
			return -1, ErrInitialTokensInvalidNumTokens
		}
		// We only compute initial tokens for clusters where all DCs have num_tokens < 16
		if numTokensInt >= 16 {
			return -1, ErrInitialTokensNumTokensTooHigh
		}
		return numTokensInt, nil
	}
	return 1, nil // default num_tokens
}

func computeDcReplicationFactor(dc *DatacenterConfig) (int, error) {
	rf := 3 // default RF
	allocateTokens, hasAllocateTokens := dc.CassandraConfig.CassandraYaml["allocate_tokens_for_local_replication_factor"]
	if hasAllocateTokens {
		// convert interface{} -> string -> int to account for all possible numeric types
		allocateTokensInt, err := strconv.Atoi(fmt.Sprintf("%v", allocateTokens))
		if err != nil {
			return -1, ErrInitialTokensInvalidAllocateTokens
		}
		rf = allocateTokensInt
	}
	// We need at least RF nodes in the DC in order to correctly distribute the generated tokens
	if int(dc.Size) < rf {
		rf = int(dc.Size)
	}
	return rf, nil
}

func computeDcPartitioner(dc *DatacenterConfig) (*utils.Partitioner, error) {
	partitionerName, hasPartitioner := dc.CassandraConfig.CassandraYaml["partitioner"]
	if hasPartitioner {
		partitionerNameStr, ok := partitionerName.(string)
		if ok {
			if strings.Contains(partitionerNameStr, "Murmur3Partitioner") {
				return &utils.Murmur3Partitioner, nil
			} else if strings.Contains(partitionerNameStr, "RandomPartitioner") {
				return &utils.RandomPartitioner, nil
			}
		}
		return nil, ErrInitialTokensInvalidPartitioner
	}
	return &utils.Murmur3Partitioner, nil // default partitioner
}

func checkPartitioner(infos []*tokenAllocationInfo) (*utils.Partitioner, error) {
	partitioner := infos[0].partitioner
	for _, info := range infos {
		if info.partitioner != partitioner {
			return nil, ErrInitialTokensPartitionerMismatch // partitioner mismatch
		}
	}
	return partitioner, nil
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
				dcConfig.CassDcName(),
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
