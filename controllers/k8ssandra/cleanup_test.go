package k8ssandra

import (
	"context"
	"testing"

	testlogr "github.com/go-logr/logr/testing"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	k8ssandralabels "github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestK8ssandraClusterReconciler_DeleteServices(t *testing.T) {
	k8sMock := fake.NewFakeClient()
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
			Labels:    k8ssandralabels.PartOfLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}),
		},
	}

	k8sMock.Create(ctx, service)

	res := K8ssandraClusterReconciler{
		Client: k8sMock,
		Scheme: scheme.Scheme,
	}

	hasError := res.deleteServices(ctx, kc, k8ssandraapi.CassandraDatacenterTemplate{}, namespace, k8sMock, logger)

	if hasError != false {
		t.Errorf("Error while deleting services")
	}

	err := k8sMock.Get(ctx, client.ObjectKeyFromObject(service), service)
	if err == nil || !errors.IsNotFound(err) {
		t.Errorf("Service was not deleted: %v", err)
	}

}

func TestK8ssandraClusterReconciler_DeleteDeployments(t *testing.T) {
	k8sMock := fake.NewFakeClient()
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
			Labels:    k8ssandralabels.PartOfLabels(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}),
		},
	}

	k8sMock.Create(ctx, deployment)

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
