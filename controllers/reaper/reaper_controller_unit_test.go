package reaper

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileDeploymentCreatesDesiredDeployment(t *testing.T) {
	testScheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(testScheme))
	require.NoError(t, appsv1.AddToScheme(testScheme))
	require.NoError(t, reaperapi.AddToScheme(testScheme))

	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	reconciler := &ReaperReconciler{
		ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second},
		Client:           fakeClient,
		Scheme:           testScheme,
		ImageRegistry:    getTestImageRegistry(),
	}
	actualReaper := newReaper("test")
	actualReaper.Spec.StorageType = reaperapi.StorageTypeCassandra
	actualReaper.Spec.DatacenterRef = reaperapi.CassandraDatacenterRef{}

	result, err := reconciler.reconcileDeployment(
		context.Background(),
		actualReaper,
		&cassdcapi.CassandraDatacenter{},
		logr.Discard(),
	)

	require.NoError(t, err)
	require.Equal(t, time.Second, result.RequeueAfter)
	deployment := &appsv1.Deployment{}
	require.NoError(t, fakeClient.Get(
		context.Background(),
		client.ObjectKey{Namespace: actualReaper.Namespace, Name: actualReaper.Name},
		deployment,
	))
}
