package cassandra

import (
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"k8s.io/utils/pointer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Masterminds/semver/v3"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCoalesce(t *testing.T) {
	storageClass := "default"
	mgmtAPIHeap := resource.MustParse("999M")

	type test struct {
		name            string
		clusterName     string
		clusterTemplate *api.CassandraClusterTemplate
		dcTemplate      *api.CassandraDatacenterTemplate
		got             *DatacenterConfig
		want            *DatacenterConfig
	}

	tests := []test{
		{
			// There are some properties that should only be set at the cluster-level
			// and should not differ among DCs.
			name:        "Set non-override configs",
			clusterName: "k8ssandra",
			clusterTemplate: &api.CassandraClusterTemplate{
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				AdditionalSeeds:    []string{"172.18.0.8", "172.18.0.14"},
				ServerType:         "dse",
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Metadata: meta.CassandraDatacenterMeta{
						Tags: meta.Tags{
							Labels: map[string]string{
								"env": "dev",
							},
						},
					},
				},
				Size: 3,
			},
			want: &DatacenterConfig{
				Cluster: "k8ssandra",
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Metadata: meta.CassandraDatacenterMeta{
						Tags: meta.Tags{
							Labels: map[string]string{
								"env": "dev",
							},
						},
					},
				},
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				Size:               3,
				AdditionalSeeds:    []string{"172.18.0.8", "172.18.0.14"},
				ServerType:         "dse",
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override ServerVersion",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "4.0.0",
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerVersion: "4.0.1",
				},
			},
			want: &DatacenterConfig{
				ServerVersion: semver.MustParse("4.0.1"),
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override ServerImage",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerImage: "k8ssandra/cass-operator:test",
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServerImage: "k8ssandra/cass-operator:dev",
				},
			},
			want: &DatacenterConfig{
				ServerImage: "k8ssandra/cass-operator:dev",
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override Resources",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("1024Mi"),
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1500m"),
							corev1.ResourceMemory: resource.MustParse("2048Mi"),
						},
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
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override StorageConfig",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
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
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
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
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override Networking",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Networking: &api.NetworkingConfig{
						HostNetwork: pointer.Bool(false),
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Networking: &api.NetworkingConfig{
						HostNetwork: pointer.Bool(true),
					},
				},
			},
			want: &DatacenterConfig{
				Networking: &cassdcapi.NetworkingConfig{
					HostNetwork: true,
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override CassandraConfig",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					CassandraConfig: &api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"concurrent_reads": 8,
						},
						JvmOptions: api.JvmOptions{
							MaxHeapSize: parseQuantity("512Mi"),
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					CassandraConfig: &api.CassandraConfig{
						CassandraYaml: unstructured.Unstructured{
							"concurrent_writes": 8,
						},
						JvmOptions: api.JvmOptions{
							MaxHeapSize: parseQuantity("1024Mi"),
						},
					},
				},
			},
			want: &DatacenterConfig{
				CassandraConfig: api.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{
						"concurrent_reads":  8,
						"concurrent_writes": 8,
					},
					JvmOptions: api.JvmOptions{
						MaxHeapSize: parseQuantity("1024Mi"),
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override racks",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Racks: []cassdcapi.Rack{
						{
							Name: "rack1",
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
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
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name:        "set management api heap size from DatacenterTemplate",
			clusterName: "k8ssandra",
			clusterTemplate: &api.CassandraClusterTemplate{
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Metadata: meta.CassandraDatacenterMeta{
						Tags: meta.Tags{
							Labels: map[string]string{
								"env": "dev",
							},
						},
					},
				},
				Size: 3,
				DatacenterOptions: api.DatacenterOptions{
					MgmtAPIHeap: &mgmtAPIHeap,
				},
			},
			want: &DatacenterConfig{
				Cluster: "k8ssandra",
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Metadata: meta.CassandraDatacenterMeta{
						Tags: meta.Tags{
							Labels: map[string]string{
								"env": "dev",
							},
						},
					},
				},
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				Size:               3,
				MgmtAPIHeap:        &mgmtAPIHeap,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name:        "set management api heap size from CassandraClusterTemplate",
			clusterName: "k8ssandra",
			clusterTemplate: &api.CassandraClusterTemplate{
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				DatacenterOptions: api.DatacenterOptions{
					MgmtAPIHeap: &mgmtAPIHeap,
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Metadata: meta.CassandraDatacenterMeta{
						Tags: meta.Tags{
							Labels: map[string]string{
								"env": "dev",
							},
						},
					},
				},
				Size: 3,
			},
			want: &DatacenterConfig{
				Cluster: "k8ssandra",
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Metadata: meta.CassandraDatacenterMeta{
						Tags: meta.Tags{
							Labels: map[string]string{
								"env": "dev",
							},
						},
					},
				},
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				Size:               3,
				MgmtAPIHeap:        &mgmtAPIHeap,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override JMX init container",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					JmxInitContainerImage: &images.Image{Name: "cluster-image"},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					JmxInitContainerImage: &images.Image{Name: "dc-image"},
				},
			},
			want: &DatacenterConfig{
				JmxInitContainerImage: &images.Image{Name: "dc-image"},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name:            "Stopped flag",
			clusterTemplate: &api.CassandraClusterTemplate{},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Stopped: true,
			},
			want: &DatacenterConfig{
				Stopped: true,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Additional cluster container",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{},
			want: &DatacenterConfig{
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
							},
							{
								Name: "cassandra",
							},
						},
					},
				},
			},
		},
		{
			name: "Additional cluster container and init containers",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					InitContainers: []corev1.Container{
						{
							Name: "server-config-init",
						},
						{
							Name: "medusa-restore",
						},
						{
							Name:  "test-init-container",
							Image: "test-image",
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test-image",
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{},
			want: &DatacenterConfig{
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{
								Name: "server-config-init",
							},
							{
								Name: "medusa-restore",
							},
							{
								Name:  "test-init-container",
								Image: "test-image",
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
							},
							{
								Name: "cassandra",
							},
						},
					},
				},
			},
		},
		{
			name: "Additional Volumes",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ExtraVolumes: &api.K8ssandraVolumes{
						PVCs: []cassdcapi.AdditionalVolumes{
							{
								Name:      "test-volume",
								MountPath: "/test",
							},
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{},
			want: &DatacenterConfig{
				StorageConfig: &cassdcapi.StorageConfig{
					AdditionalVolumes: []cassdcapi.AdditionalVolumes{
						{
							Name:      "test-volume",
							MountPath: "/test",
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Additional Volumes dc level",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ExtraVolumes: &api.K8ssandraVolumes{ // merged by Name
						Volumes: []corev1.Volume{
							{
								Name: "test-volume",
								VolumeSource: corev1.VolumeSource{ // merged atomically
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						PVCs: []cassdcapi.AdditionalVolumes{ // merged by Name
							{
								Name:      "test-pvc",
								MountPath: "/cluster",
							},
							{
								Name:      "test-pvc2",
								MountPath: "/cluster2",
							},
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ExtraVolumes: &api.K8ssandraVolumes{
						Volumes: []corev1.Volume{
							{
								Name: "test-volume",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/dc",
									},
								},
							},
							{
								Name: "test-volume2",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/dc2",
									},
								},
							},
						},
						PVCs: []cassdcapi.AdditionalVolumes{
							{
								Name:      "test-pvc",
								MountPath: "/dc",
							},
							{
								Name:      "test-pvc3",
								MountPath: "/dc3",
							},
						},
					},
				},
			},
			want: &DatacenterConfig{
				StorageConfig: &cassdcapi.StorageConfig{
					AdditionalVolumes: []cassdcapi.AdditionalVolumes{
						{
							Name:      "test-pvc",
							MountPath: "/dc",
						},
						{
							Name:      "test-pvc2",
							MountPath: "/cluster2",
						},
						{
							Name:      "test-pvc3",
							MountPath: "/dc3",
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "test-volume",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/dc",
									},
								},
							},
							{
								Name: "test-volume2",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/dc2",
									},
								},
							},
						},
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override DseWorkloads",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					DseWorkloads: &cassdcapi.DseWorkloads{AnalyticsEnabled: true},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					DseWorkloads: &cassdcapi.DseWorkloads{GraphEnabled: true},
				},
			},
			want: &DatacenterConfig{
				DseWorkloads: &cassdcapi.DseWorkloads{AnalyticsEnabled: true, GraphEnabled: true},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Containers",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Containers: []corev1.Container{ // merged by Name
						{
							Name:  reconciliation.CassandraContainerName,
							Image: "cassandra-cluster-level-image",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{ // merged by MountPath
								{
									Name:      "cluster-volume-mount",
									MountPath: "/volume-mount1",
								},
							},
							Env: []corev1.EnvVar{ // merged by Name
								{
									Name:  "ENV1",
									Value: "cluster",
								},
								{
									Name:  "ENV2",
									Value: "cluster",
								},
							},
						},
						{
							Name:            reconciliation.ServerConfigContainerName,
							Image:           "config-cluster-level-image",
							ImagePullPolicy: corev1.PullAlways,
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Containers: []corev1.Container{
						{
							Name:  reconciliation.CassandraContainerName,
							Image: "cassandra-dc-level-image",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dc-volume-mount",
									MountPath: "/volume-mount1",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "ENV1",
									Value: "dc",
								},
								{
									Name:  "ENV3",
									Value: "dc",
								},
							},
						},
						{
							Name:  reconciliation.ServerConfigContainerName,
							Image: "config-dc-level-image",
						},
					},
				},
			},
			want: &DatacenterConfig{
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  reconciliation.CassandraContainerName,
								Image: "cassandra-dc-level-image",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "dc-volume-mount",
										MountPath: "/volume-mount1",
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "ENV1",
										Value: "dc",
									},
									{
										Name:  "ENV2",
										Value: "cluster",
									},
									{
										Name:  "ENV3",
										Value: "dc",
									},
								},
							},
							{
								Name:            reconciliation.ServerConfigContainerName,
								Image:           "config-dc-level-image",
								ImagePullPolicy: corev1.PullAlways,
							},
						},
					},
				},
			},
		},
		{
			name: "Init Containers",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					InitContainers: []corev1.Container{ // merged by Name
						{
							Name:  "container1",
							Image: "container1-cluster-level-image",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
						{
							Name:            "container2",
							Image:           "container2-cluster-level-image",
							ImagePullPolicy: corev1.PullAlways,
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					InitContainers: []corev1.Container{
						{
							Name:  "container1",
							Image: "container1-dc-level-image",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
						{
							Name:  "container2",
							Image: "container2-dc-level-image",
						},
					},
				},
			},
			want: &DatacenterConfig{
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
						InitContainers: []corev1.Container{
							{
								Name:  "container1",
								Image: "container1-dc-level-image",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
								},
							},
							{
								Name:            "container2",
								Image:           "container2-dc-level-image",
								ImagePullPolicy: corev1.PullAlways,
							},
						},
					},
				},
			},
		},
		{
			name: "Override PerNodeConfigInitContainerImage",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					PerNodeConfigInitContainerImage: "cluster-level:latest",
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					PerNodeConfigInitContainerImage: "dc-level:latest",
				},
			},
			want: &DatacenterConfig{
				PerNodeInitContainerImage: "dc-level:latest",
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Set Cluster Pod Tags",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: meta.CassandraDatacenterMeta{
					CommonLabels: map[string]string{
						"common": "label",
					},
					Pods: meta.Tags{
						Labels:      map[string]string{"label": "lvalue"},
						Annotations: map[string]string{"annotation:": "avalue"},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name: "",
					Metadata: meta.CassandraDatacenterMeta{
						CommonLabels: map[string]string{
							"common": "label",
						},
						Pods: meta.Tags{
							Labels:      map[string]string{"label": "lvalue"},
							Annotations: map[string]string{"annotation:": "avalue"},
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      map[string]string{"label": "lvalue"},
						Annotations: map[string]string{"annotation:": "avalue"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Set DC Pod Tags",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: api.EmbeddedObjectMeta{
					Name: "",
					Metadata: meta.CassandraDatacenterMeta{
						CommonLabels: map[string]string{
							"common": "label",
						},
						Pods: meta.Tags{
							Labels:      map[string]string{"label": "lvalue"},
							Annotations: map[string]string{"annotation:": "avalue"},
						},
					},
				},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name: "",
					Metadata: meta.CassandraDatacenterMeta{
						CommonLabels: map[string]string{
							"common": "label",
						},
						Pods: meta.Tags{
							Labels:      map[string]string{"label": "lvalue"},
							Annotations: map[string]string{"annotation:": "avalue"},
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      map[string]string{"label": "lvalue"},
						Annotations: map[string]string{"annotation:": "avalue"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Set Cluster & DC Pod Tags",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: meta.CassandraDatacenterMeta{
					Pods: meta.Tags{
						Labels:      map[string]string{"label": "lvalue", "cluster": "cluster"},
						Annotations: map[string]string{"annotation:": "avalue", "cluster": "cluster"},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: api.EmbeddedObjectMeta{
					Name: "",
					Metadata: meta.CassandraDatacenterMeta{
						Pods: meta.Tags{
							Labels:      map[string]string{"label": "dcvalue", "dc": "dc"},
							Annotations: map[string]string{"annotation:": "dcvalue", "dc": "dc"},
						},
					},
				},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name: "",
					Metadata: meta.CassandraDatacenterMeta{
						Pods: meta.Tags{
							Labels:      map[string]string{"label": "dcvalue", "dc": "dc", "cluster": "cluster"},
							Annotations: map[string]string{"annotation:": "dcvalue", "dc": "dc", "cluster": "cluster"},
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      map[string]string{"label": "dcvalue", "cluster": "cluster", "dc": "dc"},
						Annotations: map[string]string{"annotation:": "dcvalue", "cluster": "cluster", "dc": "dc"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Set Cluster Service Tags",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: meta.CassandraDatacenterMeta{
					CommonLabels: map[string]string{
						"common": "label",
					},
					ServiceConfig: meta.CassandraDatacenterServicesMeta{
						DatacenterService: meta.Tags{
							Labels:      map[string]string{"dclabel": "dcvalue"},
							Annotations: map[string]string{"dcannotation:": "dcvalue"},
						},
						SeedService: meta.Tags{
							Labels:      map[string]string{"seedlabel": "seedvalue"},
							Annotations: map[string]string{"seedannotation:": "seedvalue"},
						},
						AdditionalSeedService: meta.Tags{
							Labels:      map[string]string{"aslabel": "asvalue"},
							Annotations: map[string]string{"asannotation:": "asvalue"},
						},
						AllPodsService: meta.Tags{
							Labels:      map[string]string{"aplabel": "apvalue"},
							Annotations: map[string]string{"apannotation:": "apvalue"},
						},
						NodePortService: meta.Tags{
							Labels:      map[string]string{"nodeportlabel": "nodeportval"},
							Annotations: map[string]string{"nodeportann:": "nodeportvalue"},
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Metadata: meta.CassandraDatacenterMeta{
						CommonLabels: map[string]string{
							"common": "label",
						},
						ServiceConfig: meta.CassandraDatacenterServicesMeta{
							DatacenterService: meta.Tags{
								Labels:      map[string]string{"dclabel": "dcvalue"},
								Annotations: map[string]string{"dcannotation:": "dcvalue"},
							},
							SeedService: meta.Tags{
								Labels:      map[string]string{"seedlabel": "seedvalue"},
								Annotations: map[string]string{"seedannotation:": "seedvalue"},
							},
							AdditionalSeedService: meta.Tags{
								Labels:      map[string]string{"aslabel": "asvalue"},
								Annotations: map[string]string{"asannotation:": "asvalue"},
							},
							AllPodsService: meta.Tags{
								Labels:      map[string]string{"aplabel": "apvalue"},
								Annotations: map[string]string{"apannotation:": "apvalue"},
							},
							NodePortService: meta.Tags{
								Labels:      map[string]string{"nodeportlabel": "nodeportval"},
								Annotations: map[string]string{"nodeportann:": "nodeportvalue"},
							},
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Set DC Service Tags",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: api.EmbeddedObjectMeta{
					Metadata: meta.CassandraDatacenterMeta{
						CommonLabels: map[string]string{
							"common": "label",
						},
						ServiceConfig: meta.CassandraDatacenterServicesMeta{
							DatacenterService: meta.Tags{
								Labels:      map[string]string{"dclabel": "dcvalue"},
								Annotations: map[string]string{"dcannotation:": "dcvalue"},
							},
							SeedService: meta.Tags{
								Labels:      map[string]string{"seedlabel": "seedvalue"},
								Annotations: map[string]string{"seedannotation:": "seedvalue"},
							},
							AdditionalSeedService: meta.Tags{
								Labels:      map[string]string{"aslabel": "asvalue"},
								Annotations: map[string]string{"asannotation:": "asvalue"},
							},
							AllPodsService: meta.Tags{
								Labels:      map[string]string{"aplabel": "apvalue"},
								Annotations: map[string]string{"apannotation:": "apvalue"},
							},
							NodePortService: meta.Tags{
								Labels:      map[string]string{"nodeportlabel": "nodeportval"},
								Annotations: map[string]string{"nodeportann:": "nodeportvalue"},
							},
						},
					},
				},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Metadata: meta.CassandraDatacenterMeta{
						CommonLabels: map[string]string{
							"common": "label",
						},
						ServiceConfig: meta.CassandraDatacenterServicesMeta{
							DatacenterService: meta.Tags{
								Labels:      map[string]string{"dclabel": "dcvalue"},
								Annotations: map[string]string{"dcannotation:": "dcvalue"},
							},
							SeedService: meta.Tags{
								Labels:      map[string]string{"seedlabel": "seedvalue"},
								Annotations: map[string]string{"seedannotation:": "seedvalue"},
							},
							AdditionalSeedService: meta.Tags{
								Labels:      map[string]string{"aslabel": "asvalue"},
								Annotations: map[string]string{"asannotation:": "asvalue"},
							},
							AllPodsService: meta.Tags{
								Labels:      map[string]string{"aplabel": "apvalue"},
								Annotations: map[string]string{"apannotation:": "apvalue"},
							},
							NodePortService: meta.Tags{
								Labels:      map[string]string{"nodeportlabel": "nodeportval"},
								Annotations: map[string]string{"nodeportann:": "nodeportvalue"},
							},
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Set Cluster & DC Service Tags",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: meta.CassandraDatacenterMeta{
					ServiceConfig: meta.CassandraDatacenterServicesMeta{
						DatacenterService: meta.Tags{
							Labels:      map[string]string{"dclabel": "cl_value"},
							Annotations: map[string]string{"dcannotation:": "cl_value"},
						},
						SeedService: meta.Tags{
							Labels:      map[string]string{"seedlabel": "cl_seedvalue"},
							Annotations: map[string]string{"seedannotation:": "cl_seedvalue"},
						},
						AdditionalSeedService: meta.Tags{
							Labels:      map[string]string{"aslabel": "cl_asvalue"},
							Annotations: map[string]string{"asannotation:": "cl_asvalue"},
						},
						AllPodsService: meta.Tags{
							Labels:      map[string]string{"aplabel": "cl_apvalue"},
							Annotations: map[string]string{"apannotation:": "cl_apvalue"},
						},
						NodePortService: meta.Tags{
							Labels:      map[string]string{"cl_nodeportlabel": "nodeportval"},
							Annotations: map[string]string{"cl_nodeportann:": "nodeportvalue"},
						},
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{},
				Meta: api.EmbeddedObjectMeta{
					Metadata: meta.CassandraDatacenterMeta{
						ServiceConfig: meta.CassandraDatacenterServicesMeta{
							DatacenterService: meta.Tags{
								Labels:      map[string]string{"dclabel": "dcvalue"},
								Annotations: map[string]string{"dcannotation:": "dcvalue"},
							},
							SeedService: meta.Tags{
								Labels:      map[string]string{"seedlabel": "seedvalue"},
								Annotations: map[string]string{"seedannotation:": "seedvalue"},
							},
							AdditionalSeedService: meta.Tags{
								Labels:      map[string]string{"aslabel": "asvalue"},
								Annotations: map[string]string{"asannotation:": "asvalue"},
							},
							AllPodsService: meta.Tags{
								Labels:      map[string]string{"aplabel": "apvalue"},
								Annotations: map[string]string{"apannotation:": "apvalue"},
							},
							NodePortService: meta.Tags{
								Labels:      map[string]string{"nodeportlabel": "nodeportval"},
								Annotations: map[string]string{"nodeportann:": "nodeportvalue"},
							},
						},
					},
				},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Metadata: meta.CassandraDatacenterMeta{
						ServiceConfig: meta.CassandraDatacenterServicesMeta{
							DatacenterService: meta.Tags{
								Labels:      map[string]string{"dclabel": "dcvalue"},
								Annotations: map[string]string{"dcannotation:": "dcvalue"},
							},
							SeedService: meta.Tags{
								Labels:      map[string]string{"seedlabel": "seedvalue"},
								Annotations: map[string]string{"seedannotation:": "seedvalue"},
							},
							AdditionalSeedService: meta.Tags{
								Labels:      map[string]string{"aslabel": "asvalue"},
								Annotations: map[string]string{"asannotation:": "asvalue"},
							},
							AllPodsService: meta.Tags{
								Labels:      map[string]string{"aplabel": "apvalue"},
								Annotations: map[string]string{"apannotation:": "apvalue"},
							},
							NodePortService: meta.Tags{
								Labels:      map[string]string{"nodeportlabel": "nodeportval", "cl_nodeportlabel": "nodeportval"},
								Annotations: map[string]string{"nodeportann:": "nodeportvalue", "cl_nodeportann:": "nodeportvalue"},
							},
						},
					},
				},
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.got = Coalesce(tc.clusterName, tc.clusterTemplate, tc.dcTemplate)
			require.Equal(t, tc.want, tc.got)
		})
	}
}

// TestNewDatacenter_MgmtAPIHeapSize_Set tests that the podTemplateSpec is populated with a `cassandra` container and associated environment variables
// when a management API heap size is set.
func TestNewDatacenter_MgmtAPIHeapSize_Set(t *testing.T) {
	template := GetDatacenterConfig()
	mgmtAPIHeap := resource.MustParse("999M")
	template.MgmtAPIHeap = &mgmtAPIHeap
	dc, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.Equal(t, err, nil)
	assert.Equal(t, dc.Spec.PodTemplateSpec.Spec.Containers[0].Env[0].Value, "999000000")
}

// TestNewDatacenter_MgmtAPIHeapSize_Unset tests that the podTemplateSpec remains empty when no management API heap size is set.
func TestNewDatacenter_MgmtAPIHeapSize_Unset(t *testing.T) {
	template := GetDatacenterConfig()
	dc, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.Equal(t, err, nil)
	assert.Equal(t, &corev1.PodTemplateSpec{}, dc.Spec.PodTemplateSpec)
}

func TestNewDatacenter_AllowMultipleCassPerNodeSet(t *testing.T) {
	template := GetDatacenterConfig()
	template.SoftPodAntiAffinity = pointer.Bool(true)
	dc, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.NoError(t, err)
	assert.Equal(t, true, dc.Spec.AllowMultipleNodesPerWorker)
}

func TestNewDatacenter_Tolerations(t *testing.T) {
	template := GetDatacenterConfig()
	template.Tolerations = []corev1.Toleration{{
		Key:      "key1",
		Operator: corev1.TolerationOpEqual,
		Value:    "value1",
		Effect:   corev1.TaintEffectNoSchedule,
	}}
	dc, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.NoError(t, err)
	assert.Equal(t, template.Tolerations, dc.Spec.Tolerations)
}

// TestValidateCoalesced_Fail_NoStorageConfig tests that NewDatacenter fails when no storage config is provided.
func TestValidateDatacenterConfig_Fail_NoStorageConfig(t *testing.T) {
	template := GetDatacenterConfig()
	template.StorageConfig = nil
	err := ValidateDatacenterConfig(&template)
	assert.IsType(t, DCConfigIncomplete{}, err)
}

// TestValidateCoalesced_Fail_NoServerVersion tests that NewDatacenter fails when no server version is provided.
func TestValidateDatacenterConfig_Fail_NoServerVersion(t *testing.T) {
	template := GetDatacenterConfig()
	template.ServerVersion = nil
	err := ValidateDatacenterConfig(&template)
	assert.IsType(t, DCConfigIncomplete{}, err)
}

func TestCDC(t *testing.T) {
	template := GetDatacenterConfig()
	template.CDC = &cassdcapi.CDCConfiguration{
		PulsarServiceUrl: pointer.String("pulsar://test-url"),
	}
	cassDC, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.NoError(t, err)
	assert.Equal(t, cassDC.Spec.CDC, template.CDC)
}

func TestDatacentersReplication(t *testing.T) {
	assert := assert.New(t)

	replication := &Replication{
		datacenters: map[string]keyspacesReplication{
			"dc2": {
				"ks1": 3,
				"ks2": 3,
			},
			"dc3": {
				"ks1": 5,
				"ks2": 1,
				"ks3": 7,
			},
			"dc4": {
				"ks1": 1,
				"ks2": 3,
				"ks3": 3,
				"ks4": 1,
			},
		},
	}

	assert.True(replication.EachDcContainsKeyspaces("ks1", "ks2"))
	assert.False(replication.EachDcContainsKeyspaces("ks1", "ks2", "ks3"))

	expected := &Replication{
		datacenters: map[string]keyspacesReplication{
			"dc3": {
				"ks1": 5,
				"ks2": 1,
				"ks3": 7,
			},
			"dc4": {
				"ks1": 1,
				"ks2": 3,
				"ks3": 3,
				"ks4": 1,
			},
		},
	}
	assert.Equal(expected, replication.ForDcs("dc3", "dc4", "dc5"))

	expected = &Replication{datacenters: map[string]keyspacesReplication{}}
	assert.Equal(expected, replication.ForDcs("dc5", "dc6"))
}

// GetDatacenterConfig returns a minimum viable DataCenterConfig.
//
//goland:noinspection GoDeprecation
func GetDatacenterConfig() DatacenterConfig {
	storageClass := "default"
	return DatacenterConfig{
		Cluster:       "k8ssandra",
		ServerVersion: semver.MustParse("4.0.3"),
		ServerType:    "cassandra",
		Meta: api.EmbeddedObjectMeta{
			Namespace: "k8ssandra",
			Name:      "dc1",
			Metadata: meta.CassandraDatacenterMeta{
				Tags: meta.Tags{
					Labels: map[string]string{
						"env": "dev",
					},
				},
			},
		},
		SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
		Size:               3,
		CassandraConfig: api.CassandraConfig{
			JvmOptions: api.JvmOptions{
				AdditionalOptions: []string{
					SystemReplicationFactorStrategy + "=dc1:3",
				},
			},
		},
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
	}
}
