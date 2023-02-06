package k8ssandra

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/nodeconfig"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestK8ssandraClusterReconciler_reconcilePerNodeConfiguration(t *testing.T) {
	type args struct {
		dcConfig     *cassandra.DatacenterConfig
		remoteClient client.Client
	}
	tests := []struct {
		name         string
		args         args
		wantResult   result.ReconcileResult
		configAssert func(*testing.T, *cassandra.DatacenterConfig)
	}{
		{
			name: "no per-node config",
			args: args{
				dcConfig: &cassandra.DatacenterConfig{
					Meta: api.EmbeddedObjectMeta{
						Name: "dc1",
					},
				},
				remoteClient: func() client.Client {
					fakeClient, _ := test.NewFakeClient()
					return fakeClient
				}(),
			},
			wantResult: result.Continue(),
			configAssert: func(t *testing.T, gotConfig *cassandra.DatacenterConfig) {
				assert.Nil(t, gotConfig.PodTemplateSpec.Spec.InitContainers)
				assert.Nil(t, gotConfig.PodTemplateSpec.Spec.Volumes)
			},
		},
		{
			name: "default per-node config",
			args: args{
				dcConfig: &cassandra.DatacenterConfig{
					Meta: api.EmbeddedObjectMeta{
						Name: "dc1",
					},
					InitialTokensByPodName: map[string][]string{
						"pod1": {"token1", "token2"},
						"pod2": {"token3", "token4"},
					},
				},
				remoteClient: func() client.Client {
					fakeClient, _ := test.NewFakeClient()
					return fakeClient
				}(),
			},
			wantResult: result.Continue(),
			configAssert: func(t *testing.T, gotConfig *cassandra.DatacenterConfig) {
				assert.Equal(t, "test-dc1-per-node-config", gotConfig.PerNodeConfigMapRef.Name)
				hash := annotations.GetAnnotation(&gotConfig.PodTemplateSpec, api.PerNodeConfigHashAnnotation)
				assert.NotEmpty(t, hash)
				_, found := cassandra.FindInitContainer(&gotConfig.PodTemplateSpec, nodeconfig.PerNodeConfigInitContainerName)
				assert.True(t, found)
				_, found = cassandra.FindVolume(&gotConfig.PodTemplateSpec, nodeconfig.PerNodeConfigVolumeName)
				assert.True(t, found)
			},
		},
		{
			name: "user-provided per-node config",
			args: args{
				dcConfig: &cassandra.DatacenterConfig{
					Meta: api.EmbeddedObjectMeta{
						Name:      "dc1",
						Namespace: "dc1-ns",
					},
					PerNodeConfigMapRef: corev1.LocalObjectReference{
						Name: "user-provided-per-node-config",
					},
				},
				remoteClient: func() client.Client {
					var perNodeConfig = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "user-provided-per-node-config",
							Namespace: "dc1-ns",
						},
						Data: map[string]string{
							"test-dc1-default-sts-0_cassandra.yaml": "irrelevant",
							"test-dc1-default-sts-1_cassandra.yaml": "irrelevant",
							"test-dc1-default-sts-2_cassandra.yaml": "irrelevant",
						},
					}
					fakeClient, _ := test.NewFakeClient(perNodeConfig)
					return fakeClient
				}(),
			},
			wantResult: result.Continue(),
			configAssert: func(t *testing.T, gotConfig *cassandra.DatacenterConfig) {
				assert.Equal(t, "user-provided-per-node-config", gotConfig.PerNodeConfigMapRef.Name)
				hash := annotations.GetAnnotation(&gotConfig.PodTemplateSpec, api.PerNodeConfigHashAnnotation)
				assert.NotEmpty(t, hash)
				_, found := cassandra.FindInitContainer(&gotConfig.PodTemplateSpec, nodeconfig.PerNodeConfigInitContainerName)
				assert.True(t, found)
				_, found = cassandra.FindVolume(&gotConfig.PodTemplateSpec, nodeconfig.PerNodeConfigVolumeName)
				assert.True(t, found)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			framework.Init(t)
			r := &K8ssandraClusterReconciler{
				Scheme: scheme.Scheme,
			}
			testLogger := testr.New(t)
			kc := &api.K8ssandraCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-ns",
				},
			}
			gotResult := r.reconcilePerNodeConfiguration(context.Background(), kc, tt.args.dcConfig, tt.args.remoteClient, testLogger)
			assert.Equal(t, tt.wantResult, gotResult)
			tt.configAssert(t, tt.args.dcConfig)
		})
	}
}

func perNodeConfiguration(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	t.Run("Default", perNodeConfigTest(ctx, f, defaultPerNodeConfiguration, namespace))
	t.Run("UserDefined", perNodeConfigTest(ctx, f, userDefinedPerNodeConfiguration, namespace))
}

type perNodeConfigTestFunc func(t *testing.T, ctx context.Context, f *framework.Framework, namespace string)

func perNodeConfigTest(ctx context.Context, f *framework.Framework, test perNodeConfigTestFunc, namespace string) func(*testing.T) {
	return func(t *testing.T) {
		managementApiFactory.SetT(t)
		managementApiFactory.UseDefaultAdapter()
		test(t, ctx, f, namespace)
	}
}

func defaultPerNodeConfiguration(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test1",
			Namespace: namespace,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						K8sContext: f.DataPlaneContexts[0],
						Meta:       api.EmbeddedObjectMeta{Name: "dc1"},
						Size:       5,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "4.0.4",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{StorageClassName: &defaultStorageClass},
							},
							CassandraConfig: &api.CassandraConfig{
								CassandraYaml: unstructured.Unstructured{
									"num_tokens": 4,
								},
							},
						},
					},
					{
						K8sContext: f.DataPlaneContexts[1],
						Meta:       api.EmbeddedObjectMeta{Name: "dc2"},
						Size:       10,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "4.0.4",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{StorageClassName: &defaultStorageClass},
							},
							Racks: []cassdcapi.Rack{
								{Name: "rack1"},
								{Name: "rack2"},
								{Name: "rack3"},
							},
							CassandraConfig: &api.CassandraConfig{
								CassandraYaml: unstructured.Unstructured{
									"num_tokens": 4,
									"allocate_tokens_for_local_replication_factor": 5,
								},
							},
						},
					},
				},
			},
		},
	}

	err := f.Client.Create(ctx, kc)
	require.NoError(t, err, "failed to create K8ssandraCluster")

	defer func() {
		if err := f.DeleteK8ssandraCluster(ctx, utils.GetKey(kc), timeout, interval); err != nil {
			t.Fatalf("failed to delete k8ssandracluster: %v", err)
		}
	}()

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	verifySuperuserSecretCreated(ctx, t, f, kc)
	verifyReplicatedSecretReconciled(ctx, t, f, kc)
	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the per-node configs were created")

	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, "dc1")
	dc2Key := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, "dc2")

	perNodeConfig1 := &corev1.ConfigMap{}
	assert.Eventually(t, func() bool {
		perNodeConfigKey := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, "test1-dc1-per-node-config")
		err := f.Get(ctx, perNodeConfigKey, perNodeConfig1)
		return assert.NoError(t, err, "failed to create ConfigMap") &&
			assert.Contains(t, perNodeConfig1.Data, "test1-dc1-default-sts-0_cassandra.yaml") &&
			assert.Contains(t, perNodeConfig1.Data, "test1-dc1-default-sts-1_cassandra.yaml") &&
			assert.Contains(t, perNodeConfig1.Data, "test1-dc1-default-sts-2_cassandra.yaml") &&
			assert.NotContains(t, perNodeConfig1.Data, "test1-dc1-default-sts-3_cassandra.yaml") &&
			assert.NotContains(t, perNodeConfig1.Data, "test1-dc1-default-sts-4_cassandra.yaml")
	}, timeout, interval)

	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(t, err, "failed to set CassandraDatacenter status to ready")

	perNodeConfig2 := &corev1.ConfigMap{}
	assert.Eventually(t, func() bool {
		perNodeConfigKey := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, "test1-dc2-per-node-config")
		err := f.Get(ctx, perNodeConfigKey, perNodeConfig2)
		return assert.NoError(t, err, "failed to create ConfigMap") &&
			assert.Contains(t, perNodeConfig2.Data, "test1-dc2-rack1-sts-0_cassandra.yaml") &&
			assert.Contains(t, perNodeConfig2.Data, "test1-dc2-rack2-sts-0_cassandra.yaml") &&
			assert.Contains(t, perNodeConfig2.Data, "test1-dc2-rack3-sts-0_cassandra.yaml") &&
			assert.Contains(t, perNodeConfig2.Data, "test1-dc2-rack1-sts-1_cassandra.yaml") &&
			assert.Contains(t, perNodeConfig2.Data, "test1-dc2-rack2-sts-1_cassandra.yaml") &&
			assert.NotContains(t, perNodeConfig2.Data, "test1-dc2-rack3-sts-1_cassandra.yaml") &&
			assert.NotContains(t, perNodeConfig2.Data, "test1-dc2-rack1-sts-2_cassandra.yaml") &&
			assert.NotContains(t, perNodeConfig2.Data, "test1-dc2-rack2-sts-2_cassandra.yaml") &&
			assert.NotContains(t, perNodeConfig2.Data, "test1-dc2-rack3-sts-2_cassandra.yaml") &&
			assert.NotContains(t, perNodeConfig2.Data, "test1-dc2-rack1-sts-3_cassandra.yaml")
	}, timeout, interval)

	err = f.SetDatacenterStatusReady(ctx, dc2Key)
	require.NoError(t, err, "failed to set CassandraDatacenter status to ready")

	withDc1 := f.NewWithDatacenter(ctx, dc1Key)
	withDc1(func(dc *cassdcapi.CassandraDatacenter) bool {
		hash := annotations.GetAnnotation(dc.Spec.PodTemplateSpec, api.PerNodeConfigHashAnnotation)
		_, foundInitContainer := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, nodeconfig.PerNodeConfigInitContainerName)
		_, foundVolume := cassandra.FindVolume(dc.Spec.PodTemplateSpec, nodeconfig.PerNodeConfigVolumeName)
		return utils.DeepHashString(perNodeConfig1) == hash && foundInitContainer && foundVolume
	})

	withDc2 := f.NewWithDatacenter(ctx, dc2Key)
	withDc2(func(dc *cassdcapi.CassandraDatacenter) bool {
		hash := annotations.GetAnnotation(dc.Spec.PodTemplateSpec, api.PerNodeConfigHashAnnotation)
		_, foundInitContainer := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, nodeconfig.PerNodeConfigInitContainerName)
		_, foundVolume := cassandra.FindVolume(dc.Spec.PodTemplateSpec, nodeconfig.PerNodeConfigVolumeName)
		return utils.DeepHashString(perNodeConfig2) == hash && foundInitContainer && foundVolume
	})
}

func userDefinedPerNodeConfiguration(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {

	perNodeConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-provided-per-node-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"test2-dc1-default-sts-0_cassandra.yaml": "irrelevant",
			"test2-dc1-default-sts-1_cassandra.yaml": "irrelevant",
			"test2-dc1-default-sts-2_cassandra.yaml": "irrelevant",
		},
	}
	perNodeConfigKey := framework.NewClusterKey(f.DataPlaneContexts[0], perNodeConfig.Namespace, perNodeConfig.Name)
	err := f.Create(ctx, perNodeConfigKey, perNodeConfig)
	require.NoError(t, err, "failed to create ConfigMap")

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test2",
			Namespace: namespace,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					{
						K8sContext: f.DataPlaneContexts[0],
						Meta:       api.EmbeddedObjectMeta{Name: "dc1"},
						Size:       3,
						DatacenterOptions: api.DatacenterOptions{
							ServerVersion: "4.0.4",
							StorageConfig: &cassdcapi.StorageConfig{
								CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{StorageClassName: &defaultStorageClass},
							},
						},
						PerNodeConfigMapRef: corev1.LocalObjectReference{
							Name: perNodeConfig.Name,
						},
					},
				},
			},
		},
	}

	err = f.Client.Create(ctx, kc)
	require.NoError(t, err, "failed to create K8ssandraCluster")

	defer func() {
		if err := f.DeleteK8ssandraCluster(ctx, utils.GetKey(kc), timeout, interval); err != nil {
			t.Fatalf("failed to delete k8ssandracluster: %v", err)
		}
	}()

	verifyFinalizerAdded(ctx, t, f, client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name})
	verifySuperuserSecretCreated(ctx, t, f, kc)
	verifyReplicatedSecretReconciled(ctx, t, f, kc)
	verifySystemReplicationAnnotationSet(ctx, t, f, kc)

	t.Log("check that the per-node config was mounted")
	dcKey := framework.NewClusterKey(f.DataPlaneContexts[0], kc.Namespace, "dc1")
	withDc := f.NewWithDatacenter(ctx, dcKey)
	withDc(func(dc *cassdcapi.CassandraDatacenter) bool {
		hash := annotations.GetAnnotation(dc.Spec.PodTemplateSpec, api.PerNodeConfigHashAnnotation)
		_, foundInitContainer := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, nodeconfig.PerNodeConfigInitContainerName)
		_, foundVolume := cassandra.FindVolume(dc.Spec.PodTemplateSpec, nodeconfig.PerNodeConfigVolumeName)
		return utils.DeepHashString(perNodeConfig) == hash && foundInitContainer && foundVolume
	})
}
