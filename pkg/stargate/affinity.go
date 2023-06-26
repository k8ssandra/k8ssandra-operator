package stargate

import (
	"sort"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Deprecated label name used in cass-operator.
// See pkg/reconciliation/construct_statefulset.go
const zoneLabel = "failure-domain.beta.kubernetes.io/zone"

// computeNodeAffinityLabels computes a set of labels to select worker nodes for the Stargate pods.
// We use the same labels that were used to select worker nodes for the data pods. This means that Stargate pods will
// be scheduled on the same category of nodes that can host data pods.
// See cass-operator, pkg/reconciliation/construct_podtemplatespec.go
func computeNodeAffinityLabels(dc *cassdcapi.CassandraDatacenter, rackName string) map[string]string {
	var labels map[string]string
	racks := dc.GetRacks()
	for _, rack := range racks {
		if rack.Name == rackName {
			labels = utils.MergeMap(rack.NodeAffinityLabels, dc.Spec.NodeAffinityLabels)
			//goland:noinspection GoDeprecation
			break
		}
	}
	return labels
}

// computeNodeAffinity returns the node affinity to use to colocate Stargate pods close to data pods.
// We use the same affinity rule that is used to select worker nodes for the data pods. This means that Stargate pods
// will be scheduled on the same category of nodes as the data pods.
// See cass-operator, pkg/reconciliation/construct_podtemplatespec.go
func computeNodeAffinity(dc *cassdcapi.CassandraDatacenter, rackName string) *corev1.NodeAffinity {
	labels := computeNodeAffinityLabels(dc, rackName)
	if len(labels) == 0 {
		return nil
	}

	var nodeSelectors []corev1.NodeSelectorRequirement

	// we make a new map in order to sort because a map is random by design
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys) // Keep labels in the same order across statefulsets
	for _, key := range keys {
		selector := corev1.NodeSelectorRequirement{
			Key:      key,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{labels[key]},
		}
		nodeSelectors = append(nodeSelectors, selector)
	}

	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{
				MatchExpressions: nodeSelectors,
			}},
		},
	}
}

// computePodAntiAffinity returns an anti-affinity rule that either requires or recommends to keep Stargate pods away
// from data pods, depending on whether Stargate pods can be co-located with data pods on the same worker node.
func computePodAntiAffinity(allowStargateOnDataNodes bool, dc *cassdcapi.CassandraDatacenter, rackName string) *corev1.PodAntiAffinity {
	// Define a pod anti-affinity template to match data pods in this rack
	podAffinityTerm := corev1.PodAffinityTerm{
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      cassdcapi.ClusterLabel,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{cassdcapi.CleanLabelValue(dc.Spec.ClusterName)},
				},
				{
					Key:      cassdcapi.DatacenterLabel,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{dc.Name},
				},
				{
					Key:      cassdcapi.RackLabel,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{rackName},
				},
			},
		},
		TopologyKey: "kubernetes.io/hostname",
		Namespaces:  []string{dc.Namespace},
	}
	// If co-locating Stargate pods with data pods is allowed: define a preferred pod anti-affinity to avoid
	// co-locating Stargate pods with data pods as much as possible, without enforcing it.
	// If co-locating Stargate pods with data pods is forbidden: define a required pod anti-affinity; this means
	// Stargate pods will need to sit on separate workers, otherwise they won't be scheduled.
	if allowStargateOnDataNodes {
		return &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				Weight:          100,
				PodAffinityTerm: podAffinityTerm,
			}},
		}
	} else {
		return &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{podAffinityTerm},
		}
	}
}
