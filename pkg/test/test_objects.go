// Package test: this file contains minimum viable configurations for various objects in k8ssandra-operator to facilitate testing.
package test

import (
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func NewCassandraDatacenter(name string, namespace string) cassdcapi.CassandraDatacenter {
	return cassdcapi.CassandraDatacenter{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CassandraDatacenter",
			APIVersion: "cassandra.datastax.com/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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

func NewStargate(name string, namespace string) stargateapi.Stargate {
	return stargateapi.Stargate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "stargate.k8ssandra.io/v1alpha1",
			Kind:       "Stargate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: stargateapi.StargateSpec{
			StargateDatacenterTemplate: stargateapi.StargateDatacenterTemplate{
				StargateClusterTemplate: stargateapi.StargateClusterTemplate{
					StargateTemplate: stargateapi.StargateTemplate{
						AllowStargateOnDataNodes: true,
					},
					Size: 1,
				},
			},
		},
	}
}

func NewMedusaBackup(namespace string, backupName string, specBackupName string, dc string) *medusaapi.CassandraBackup {
	return &medusaapi.CassandraBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      backupName,
		},
		Spec: medusaapi.CassandraBackupSpec{
			Name:                specBackupName,
			CassandraDatacenter: dc,
		},
	}
}

func NewMedusaRestore(namespace string, localRestoreName string, remoteBackupName string, dc string, clusterName string) *medusaapi.CassandraRestore {
	return &medusaapi.CassandraRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      localRestoreName,
		},
		Spec: medusaapi.CassandraRestoreSpec{
			Backup:   remoteBackupName,
			Shutdown: true,
			InPlace:  true,
			CassandraDatacenter: medusaapi.CassandraDatacenterConfig{
				Name:        dc,
				ClusterName: clusterName,
			},
		},
	}
}
