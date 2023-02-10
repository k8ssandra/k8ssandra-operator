package reconciliation

import (
	"context"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileConfigMap(ctx context.Context, kClient client.Client, requeueDelay time.Duration, desiredObject corev1.ConfigMap) result.ReconcileResult {
	objectKey := types.NamespacedName{
		Name:      desiredObject.Name,
		Namespace: desiredObject.Namespace,
	}
	annotations.AddHashAnnotation(&desiredObject)

	currentCm := &corev1.ConfigMap{}

	err := kClient.Get(ctx, objectKey, currentCm)

	if err != nil {
		if errors.IsNotFound(err) {
			if err := kClient.Create(ctx, &desiredObject); err != nil {
				if errors.IsAlreadyExists(err) {
					// the read from the local cache didn't catch that the resource was created
					// already; simply requeue until the cache is up-to-date
					dcLogger.Info("Reaper Vector Agent configuration already exists, requeueing")
					return result.RequeueSoon(requeueDelay)
				return result.Error(err)
			}
			return result.RequeueSoon(requeueDelay)
		} else {
			return result.Error(err)
		}
	}

	if !annotations.CompareHashAnnotations(currentCm, &desiredObject) {
		resourceVersion := currentCm.GetResourceVersion()
		desiredObject.DeepCopyInto(currentCm)
		currentCm.SetResourceVersion(resourceVersion)
		if err := kClient.Update(ctx, currentCm); err != nil {
			return result.Error(err)
		}
		return result.RequeueSoon(requeueDelay)
	}
	return result.Done()
}
