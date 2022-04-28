package stargate

import (
	"github.com/adutra/goalesce"
	api "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
)

var resourceQuantityType = reflect.TypeOf(resource.Quantity{})

func MergeStargateTemplates(src, dest *api.StargateTemplate) (*api.StargateTemplate, error) {
	coalesced, err := goalesce.Coalesce(src, dest, goalesce.WithTrileans(), goalesce.WithAtomicType(resourceQuantityType))
	if err != nil {
		return nil, err
	}
	return coalesced.(*api.StargateTemplate), nil
}

func MergeStargateClusterTemplates(src, dest *api.StargateClusterTemplate) (*api.StargateClusterTemplate, error) {
	coalesced, err := goalesce.Coalesce(src, dest, goalesce.WithTrileans(), goalesce.WithAtomicType(resourceQuantityType))
	if err != nil {
		return nil, err
	}
	return coalesced.(*api.StargateClusterTemplate), nil
}
