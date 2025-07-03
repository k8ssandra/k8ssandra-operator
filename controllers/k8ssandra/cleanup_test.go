package k8ssandra

import (
	"context"
	"testing"
	"time"

	testlogr "github.com/go-logr/logr/testing"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/clientcache"
	"github.com/k8ssandra/k8ssandra-operator/pkg/config"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestK8ssandraClusterReconciler_DeleteServices(t *testing.T) {
	k8sMock := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	ctx := context.Background()
	logger := testlogr.NewTestLogger(t)

	kc := &k8ssandraapi.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
		},
	}

	namespace := "test-namespace"

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-1",
			Namespace: "test-namespace",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}),
		},
	}

	require.NoError(t, k8sMock.Create(ctx, service))

	res := K8ssandraClusterReconciler{
		Client: k8sMock,
		Scheme: scheme.Scheme,
	}

	hasError := res.deleteServices(ctx, kc, k8ssandraapi.CassandraDatacenterTemplate{}, namespace, k8sMock, logger)
	require.False(t, hasError, "Error while deleting services")

	err := k8sMock.Get(ctx, client.ObjectKeyFromObject(service), service)
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err))
}

func TestK8ssandraClusterReconciler_DeleteDeployments(t *testing.T) {
	k8sMock := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	ctx := context.Background()
	logger := testlogr.NewTestLogger(t)

	kc := &k8ssandraapi.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test-namespace",
		},
	}

	namespace := "test-namespace"

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-1",
			Namespace: "test-namespace",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}),
		},
	}

	require.NoError(t, k8sMock.Create(ctx, deployment))

	res := K8ssandraClusterReconciler{
		Client: k8sMock,
		Scheme: scheme.Scheme,
	}

	hasError := res.deleteDeployments(ctx, kc, k8ssandraapi.CassandraDatacenterTemplate{}, namespace, k8sMock, logger)

	if hasError != false {
		t.Errorf("Error while deleting deployments")
	}

	err := k8sMock.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)

	if err == nil || !errors.IsNotFound(err) {
		t.Errorf("Deployment was not deleted: %v", err)
	}

}

func TestK8ssandraClusterReconciler_CheckDeletion(t *testing.T) {
	// Setup scheme with all required types
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, k8ssandraapi.AddToScheme(s))
	require.NoError(t, cassdcapi.AddToScheme(s))
	require.NoError(t, reaperapi.AddToScheme(s))
	require.NoError(t, stargateapi.AddToScheme(s))

	ctx := context.Background()
	logger := testlogr.NewTestLogger(t)

	// Create a K8ssandraCluster with deletion timestamp and finalizer
	kc := &k8ssandraapi.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster",
			Namespace:         "test-namespace",
			DeletionTimestamp: &metav1.Time{Time: time.Now()},
			Finalizers:        []string{k8ssandraClusterFinalizer},
		},
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				Datacenters: []k8ssandraapi.CassandraDatacenterTemplate{
					{
						Meta: k8ssandraapi.EmbeddedObjectMeta{
							Name: "dc1",
						},
						K8sContext: "default",
					},
				},
			},
		},
	}

	// Create a CassandraDatacenter that should be deleted
	dc := &cassdcapi.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dc1",
			Namespace: "test-namespace",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}),
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			ClusterName: "test-cluster",
		},
	}

	// Create other resources that should be cleaned up first
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "test-namespace",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}),
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-namespace",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}),
		},
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.SanitizedName()}),
		},
	}

	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cronjob",
			Namespace: "test-namespace",
			Labels:    k8ssandralabels.CleanedUpByLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.SanitizedName()}),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test",
									Image: "test:latest",
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}

	// Create mock client with all resources
	k8sMock := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(kc, dc, service, deployment, configMap, cronJob).
		Build()

	// Create client cache
	clientCache := clientcache.New(k8sMock, k8sMock, s)
	clientCache.AddClient("default", k8sMock)

	// Create reconciler
	reconciler := &K8ssandraClusterReconciler{
		ReconcilerConfig: &config.ReconcilerConfig{DefaultDelay: time.Second},
		Client:           k8sMock,
		Scheme:           s,
		ClientCache:      clientCache,
	}

	// First call to checkDeletion - should clean up other resources and delete DC
	result := reconciler.checkDeletion(ctx, kc, logger)

	// Should requeue because DC still exists (simulating real deletion takes time)
	require.True(t, result.IsRequeue())
	ctrlResult, resultErr := result.Output()
	require.NoError(t, resultErr)
	require.True(t, ctrlResult.Requeue)
	require.Equal(t, time.Second, ctrlResult.RequeueAfter)

	// Verify other resources were deleted
	err := k8sMock.Get(ctx, client.ObjectKeyFromObject(service), service)
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err), "Service should be deleted")

	err = k8sMock.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err), "Deployment should be deleted")

	err = k8sMock.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err), "ConfigMap should be deleted")

	err = k8sMock.Get(ctx, client.ObjectKeyFromObject(cronJob), cronJob)
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err), "CronJob should be deleted")

	// Verify CassandraDatacenter was deleted
	err = k8sMock.Get(ctx, client.ObjectKeyFromObject(dc), dc)
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err), "CassandraDatacenter should be deleted")

	// Verify finalizer is still present (since we requeued)
	err = k8sMock.Get(ctx, client.ObjectKeyFromObject(kc), kc)
	require.NoError(t, err)
	require.True(t, controllerutil.ContainsFinalizer(kc, k8ssandraClusterFinalizer), "Finalizer should still be present")

	// Second call to checkDeletion - DC is now gone, should remove finalizer
	result = reconciler.checkDeletion(ctx, kc, logger)

	// Should be done now
	require.True(t, result.IsDone())
	require.False(t, result.IsRequeue())
	require.False(t, result.IsError())

	// Verify finalizer was removed
	err = k8sMock.Get(ctx, client.ObjectKeyFromObject(kc), kc)
	require.Error(t, err)
	require.True(t, errors.IsNotFound(err), "K8ssandraCluster should be deleted after finalizer is removed")
}
