package reconciliation

import (
	"context"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PtrIsInterface[T any] interface {
	client.Object
	DeepCopy() T
	DeepCopyInto(out T)
	*T
}

// type DeepCopyable[V PtrIsInterface[V]] interface {
// 	Reconcilable
// 	PtrIsInterface[V]
// }

// UNDER TEST

// type ReconcileDecorator[C Reconcilable[C], T PtrIsInterface[C]] struct {
// 	desiredObj C
// 	currentObj C
// }

// func NewReconcileDecorator[C client.Object, V Reconcilable[C]](desiredObj V) ReconcileDecorator[V] {
// 	out := ReconcileDecorator[C, V]{
// 		desiredObj: desiredObj,
// 		currentObj: *new(V),
// 	}
// 	return out
// }

// UNDER TEST

func ReconcileObject[T any, U PtrIsInterface[T]](
	desiredObject U,
	ctx context.Context,
	kClient client.Client,
	requeueDelay time.Duration,
) result.ReconcileResult {
	container := new(U)
	kClient.Get(ctx, types.NamespacedName{Name: "test", Namespace: "test-ns"}, (*container))
	newObject := new(T)
	(desiredObject).DeepCopyInto(*newObject)
	return result.Done()
}

// ReconcileObject takes a desired k8s resource and an empty object of the same type
// and reconciles it into the desired state, returning a ReconcileResult according to the outcome of the reconciliation.
// func ReconcileObject[V Reconcilable[V], T PtrIsInterface[V]](
// 	decorator ReconcileDecorator[V, T],
// 	)  {
// 	objectKey := types.NamespacedName{
// 		Name:      decorator.desiredObj.GetName(),
// 		Namespace: decorator.desiredObj.GetNamespace(),
// 	}
// 	annotations.AddHashAnnotation(decorator.desiredObj)
// 	err := kClient.Get(ctx, objectKey, decorator.currentObj)
// 	if err != nil {
// 		if errors.IsNotFound(err) {
// 			if err := kClient.Create(ctx, decorator.desiredObj); err != nil {
// 				return result.Error(err)
// 			}
// 			return result.RequeueSoon(requeueDelay)
// 		} else {
// 			return result.Error(err)
// 		}
// 	}
// 	newObj := new(T)
// 	if !annotations.CompareHashAnnotations(decorator.currentObj, decorator.desiredObj) {
// 		resourceVersion := (decorator.currentObj).GetResourceVersion()
// 		decorator.desiredObj.DeepCopyInto(*newObj)
// 		decorator.currentObj.SetResourceVersion(resourceVersion)
// 		if err := kClient.Update(ctx, decorator.currentObj); err != nil {
// 			return result.Error(err)
// 		}
// 		return result.RequeueSoon(requeueDelay)
// 	}
// 	return result.Done()
// }

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
