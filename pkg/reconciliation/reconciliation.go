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

type Reconcilable interface {
	client.Object
	DeepCopy()
	DeepCopyInto(out *client.Object)
}

func ReconcileObject[G Reconcilable](ctx context.Context, desiredObj G, client client.Client, requeueDelay time.Duration) result.ReconcileResult {
	objectKey := types.NamespacedName{
		Name:      desiredObj.GetName(),
		Namespace: desiredObj.GetNamespace(),
	}
	annotations.AddHashAnnotation(desiredObj)
	var currentObj G
	err := client.Get(ctx, objectKey, currentObj)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := client.Create(ctx, desiredObj); err != nil {
				return result.Error(err)
			}
			return result.RequeueSoon(requeueDelay)
		} else {
			return result.Error(err)
		}
	}

	if !annotations.CompareHashAnnotations(currentObj, desiredObj) {
		resourceVersion := (currentObj).GetResourceVersion()
		desiredObj.DeepCopyInto((*currentObj))
		currentObj.SetResourceVersion(resourceVersion)
		if err := client.Update(ctx, currentObj); err != nil {
			return result.Error(err)
		}
		return result.RequeueSoon(requeueDelay)
	}
	return result.Done()
}
