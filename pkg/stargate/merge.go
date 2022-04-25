package stargate

import (
	"github.com/imdario/mergo"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
)

func MergeStargateTemplates(src, dest *api.StargateTemplate) (*api.StargateTemplate, error) {
	if src == nil {
		return dest, nil
	} else if dest == nil {
		return src, nil
	} else {
		merged := dest.DeepCopy()
		err := mergo.Merge(merged, src)
		return merged, err
	}
}

func MergeStargateClusterTemplates(src, dest *api.StargateClusterTemplate) (*api.StargateClusterTemplate, error) {
	if src == nil {
		return dest, nil
	} else if dest == nil {
		return src, nil
	} else {
		merged := dest.DeepCopy()
		err := mergo.Merge(merged, src)
		return merged, err
	}
}
