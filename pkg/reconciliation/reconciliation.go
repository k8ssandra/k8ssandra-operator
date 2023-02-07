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

type ReconcileDecorator[G Reconcilable[G]] struct {
	desiredObj G
	currentObj G
}

func NewReconcileDecorator[G Reconcilable[G]](desiredObj G) ReconcileDecorator[G] {
	out := ReconcileDecorator[G]{
		desiredObj: desiredObj,
		currentObj: *new(G),
	}
	return out
}

// ReconcileObject takes a desired k8s resource and an empty object of the same type
// and reconciles it into the desired state, returning a ReconcileResult according to the outcome of the reconciliation.
func (decorator ReconcileDecorator[G]) ReconcileObject(ctx context.Context,
	kClient client.Client,
	requeueDelay time.Duration) result.ReconcileResult {
	objectKey := types.NamespacedName{
		Name:      decorator.desiredObj.GetName(),
		Namespace: decorator.desiredObj.GetNamespace(),
	}
	annotations.AddHashAnnotation(decorator.desiredObj)
	err := kClient.Get(ctx, objectKey, decorator.currentObj)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := kClient.Create(ctx, decorator.desiredObj); err != nil {
				return result.Error(err)
			}
			return result.RequeueSoon(requeueDelay)
		} else {
			return result.Error(err)
		}
	}

	if !annotations.CompareHashAnnotations(decorator.currentObj, decorator.desiredObj) {
		resourceVersion := (decorator.currentObj).GetResourceVersion()
		decorator.desiredObj.DeepCopyInto(decorator.currentObj)
		decorator.currentObj.SetResourceVersion(resourceVersion)
		if err := kClient.Update(ctx, decorator.currentObj); err != nil {
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
