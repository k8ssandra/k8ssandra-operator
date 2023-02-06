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

type k8sResource interface {
	client.Object
	annotations.Annotated
	DeepCopy()
	DeepCopyInto(out *client.Object)
}

func ReconcileObject[G k8sResource](ctx context.Context,
	desiredObj k8sResource,
	currentObj *k8sResource,
	client client.Client,
	requeueDelay time.Duration) result.ReconcileResult {
	objectKey := types.NamespacedName{
		Name:      desiredObj.GetName(),
		Namespace: desiredObj.GetNamespace(),
	}
	annotations.AddHashAnnotation(desiredObj)
	err := client.Get(ctx, objectKey, *currentObj)
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

	if !annotations.CompareHashAnnotations(*currentObj, desiredObj) {
		resourceVersion := (*currentObj).GetResourceVersion()
		desiredObj.DeepCopyInto(currentObj)
		currentObj.SetResourceVersion(resourceVersion)
		if err := client.Update(ctx, currentObj); err != nil {
			return result.Error(err)
		}
		return result.RequeueSoon(requeueDelay)
	}
	return result.Done()
}
