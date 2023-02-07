package reconciliation

import (
	"context"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconcilable[G client.Object] interface {
	client.Object
	annotations.Annotated
	DeepCopy() G
	DeepCopyInto(out G)
}

type ReconcileWrapper[G Reconcilable[G]] struct {
	desiredObj G
	currentObj G
}

func NewReconcileWrapper[G Reconcilable[G]](desiredObj G) ReconcileWrapper[G] {
	out := ReconcileWrapper[G]{
		desiredObj: desiredObj,
		currentObj: *new(G),
	}
	return out
}

// ReconcileObject takes a desired k8s resource and an empty object of the same type
// and reconciles it into the desired state, returning a ReconcileResult according to the outcome of the reconciliation.
func (wrapper ReconcileWrapper[G]) ReconcileObject(ctx context.Context,
	kClient client.Client,
	requeueDelay time.Duration) result.ReconcileResult {
	objectKey := types.NamespacedName{
		Name:      wrapper.desiredObj.GetName(),
		Namespace: wrapper.desiredObj.GetNamespace(),
	}
	annotations.AddHashAnnotation(wrapper.desiredObj)
	err := kClient.Get(ctx, objectKey, wrapper.currentObj)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := kClient.Create(ctx, wrapper.desiredObj); err != nil {
				return result.Error(err)
			}
			return result.RequeueSoon(requeueDelay)
		} else {
			return result.Error(err)
		}
	}

	if !annotations.CompareHashAnnotations(wrapper.currentObj, wrapper.desiredObj) {
		resourceVersion := (wrapper.currentObj).GetResourceVersion()
		wrapper.desiredObj.DeepCopyInto(wrapper.currentObj)
		wrapper.currentObj.SetResourceVersion(resourceVersion)
		if err := kClient.Update(ctx, wrapper.currentObj); err != nil {
			return result.Error(err)
		}
		return result.RequeueSoon(requeueDelay)
	}
	return result.Done()
}

// ReconcileObjectForKluster takes a K8ssandraCluster, a desired object, and an empty object of the same type,
// and creates an object with all required `part of` labels derived from the K8ssandraCluster.
// func ReconcileObjectForKluster(ctx context.Context,
// 	desiredObj k8sResource,
// 	currentObj *client.Object,
// 	kluster k8ssandraapi.K8ssandraCluster,
// 	client client.Client,
// 	requeueDelay time.Duration) result.ReconcileResult {
// 	kKey := types.NamespacedName{
// 		Name:      kluster.Name,
// 		Namespace: kluster.Namespace,
// 	}
// 	labels.IsPartOf(desiredObj, kKey)
// 	return ReconcileObject(ctx, desiredObj, currentObj, client, requeueDelay)
// }
