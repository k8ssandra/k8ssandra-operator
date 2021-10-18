package cassandra

import (
	"testing"

	"github.com/stretchr/testify/require"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCoalesce(t *testing.T) {
	storageClass := "default"

	type test struct {
		name string

		clusterTemplate *api.CassandraClusterTemplate

		dcTemplate *api.CassandraDatacenterTemplate

		got *DatacenterConfig

		want *DatacenterConfig
	}

	tests := []test{
		{
			// There are some properties that should only be set at the cluster-level
			// and should not differ among DCs.
			name: "Set non-override configs",
			clusterTemplate: &api.CassandraClusterTemplate{
				Cluster:             "k8ssandra",
				SuperuserSecretName: "test-superuser",
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Labels: map[string]string{
						"env": "dev",
					},
				},
				Size: 3,
			},
			want: &DatacenterConfig{
				Cluster: "k8ssandra",
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Labels: map[string]string{
						"env": "dev",
					},
				},
				SuperUserSecretName: "test-superuser",
				Size:                3,
			},
		},
		{
			name: "Override ServerVersion",
			clusterTemplate: &api.CassandraClusterTemplate{
				ServerVersion: "4.0.0",
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				ServerVersion: "4.0.1",
			},
			want: &DatacenterConfig{
				ServerVersion: "4.0.1",
			},
		},
		{
			name: "Override ServerImage",
			clusterTemplate: &api.CassandraClusterTemplate{
				ServerImage: "k8ssandra/cass-operator:test",
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				ServerImage: "k8ssandra/cass-operator:dev",
			},
			want: &DatacenterConfig{
				ServerImage: "k8ssandra/cass-operator:dev",
			},
		},
		{
			name: "Override Resources",
			clusterTemplate: &api.CassandraClusterTemplate{
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("1024Mi"),
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500m"),
						corev1.ResourceMemory: resource.MustParse("2048Mi"),
					},
				},
			},
			want: &DatacenterConfig{
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500m"),
						corev1.ResourceMemory: resource.MustParse("2048Mi"),
					},
				},
			},
		},
		{
			name: "Override StorageConfig",
			clusterTemplate: &api.CassandraClusterTemplate{
				StorageConfig: &cassdcapi.StorageConfig{
					CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
						StorageClassName: &storageClass,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("2Ti"),
							},
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				StorageConfig: &cassdcapi.StorageConfig{
					CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
						StorageClassName: &storageClass,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("4Ti"),
							},
						},
					},
				},
			},
			want: &DatacenterConfig{
				StorageConfig: &cassdcapi.StorageConfig{
					CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
						StorageClassName: &storageClass,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("4Ti"),
							},
						},
					},
				},
			},
		},
		{
			name: "Override Networking",
			clusterTemplate: &api.CassandraClusterTemplate{
				Networking: &cassdcapi.NetworkingConfig{
					HostNetwork: false,
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Networking: &cassdcapi.NetworkingConfig{
					HostNetwork: true,
				},
			},
			want: &DatacenterConfig{
				Networking: &cassdcapi.NetworkingConfig{
					HostNetwork: true,
				},
			},
		},
		{
			name: "Override CassandraConfig",
			clusterTemplate: &api.CassandraClusterTemplate{
				CassandraConfig: &api.CassandraConfig{
					CassandraYaml: &api.CassandraYaml{
						ConcurrentReads: intPtr(8),
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				CassandraConfig: &api.CassandraConfig{
					CassandraYaml: &api.CassandraYaml{
						ConcurrentWrites: intPtr(8),
					},
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
					},
				},
			},
			want: &DatacenterConfig{
				CassandraConfig: &api.CassandraConfig{
					CassandraYaml: &api.CassandraYaml{
						ConcurrentWrites: intPtr(8),
					},
					JvmOptions: &api.JvmOptions{
						HeapSize: parseResource("1024Mi"),
					},
				},
			},
		},
		{
			name: "Override racks",
			clusterTemplate: &api.CassandraClusterTemplate{
				Racks: []cassdcapi.Rack{
					{
						Name: "rack1",
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Racks: []cassdcapi.Rack{
					{
						Name: "rack1",
					},
					{
						Name: "rack2",
					},
					{
						Name: "rack3",
					},
				},
			},
			want: &DatacenterConfig{
				Racks: []cassdcapi.Rack{
					{
						Name: "rack1",
					},
					{
						Name: "rack2",
					},
					{
						Name: "rack3",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.got = Coalesce(tc.clusterTemplate, tc.dcTemplate)
			require.Equal(t, tc.want, tc.got)
		})
	}
}
