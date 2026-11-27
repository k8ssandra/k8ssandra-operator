package reconciliation

import (
	"context"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/leancache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	corev1 "k8s.io/api/core/v1"
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
	recResult, _ := ReconcileAndGetObject[U, T](ctx, kClient, requeueDelay, desiredObject)
	return recResult
}

// ReconcileAndGetObject ensures that desiredObject exists in the given state, either by creating it, or updating it if
// it already exists. It returns the current state of the object on the server after the reconciliation.
func ReconcileAndGetObject[U any, T Reconcileable[U]](
	ctx context.Context, kClient client.Client, requeueDelay time.Duration, desiredObject U,
) (result.ReconcileResult, *U) {
	objectKey := types.NamespacedName{
		Name:      T(&desiredObject).GetName(),
		Namespace: T(&desiredObject).GetNamespace(),
	}
	switch any(&desiredObject).(type) {
	case *corev1.Secret, *corev1.ConfigMap:
		// Part of the hashed state, so objects created before the label existed get it on update
		leancache.MarkWatched(T(&desiredObject))
	}
	annotations.AddHashAnnotation(T(&desiredObject))

	currentCm := new(U)

	err := kClient.Get(ctx, objectKey, T(currentCm))

	if err != nil {
		if !errors.IsNotFound(err) {
			return result.Error(err), nil
		}
		if err := kClient.Create(ctx, T(&desiredObject)); err == nil {
			return result.Continue(), &desiredObject
		} else if !errors.IsAlreadyExists(err) {
			return result.Error(err), nil
		}
		// Either the cache is behind, or the object exists without the labels the cache selects
		// on. Read it live so the update below can fix the labels.
		if err := leancache.APIReader(kClient).Get(ctx, objectKey, T(currentCm)); err != nil {
			if errors.IsNotFound(err) {
				return result.RequeueSoon(requeueDelay), nil
			}
			return result.Error(err), nil
		}
	}

	if !annotations.CompareHashAnnotations(T(currentCm), T(&desiredObject)) {
		resourceVersion := T(currentCm).GetResourceVersion()
		T(&desiredObject).DeepCopyInto(currentCm)
		T(currentCm).SetResourceVersion(resourceVersion)
		if err := kClient.Update(ctx, T(currentCm)); err != nil {
			return result.Error(err), nil
		}
		return result.Continue(), currentCm
	}
	return result.Continue(), currentCm
}
