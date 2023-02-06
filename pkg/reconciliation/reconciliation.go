package reconciliation

import (
	"context"
	"time"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
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

// ReconcileObject takes a desired k8s resource and an empty object of the same type
// and reconciles it into the desired state, returning a ReconcileResult according to the outcome of the reconciliation.
func ReconcileObject(ctx context.Context,
	desiredObj k8sResource,
	currentObj *client.Object,
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
		(*currentObj).SetResourceVersion(resourceVersion)
		if err := client.Update(ctx, *currentObj); err != nil {
			return result.Error(err)
		}
		return result.RequeueSoon(requeueDelay)
	}
	return result.Done()
}

// ReconcileObjectForKluster takes a K8ssandraCluster, a desired object, and an empty object of the same type,
// and creates an object with all required `part of` labels derived from the K8ssandraCluster.
func ReconcileObjectForKluster(ctx context.Context,
	desiredObj k8sResource,
	currentObj *client.Object,
	kluster k8ssandraapi.K8ssandraCluster,
	client client.Client,
	requeueDelay time.Duration) result.ReconcileResult {
	kKey := types.NamespacedName{
		Name:      kluster.Name,
		Namespace: kluster.Namespace,
	}
	labels.IsPartOf(desiredObj, kKey)
	return ReconcileObject(ctx, desiredObj, currentObj, client, requeueDelay)
}
