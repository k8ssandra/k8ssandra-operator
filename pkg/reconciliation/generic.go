package reconciliation

import (
	"context"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
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

// Try with U, a type of any whose POINTER still fulfils Reoncilable...
func ReconcileObject[U any, T Reconcileable[U]](ctx context.Context, kClient client.Client, requeueDelay time.Duration, desiredObject U, klKey *types.NamespacedName) result.ReconcileResult {
	objectKey := types.NamespacedName{
		Name:      T(&desiredObject).GetName(),
		Namespace: T(&desiredObject).GetNamespace(),
	}
	if klKey != nil {
		// if a key is provided for a K8ssandra object, add the part-of labels to the reconciled object
		partOfLabels := labels.PartOfLabels(*klKey)
		T(&desiredObject).SetLabels(partOfLabels)
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
	return result.Done()
}
