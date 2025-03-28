package cassandra

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/cass-operator/pkg/reconciliation"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
					Tags: meta.Tags{
						Labels: map[string]string{
							"env": "dev",
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
					Tags: meta.Tags{
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				Size:               3,
				AdditionalSeeds:    []string{"172.18.0.8", "172.18.0.14"},
				ServerType:         "dse",
				McacEnabled:        true,
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
				McacEnabled:   true,
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
				McacEnabled: true,
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
				McacEnabled: true,
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
							Resources: corev1.VolumeResourceRequirements{
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
							Resources: corev1.VolumeResourceRequirements{
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
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("4Ti"),
							},
						},
					},
				},
				McacEnabled: true,
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
						HostNetwork: ptr.To(false),
					},
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					Networking: &api.NetworkingConfig{
						HostNetwork: ptr.To(true),
					},
				},
			},
			want: &DatacenterConfig{
				Networking: &cassdcapi.NetworkingConfig{
					HostNetwork: true,
				},
				McacEnabled: true,
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
				McacEnabled: true,
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
				McacEnabled: true,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name:        "set management api heap size and MCAC disabled from DatacenterTemplate",
			clusterName: "k8ssandra",
			clusterTemplate: &api.CassandraClusterTemplate{
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Tags: meta.Tags{
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				Size: 3,
				DatacenterOptions: api.DatacenterOptions{
					MgmtAPIHeap: &mgmtAPIHeap,
					Telemetry: &v1alpha1.TelemetrySpec{
						Mcac: &v1alpha1.McacTelemetrySpec{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			want: &DatacenterConfig{
				Cluster: "k8ssandra",
				Meta: api.EmbeddedObjectMeta{
					Namespace: "k8ssandra",
					Name:      "dc1",
					Tags: meta.Tags{
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				Size:               3,
				MgmtAPIHeap:        &mgmtAPIHeap,
				McacEnabled:        false,
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
					Tags: meta.Tags{
						Labels: map[string]string{
							"env": "dev",
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
					Tags: meta.Tags{
						Labels: map[string]string{
							"env": "dev",
						},
					},
				},
				SuperuserSecretRef: corev1.LocalObjectReference{Name: "test-superuser"},
				Size:               3,
				MgmtAPIHeap:        &mgmtAPIHeap,
				McacEnabled:        true,
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
				Stopped:     true,
				McacEnabled: true,
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
				McacEnabled: true,
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
				McacEnabled: true,
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
				McacEnabled: true,
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
				McacEnabled: true,
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
				McacEnabled:  true,
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
				McacEnabled: true,
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
				McacEnabled: true,
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
				McacEnabled:               true,
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
				Meta: meta.CassandraClusterMeta{
					CommonLabels: map[string]string{
						"common": "label",
					},
					CommonAnnotations: map[string]string{
						"common": "annotation",
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
					CommonLabels: map[string]string{
						"common": "label",
					},
					CommonAnnotations: map[string]string{
						"common": "annotation",
					},
					Pods: meta.Tags{
						Labels:      map[string]string{"label": "lvalue"},
						Annotations: map[string]string{"annotation:": "avalue"},
					},
				},
				McacEnabled: true,
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
					CommonLabels: map[string]string{
						"common": "label",
					},
					CommonAnnotations: map[string]string{
						"common": "annotation",
					},
					Pods: meta.Tags{
						Labels:      map[string]string{"label": "lvalue"},
						Annotations: map[string]string{"annotation:": "avalue"},
					},
				},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name: "",
					CommonLabels: map[string]string{
						"common": "label",
					},
					CommonAnnotations: map[string]string{
						"common": "annotation",
					},
					Pods: meta.Tags{
						Labels:      map[string]string{"label": "lvalue"},
						Annotations: map[string]string{"annotation:": "avalue"},
					},
				},
				McacEnabled: true,
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
				Meta: meta.CassandraClusterMeta{
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
					Pods: meta.Tags{
						Labels:      map[string]string{"label": "dcvalue", "dc": "dc"},
						Annotations: map[string]string{"annotation:": "dcvalue", "dc": "dc"},
					},
				},
			},
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
					Name: "",
					Pods: meta.Tags{
						Labels:      map[string]string{"label": "dcvalue", "dc": "dc", "cluster": "cluster"},
						Annotations: map[string]string{"annotation:": "dcvalue", "dc": "dc", "cluster": "cluster"},
					},
				},
				McacEnabled: true,
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
				Meta: meta.CassandraClusterMeta{
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
				McacEnabled: true,
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
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
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
				McacEnabled: true,
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
				Meta: meta.CassandraClusterMeta{
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
			want: &DatacenterConfig{
				Meta: api.EmbeddedObjectMeta{
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
				McacEnabled: true,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Override service account",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServiceAccount: "cluster_account",
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ServiceAccount: "dc_account",
				},
			},
			want: &DatacenterConfig{
				ServiceAccount: "dc_account",
				McacEnabled:    true,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "cassandra"}},
					},
				},
			},
		},
		{
			name: "Set priority class name at cluster level",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					PodPriorityClassName: "mock-priority",
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{},
			want: &DatacenterConfig{
				McacEnabled: true,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers:        []corev1.Container{{Name: "cassandra"}},
						PriorityClassName: "mock-priority",
					},
				},
			},
		},
		{
			name: "Override priority class name",
			clusterTemplate: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					PodPriorityClassName: "ignored-priority",
				},
			},
			dcTemplate: &api.CassandraDatacenterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					PodPriorityClassName: "mock-priority",
				},
			},
			want: &DatacenterConfig{
				McacEnabled: true,
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers:        []corev1.Container{{Name: "cassandra"}},
						PriorityClassName: "mock-priority",
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

func TestNewDatacenter_McacDisabled_Set(t *testing.T) {
	template := GetDatacenterConfig()
	template.McacEnabled = false
	dc, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.Equal(t, err, nil)
	cassContainerIdx, found := FindContainer(dc.Spec.PodTemplateSpec, "cassandra")
	if !found {
		assert.Fail(t, "could not find cassandra container")
	}

	found = false
	for _, envVar := range dc.Spec.PodTemplateSpec.Spec.Containers[cassContainerIdx].Env {
		if envVar.Name == mcacDisabledEnvVar {
			found = true
		}
	}
	assert.True(t, found, "could not find expected MCAC disabled environment variable")
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
	template.SoftPodAntiAffinity = ptr.To(true)
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

func TestNewDatacenter_ServiceAccount(t *testing.T) {
	template := GetDatacenterConfig()
	template.ServiceAccount = "svc"
	dc, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.NoError(t, err)
	assert.Equal(t, template.ServiceAccount, dc.Spec.ServiceAccountName)
}

func TestNewDatacenter_PodPriorityClassName(t *testing.T) {
	template := GetDatacenterConfig()
	template.PodTemplateSpec.Spec.PriorityClassName = "mock-priority"
	dc, err := NewDatacenter(
		types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
		&template,
	)
	assert.NoError(t, err)
	assert.Equal(t, "mock-priority", dc.Spec.PodTemplateSpec.Spec.PriorityClassName)
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
		PulsarServiceUrl: ptr.To("pulsar://test-url"),
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
			Tags: meta.Tags{
				Labels: map[string]string{
					"env": "dev",
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
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("4Ti"),
					},
				},
			},
		},
		McacEnabled: true,
	}
}

func TestSetNewDefaultNumTokens(t *testing.T) {
	testCases := []struct {
		name                string
		kc                  *api.K8ssandraCluster
		actualDcConfig      *unstructured.Unstructured
		desiredDcConfig     *unstructured.Unstructured
		expectedDcNumTokens float64
	}{
		{
			name: "num_tokens exists in CassandraConfig",
			kc: &api.K8ssandraCluster{
				Spec: api.K8ssandraClusterSpec{
					Cassandra: &api.CassandraClusterTemplate{
						DatacenterOptions: api.DatacenterOptions{
							CassandraConfig: &api.CassandraConfig{
								CassandraYaml: unstructured.Unstructured{"num_tokens": 33},
							},
						},
					},
				},
			},
			actualDcConfig:      &unstructured.Unstructured{"num_tokens": 24},
			desiredDcConfig:     &unstructured.Unstructured{"num_tokens": 33},
			expectedDcNumTokens: 33,
		},
		{
			name: "num_tokens does not exist, set to value from actualDc",
			kc: &api.K8ssandraCluster{
				Spec: api.K8ssandraClusterSpec{
					Cassandra: &api.CassandraClusterTemplate{
						DatacenterOptions: api.DatacenterOptions{
							CassandraConfig: &api.CassandraConfig{
								CassandraYaml: unstructured.Unstructured{"other": "setting"},
							},
						},
					},
				},
			},
			actualDcConfig:      &unstructured.Unstructured{"num_tokens": 256},
			desiredDcConfig:     &unstructured.Unstructured{"num_tokens": 16},
			expectedDcNumTokens: 256,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualDcConfig := GetDatacenterConfig()

			actualDcConfig.CassandraConfig.CassandraYaml = *tc.actualDcConfig
			actualDc, err := NewDatacenter(
				types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
				&actualDcConfig,
			)
			assert.NoError(t, err)
			desiredDcConfig := GetDatacenterConfig()
			desiredDcConfig.CassandraConfig.CassandraYaml = *tc.desiredDcConfig
			desiredDc, err := NewDatacenter(
				types.NamespacedName{Name: "testdc", Namespace: "test-namespace"},
				&desiredDcConfig,
			)
			assert.NoError(t, err)

			got, err := SetNewDefaultNumTokens(tc.kc, desiredDc, actualDc)
			assert.NoError(t, err)

			config, err := utils.UnmarshalToMap(got.Spec.Config)
			assert.NoError(t, err)
			cassYaml := config["cassandra-yaml"].(map[string]interface{})
			assert.Equal(t, tc.expectedDcNumTokens, cassYaml["num_tokens"])
		})
	}
}
