// Package test: this file contains minimum viable configurations for various objects in k8ssandra-operator to facilitate testing.
package test

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// NewK8ssandraCluster returns a minimum viable k8ssandra cluster.
func NewK8ssandraCluster(name string, namespace string) k8ssandraapi.K8ssandraCluster {
	storageClassName := "test-storage-class"
	return k8ssandraapi.K8ssandraCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "K8ssandraCluster",
			APIVersion: "k8ssandra.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: k8ssandraapi.K8ssandraClusterSpec{
			Cassandra: &k8ssandraapi.CassandraClusterTemplate{
				Cluster:         "test-cluster",
				ServerVersion:   "4.0.0",
				CassandraConfig: nil,
				StorageConfig: &cassdcapi.StorageConfig{
					CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
						StorageClassName: &storageClassName,
					},
				},
			},
		},
	}

}

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

func NewCassandraDatacenter() cassdcapi.CassandraDatacenter {
	return cassdcapi.CassandraDatacenter{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CassandraDatacenter",
			APIVersion: "cassandra.datastax.com/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cassdc",
			Namespace: "test-namespace",
		},
		Spec: cassdcapi.CassandraDatacenterSpec{
			Size:          1,
			ServerVersion: "4.0.0",
			ServerType:    "cassandra",
			StorageConfig: cassdcapi.StorageConfig{
				CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
					StorageClassName: nil,
				},
			},
			ClusterName: "test-cluster",
		},
	}
}
