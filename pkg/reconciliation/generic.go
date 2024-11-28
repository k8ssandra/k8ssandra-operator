package reconciliation

import (
	"context"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconcileable[T any] interface {
	client.Object
	DeepCopy() *T
	DeepCopyInto(o *T)
	*T
}

// ReconcileObject ensures that desiredObject exists in the given state, either by creating it, or updating it if it
// already exists.
func ReconcileObject[U any, T Reconcileable[U]](ctx context.Context, kClient client.Client, requeueDelay time.Duration, desiredObject U) result.ReconcileResult {
	objectKey := types.NamespacedName{
		Name:      T(&desiredObject).GetName(),
		Namespace: T(&desiredObject).GetNamespace(),
	}
	annotations.AddHashAnnotation(T(&desiredObject))

	currentCm := new(U)

	err := kClient.Get(ctx, objectKey, T(currentCm))

	if err != nil {
		if errors.IsNotFound(err) {
			if err := kClient.Create(ctx, T(&desiredObject)); err != nil {
				if errors.IsAlreadyExists(err) {
					return result.RequeueSoon(requeueDelay)
				}
				return result.Error(err)
			}
			return result.RequeueSoon(requeueDelay)
		} else {
			return result.Error(err)
		}
	}

	if !annotations.CompareHashAnnotations(T(currentCm), T(&desiredObject)) {
		resourceVersion := T(currentCm).GetResourceVersion()
		T(&desiredObject).DeepCopyInto(currentCm)
		T(currentCm).SetResourceVersion(resourceVersion)
		if err := kClient.Update(ctx, T(currentCm)); err != nil {
			return result.Error(err)
		}
		return result.RequeueSoon(requeueDelay)
	}
	return result.Continue()
}
