package test

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// NewFakeClient gets a fake client loaded up with a scheme that contains all the APIs used in this project.
func NewFakeClient() (client.Client, error) {
	schemeBuilder := scheme.Builder{}
	testScheme, err := schemeBuilder.Build()
	if err != nil {
		return nil, err
	}
	utilruntime.Must(promapi.AddToScheme(testScheme))
	utilruntime.Must(cassdcapi.AddToScheme(testScheme))
	utilruntime.Must(k8ssandraapi.AddToScheme(testScheme))
	utilruntime.Must(reaperapi.AddToScheme(testScheme))
	utilruntime.Must(stargateapi.AddToScheme(testScheme))
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		Build()
	return fakeClient, nil
}

// Some magic to override the RESTMapper().KindsFor(...) call. fake client blows up with a panic otherwise.
type composedClient struct {
	client.Client
}
type fakeRESTMapper struct {
	meta.RESTMapper
}

func (c composedClient) RESTMapper() meta.RESTMapper {
	return fakeRESTMapper{}
}
func (rm fakeRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return []schema.GroupVersionKind{
		{
			Group:   promapi.SchemeGroupVersion.Group,
			Version: promapi.Version,
			Kind:    promapi.ServiceMonitorsKind,
		},
	}, nil
}
func NewFakeClientWRestMapper() client.Client {
	fakeClient, _ := NewFakeClient()
	return composedClient{fakeClient}
}
