package reconciliation

import (
	"context"
	"testing"
	"time"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/leancache"
	testutils "github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var (
	desiredObject = corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"test-key": "test value",
		},
	}
	ctx          = context.Background()
	requeueDelay = time.Second
)

func Test_ReconcileObject_UpdateDone(t *testing.T) {
	kClient := testutils.NewFakeClientWRestMapper() // Reset the Client
	// Launch reconciliation.
	recRes := ReconcileObject(ctx, kClient, requeueDelay, desiredObject)
	// Should update immediately and signal we can continue
	assert.False(t, recRes.Completed())
	// After the update we should see the expected ConfigMap
	afterUpdateCM := &corev1.ConfigMap{}
	err := kClient.Get(ctx,
		types.NamespacedName{
			Name:      desiredObject.Name,
			Namespace: desiredObject.Namespace,
		},
		afterUpdateCM)
	assert.NoError(t, err)
}

func Test_ReconcileObject_CreateSuccess(t *testing.T) {
	kClient := testutils.NewFakeClientWRestMapper() // Reset the Client
	recRes := ReconcileObject(ctx, kClient, requeueDelay, desiredObject)
	// Should create immediately and signal we can continue
	assert.False(t, recRes.Completed())
	actualCm := &corev1.ConfigMap{}
	err := kClient.Get(ctx, types.NamespacedName{Name: desiredObject.Name, Namespace: desiredObject.Namespace}, actualCm)
	assert.NoError(t, err)
}
func Test_ReconcileObject_CreateFailed(t *testing.T) {
	kClient := testutils.NewCreateFailingFakeClient() // Reset the Client
	recRes := ReconcileObject(ctx, kClient, requeueDelay, desiredObject)
	assert.True(t, recRes.IsError())
}

func Test_ReconcileObject_UpdateSuccess(t *testing.T) {
	kClient := testutils.NewFakeClientWRestMapper() // Reset the Client
	// Create an initial ConfigMap with the same name.
	initialCm := desiredObject.DeepCopy()
	initialCm.Data = map[string]string{"wrong-key": "wrong-value"}
	initialCm.Annotations = make(map[string]string)
	initialCm.Annotations[k8ssandraapi.ResourceHashAnnotation] = "gobbledegook"
	initialCm.Data = map[string]string{"gobbledegook": "gobbledegook"}
	if err := kClient.Create(ctx, initialCm); err != nil {
		assert.Fail(t, "could not create initial ConfigMap")
	}
	// Launch reconciliation.
	recRes := ReconcileObject(ctx, kClient, requeueDelay, desiredObject)
	assert.False(t, recRes.Completed())
	expected := desiredObject.DeepCopy()
	leancache.MarkWatched(expected)
	annotations.AddHashAnnotation(expected)
	// After the update we should see the expected ConfigMap
	afterUpdateCM := &corev1.ConfigMap{}
	err := kClient.Get(ctx,
		types.NamespacedName{
			Name:      desiredObject.Name,
			Namespace: desiredObject.Namespace},
		afterUpdateCM)
	assert.NoError(t, err)
	assert.Equal(t,
		expected.Annotations[k8ssandraapi.ResourceHashAnnotation],
		afterUpdateCM.Annotations[k8ssandraapi.ResourceHashAnnotation],
	)
	assert.Equal(t, "true", afterUpdateCM.Labels[leancache.WatchedLabel])

	assert.NoError(t, err)
	assert.Equal(t, desiredObject.Data["test-key"], afterUpdateCM.Data["test-key"])
	assert.Equal(t, desiredObject.Name, afterUpdateCM.Name)
	assert.Equal(t, desiredObject.Namespace, afterUpdateCM.Namespace)
}

// An object created by an older version without the labels the cache selects on is invisible to
// the cache, so Create reports AlreadyExists. It must be read live and updated with the labels.
func Test_ReconcileObject_UpdatesUncachedObject(t *testing.T) {
	legacy := desiredObject.DeepCopy()
	legacy.Data = map[string]string{"test-key": "old value"}
	kClient := fake.NewClientBuilder().WithObjects(legacy).Build()

	cacheMisses := 0
	cacheBlind := interceptor.NewClient(kClient, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if cacheMisses == 0 {
				cacheMisses++
				return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, key.Name)
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})

	recRes := ReconcileObject(ctx, cacheBlind, requeueDelay, desiredObject)
	assert.False(t, recRes.Completed())

	updated := &corev1.ConfigMap{}
	assert.NoError(t, kClient.Get(ctx, client.ObjectKeyFromObject(legacy), updated))
	assert.Equal(t, "test value", updated.Data["test-key"])
	assert.Equal(t, "true", updated.Labels[leancache.WatchedLabel])
}
