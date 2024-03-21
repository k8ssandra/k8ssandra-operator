package k8ssandra

import (
	"context"
	"fmt"
	"strings"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	cassandra "github.com/k8ssandra/k8ssandra-operator/pkg/cassandra"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	medusa "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	medusaImageRepo                     = "test"
	storageSecret                       = "storage-secret"
	cassandraUserSecret                 = "medusa-secret"
	k8ssandraClusterName                = "test"
	medusaConfigName                    = "medusa-config"
	medusaBucketSecretName              = "medusa-bucket-secret"
	prefixFromMedusaConfig              = "prefix-from-medusa-config"
	prefixFromClusterSpec               = "prefix-from-cluster-spec"
	defaultConcurrentTransfers          = 1
	concurrentTransfersFromMedusaConfig = 2
)

func dcTemplate(dcName string, dataPlaneContext string) api.CassandraDatacenterTemplate {
	return api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: dcName,
		},
		K8sContext: dataPlaneContext,
		Size:       3,
		DatacenterOptions: api.DatacenterOptions{
			ServerVersion: "3.11.14",
			StorageConfig: &cassdcapi.StorageConfig{
				CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
					StorageClassName: &defaultStorageClass,
				},
			},
		},
	}
}

func MedusaConfig(name, namespace string) *medusaapi.MedusaConfiguration {
	return &medusaapi.MedusaConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: medusaapi.MedusaConfigurationSpec{
			StorageProperties: medusaapi.Storage{
				Prefix:              prefixFromMedusaConfig,
				ConcurrentTransfers: concurrentTransfersFromMedusaConfig,
			},
		},
	}
}

func medusaTemplateWithoutConfigRef() *medusaapi.MedusaClusterTemplate {
	return medusaTemplate(nil)
}

func medusaTemplateWithConfigRef(configRefName, namespace string) *medusaapi.MedusaClusterTemplate {
	configRef := &corev1.ObjectReference{
		Name:      configRefName,
		Namespace: namespace,
	}
	return medusaTemplate(configRef)
}

func medusaTemplateWithConfigRefWithoutPrefix(configRefName, namespace string) *medusaapi.MedusaClusterTemplate {
	template := medusaTemplateWithConfigRef(configRefName, namespace)
	template.StorageProperties.Prefix = ""
	return template
}

func medusaTemplateWithConfigRefWithPrefix(configRefName, namespace, prefix string) *medusaapi.MedusaClusterTemplate {
	template := medusaTemplateWithConfigRef(configRefName, namespace)
	template.StorageProperties.Prefix = prefix
	return template
}

func medusaTemplate(configObjectReference *corev1.ObjectReference) *medusaapi.MedusaClusterTemplate {
	template := medusaapi.MedusaClusterTemplate{
		ContainerImage: &images.Image{
			Repository: medusaImageRepo,
		},
		StorageProperties: medusaapi.Storage{
			StorageSecretRef: corev1.LocalObjectReference{
				Name: cassandraUserSecret,
			},
		},
		CassandraUserSecretRef: corev1.LocalObjectReference{
			Name: cassandraUserSecret,
		},
		ReadinessProbe: &corev1.Probe{
			InitialDelaySeconds: 1,
			TimeoutSeconds:      2,
			PeriodSeconds:       3,
			SuccessThreshold:    1,
			FailureThreshold:    5,
		},
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: 6,
			TimeoutSeconds:      7,
			PeriodSeconds:       8,
			SuccessThreshold:    1,
			FailureThreshold:    10,
		},
		Resources: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("150m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
		},
	}

	if configObjectReference != nil {
		configObjectReference.DeepCopyInto(&template.MedusaConfigurationRef)
	}

	return &template
}

func createMultiDcClusterWithMedusa(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      k8ssandraClusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					dcTemplate("dc1", f.DataPlaneContexts[0]),
					dcTemplate("dc2", f.DataPlaneContexts[1]),
				},
			},
			Medusa: medusaTemplateWithoutConfigRef(),
		},
	}
	require.NotNil(kc.Spec.Medusa.MedusaConfigurationRef)
	require.Equal("", kc.Spec.Medusa.MedusaConfigurationRef.Name)
	require.Equal("", kc.Spec.Medusa.MedusaConfigurationRef.Namespace)

	t.Log("Creating k8ssandracluster with Medusa")
	err := f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	reconcileMedusaStandaloneDeployment(ctx, t, f, kc, "dc1", f.DataPlaneContexts[0])
	t.Log("check that dc1 was created")
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	t.Log("verify the config map exists and has the contents from the MedusaConfiguration object")
	defaultPrefix := kc.Spec.Medusa.StorageProperties.Prefix
	verifyConfigMap(require, ctx, f, namespace, defaultPrefix, defaultConcurrentTransfers)

	t.Log("check that the standalone Medusa deployment was created in dc1")
	medusaDeploymentKey1 := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: medusa.MedusaStandaloneDeploymentName(k8ssandraClusterName, "dc1")}, K8sContext: f.DataPlaneContexts[0]}
	medusaDeployment1 := &appsv1.Deployment{}
	require.Eventually(func() bool {
		if err := f.Get(ctx, medusaDeploymentKey1, medusaDeployment1); err != nil {
			return false
		}
		return true
	}, timeout, interval)

	require.True(f.ContainerHasEnvVar(medusaDeployment1.Spec.Template.Spec.Containers[0], "MEDUSA_RESOLVE_IP_ADDRESSES", "False"))

	t.Log("check that the standalone Medusa service was created")
	medusaServiceKey1 := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: medusa.MedusaServiceName(k8ssandraClusterName, "dc1")}, K8sContext: f.DataPlaneContexts[0]}
	medusaService1 := &corev1.Service{}
	require.Eventually(func() bool {
		if err := f.Get(ctx, medusaServiceKey1, medusaService1); err != nil {
			return false
		}
		return true
	}, timeout, interval)

	t.Log("update datacenter status to scaling up")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterScalingUp,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to patch datacenter status")

	kcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: k8ssandraClusterName}}

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
		err = f.Get(ctx, kcKey, kc)

		if err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}

		if len(kc.Status.Datacenters) == 0 {
			return false
		}

		k8ssandraStatus, found := kc.Status.Datacenters[dc1Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc1Key)
			return false
		}

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterScalingUp)
		return !(condition == nil && condition.Status == corev1.ConditionFalse)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	dc1 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc1Key, dc1)
	checkMedusaObjectsCompliance(t, f, dc1, kc)

	t.Log("check that dc2 has not been created yet")
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[1]}
	dc2 := &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.True(err != nil && errors.IsNotFound(err), "dc2 should not be created until dc1 is ready")

	t.Log("update dc1 status to ready")
	err = f.PatchDatacenterStatus(ctx, dc1Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to update dc1 status to ready")

	require.Eventually(func() bool {
		return f.UpdateDatacenterGeneration(ctx, t, dc1Key)
	}, timeout, interval, "failed to update dc1 generation")

	reconcileMedusaStandaloneDeployment(ctx, t, f, kc, "dc2", f.DataPlaneContexts[1])
	t.Log("check that dc2 was created")
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	t.Log("check that the standalone Medusa deployment was created in dc2")
	medusaDeploymentKey2 := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: medusa.MedusaStandaloneDeploymentName(k8ssandraClusterName, "dc2")}, K8sContext: f.DataPlaneContexts[1]}
	medusaDeployment2 := &appsv1.Deployment{}
	require.Eventually(func() bool {
		if err := f.Get(ctx, medusaDeploymentKey2, medusaDeployment2); err != nil {
			return false
		}
		return true
	}, timeout, interval)

	t.Log("check that the standalone Medusa service was created in dc2")
	medusaServiceKey2 := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: medusa.MedusaServiceName(k8ssandraClusterName, "dc2")}, K8sContext: f.DataPlaneContexts[1]}
	medusaService2 := &corev1.Service{}
	require.Eventually(func() bool {
		if err := f.Get(ctx, medusaServiceKey2, medusaService2); err != nil {
			return false
		}
		return true
	}, timeout, interval)

	t.Log("check that remote seeds are set on dc2")
	dc2 = &cassdcapi.CassandraDatacenter{}
	err = f.Get(ctx, dc2Key, dc2)
	require.NoError(err, "failed to get dc2")

	t.Log("update dc2 status to ready")
	err = f.PatchDatacenterStatus(ctx, dc2Key, func(dc *cassdcapi.CassandraDatacenter) {
		dc.Status.CassandraOperatorProgress = cassdcapi.ProgressReady
		dc.SetCondition(cassdcapi.DatacenterCondition{
			Type:               cassdcapi.DatacenterReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
		})
	})
	require.NoError(err, "failed to update dc2 status to ready")

	require.Eventually(func() bool {
		return f.UpdateDatacenterGeneration(ctx, t, dc2Key)
	}, timeout, interval, "failed to update dc2 generation")

	t.Log("check that dc2 was rebuilt")
	verifyRebuildTaskCreated(ctx, t, f, dc2Key, dc1Key)
	rebuildTaskKey := framework.NewClusterKey(f.DataPlaneContexts[1], kc.Namespace, "dc2-rebuild")
	setRebuildTaskFinished(ctx, t, f, rebuildTaskKey, dc2Key)

	checkMedusaObjectsCompliance(t, f, dc2, kc)

	t.Log("check that the K8ssandraCluster status is updated")
	require.Eventually(func() bool {
		kc := &api.K8ssandraCluster{}
		err = f.Get(ctx, kcKey, kc)
		if err != nil {
			t.Logf("failed to get K8ssandraCluster: %v", err)
			return false
		}

		if len(kc.Status.Datacenters) != 2 {
			return false
		}

		k8ssandraStatus, found := kc.Status.Datacenters[dc1Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc1Key)
			return false
		}

		condition := FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			t.Logf("k8ssandracluster status check failed: cassandra in %s is not ready", dc1Key.Name)
			return false
		}

		k8ssandraStatus, found = kc.Status.Datacenters[dc2Key.Name]
		if !found {
			t.Logf("status for datacenter %s not found", dc2Key)
			return false
		}

		condition = FindDatacenterCondition(k8ssandraStatus.Cassandra, cassdcapi.DatacenterReady)
		if condition == nil || condition.Status == corev1.ConditionFalse {
			t.Logf("k8ssandracluster status check failed: cassandra in %s is not ready", dc2Key.Name)
			return false
		}

		return true
	}, timeout, interval, "timed out waiting for K8ssandraCluster status update")

	// Test cluster deletion
	t.Log("deleting K8ssandraCluster")
	err = f.DeleteK8ssandraCluster(ctx, client.ObjectKey{Namespace: namespace, Name: kc.Name}, timeout, interval)
	require.NoError(err, "failed to delete K8ssandraCluster")
	f.AssertObjectDoesNotExist(ctx, t, dc1Key, &cassdcapi.CassandraDatacenter{}, timeout, interval)
	// Check that Medusa Standalone deployment and service were deleted
	f.AssertObjectDoesNotExist(ctx, t, medusaDeploymentKey1, &appsv1.Deployment{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, medusaDeploymentKey2, &appsv1.Deployment{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, medusaServiceKey1, &corev1.Service{}, timeout, interval)
	f.AssertObjectDoesNotExist(ctx, t, medusaServiceKey2, &corev1.Service{}, timeout, interval)
}

// Check that all the Medusa related objects have been created and are in the expected state.
func checkMedusaObjectsCompliance(t *testing.T, f *framework.Framework, dc *cassdcapi.CassandraDatacenter, kc *api.K8ssandraCluster) {
	require := require.New(t)

	// Check containers presence
	initContainerIndex, found := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "medusa-restore")
	require.True(found, fmt.Sprintf("%s doesn't have medusa-restore init container", dc.Name))
	_, foundConfig := cassandra.FindInitContainer(dc.Spec.PodTemplateSpec, "server-config-init")
	require.True(foundConfig, fmt.Sprintf("%s doesn't have server-config-init container", dc.Name))
	initContainer := dc.Spec.PodTemplateSpec.Spec.InitContainers[initContainerIndex]
	containerIndex, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "medusa")
	require.True(found, fmt.Sprintf("%s doesn't have medusa container", dc.Name))
	mainContainer := dc.Spec.PodTemplateSpec.Spec.Containers[containerIndex]

	for _, container := range [](corev1.Container){initContainer, mainContainer} {
		// Check containers Image
		require.True(container.Image == fmt.Sprintf("docker.io/%s/medusa:latest", medusaImageRepo), fmt.Sprintf("%s %s init container doesn't have the right image %s vs docker.io/%s/medusa:latest", dc.Name, container.Name, container.Image, medusaImageRepo))

		// Check volume mounts
		assert.True(t, f.ContainerHasVolumeMount(container, "server-config", "/etc/cassandra"), "Missing Volume Mount for medusa-restore server-config")
		assert.True(t, f.ContainerHasVolumeMount(container, "server-data", "/var/lib/cassandra"), "Missing Volume Mount for medusa-restore server-data")
		assert.True(t, f.ContainerHasVolumeMount(container, "podinfo", "/etc/podinfo"), "Missing Volume Mount for medusa-restore podinfo")
		assert.True(t, f.ContainerHasVolumeMount(container, cassandraUserSecret, "/etc/medusa-secrets"), "Missing Volume Mount for medusa-restore medusa-secrets")
		assert.True(t, f.ContainerHasVolumeMount(container, fmt.Sprintf("%s-medusa", kc.Name), "/etc/medusa"), "Missing Volume Mount for medusa-restore medusa config")

		// Check env vars
		if container.Name == "medusa" {
			assert.True(t, f.ContainerHasEnvVar(container, "MEDUSA_MODE", "GRPC"), "Wrong MEDUSA_MODE env var for medusa")
		} else {
			assert.True(t, f.ContainerHasEnvVar(container, "MEDUSA_MODE", "RESTORE"), "Wrong MEDUSA_MODE env var for medusa-restore")
		}
		assert.True(t, f.ContainerHasEnvVar(container, "MEDUSA_TMP_DIR", ""), "Missing MEDUSA_TMP_DIR env var for medusa-restore")
	}
}

func reconcileMedusaStandaloneDeployment(ctx context.Context, t *testing.T, f *framework.Framework, kc *api.K8ssandraCluster, dcName string, k8sContext string) {
	t.Log("create Medusa Standalone deployment")

	medusaDepl := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      medusa.MedusaStandaloneDeploymentName(kc.SanitizedName(), dcName),
			Namespace: kc.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": medusa.MedusaStandaloneDeploymentName(kc.SanitizedName(), dcName)},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": medusa.MedusaStandaloneDeploymentName(kc.SanitizedName(), dcName)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  medusa.MedusaStandaloneDeploymentName(kc.SanitizedName(), dcName),
							Image: "quay.io/k8ssandra/medusa:0.11.0",
						},
					},
				},
			},
		},
	}
	medusaKey := framework.ClusterKey{NamespacedName: utils.GetKey(medusaDepl), K8sContext: k8sContext}
	require.NoError(t, f.Create(ctx, medusaKey, medusaDepl))

	actualMedusaDepl := &appsv1.Deployment{}
	assert.Eventually(t, func() bool {
		err := f.Get(ctx, medusaKey, actualMedusaDepl)
		return err == nil
	}, timeout, interval, "failed to get Medusa Deployment")

	err := f.SetMedusaDeplAvailable(ctx, medusaKey)

	require.NoError(t, err, "Failed to update Medusa Deployment status")
}

func createSingleDcClusterWithMedusaConfigRef(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	t.Log("Creating Medusa Bucket secret")
	medusaSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      medusaBucketSecretName,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"credentials": "some-credentials",
		},
	}
	// create the secret in the control plane
	err := f.Create(ctx, controlPlaneContextKey(f, medusaSecret, f.ControlPlaneContext), medusaSecret)
	require.NoError(err, fmt.Sprintf("failed to create secret in control plane %s", f.ControlPlaneContext))

	t.Log("Creating Medusa Configuration object")
	medusaConfig := MedusaConfig(medusaConfigName, namespace)
	medusaConfig.Spec.StorageProperties.StorageSecretRef = corev1.LocalObjectReference{Name: medusaBucketSecretName}
	medusaConfigKey := controlPlaneContextKey(f, medusaConfig, f.ControlPlaneContext)
	err = f.Create(ctx, medusaConfigKey, medusaConfig)

	require.NoError(err, "failed to create Medusa Configuration")
	require.Eventually(f.MedusaConfigExists(ctx, f.ControlPlaneContext, medusaConfigKey), timeout, interval)

	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      k8ssandraClusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					dcTemplate("dc1", f.DataPlaneContexts[0]),
				},
			},
			Medusa: medusaTemplateWithConfigRefWithPrefix(medusaConfigName, namespace, prefixFromClusterSpec),
		},
	}
	require.NotNil(kc.Spec.Medusa.MedusaConfigurationRef)
	require.Equal(medusaConfigName, kc.Spec.Medusa.MedusaConfigurationRef.Name)

	t.Log("Creating k8ssandracluster with Medusa and a config ref")
	err = f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	t.Log("verify the config map exists and has the contents from the MedusaConfiguration object")
	verifyConfigMap(require, ctx, f, namespace, prefixFromClusterSpec, concurrentTransfersFromMedusaConfig)
}

func verifyConfigMap(r *require.Assertions, ctx context.Context, f *framework.Framework, namespace string, expectedPrefix string, expectedConcurrentTransfers int) {
	configMapName := fmt.Sprintf("%s-medusa", k8ssandraClusterName)
	configMapKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: configMapName}, K8sContext: f.DataPlaneContexts[0]}
	configMap := &corev1.ConfigMap{}
	r.Eventually(func() bool {
		if err := f.Get(ctx, configMapKey, configMap); err != nil {
			r.NoError(err, "failed to get Medusa ConfigMap")
			return false
		}
		prefixCorrect := strings.Contains(configMap.Data["medusa.ini"], fmt.Sprintf("prefix = %s", expectedPrefix))
		concurrentTransfersCorrect := strings.Contains(configMap.Data["medusa.ini"], fmt.Sprintf("concurrent_transfers = %d", expectedConcurrentTransfers))
		return prefixCorrect && concurrentTransfersCorrect
	}, timeout, interval, "Medusa ConfigMap doesn't have the right content")
}

func creatingSingleDcClusterWithoutPrefixInClusterSpecFails(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	// make a cluster spec without the prefix
	kcFirstAttempt := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      k8ssandraClusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					dcTemplate("dc1", f.DataPlaneContexts[0]),
				},
			},
			Medusa: medusaTemplateWithConfigRefWithoutPrefix(medusaConfigName, namespace),
		},
	}
	require.NotNil(kcFirstAttempt.Spec.Medusa.MedusaConfigurationRef)
	require.Equal(medusaConfigName, kcFirstAttempt.Spec.Medusa.MedusaConfigurationRef.Name)
	require.Equal("", kcFirstAttempt.Spec.Medusa.StorageProperties.Prefix)
	kcSecondAttempt := kcFirstAttempt.DeepCopy()

	// submit the cluster for creation
	t.Log("Creating k8ssandracluster with Medusa but without MedusaConfig and without a prefix in the cluster spec")
	err := f.Client.Create(ctx, kcFirstAttempt)
	require.Error(err, "creating a cluster without Medusa's storage prefix should not happen")

	// verify the cluster doesn't get created
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Never(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	// create the MedusaConfiguration object
	t.Log("Creating Medusa Configuration object")
	medusaConfigKey := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: medusaConfigName}, K8sContext: f.DataPlaneContexts[0]}
	err = f.Create(ctx, medusaConfigKey, MedusaConfig(medusaConfigName, namespace))
	require.NoError(err, "failed to create Medusa Configuration")
	require.Eventually(f.MedusaConfigExists(ctx, f.DataPlaneContexts[0], medusaConfigKey), timeout, interval)

	// add a reference to the MedusaConfiguration object to the cluster spec
	kcSecondAttempt.Spec.Medusa.MedusaConfigurationRef.Name = medusaConfigName

	// confirm the prefix in the storage properties is still empty
	require.Equal("", kcSecondAttempt.Spec.Medusa.StorageProperties.Prefix)

	// retry creating the cluster
	t.Log("Creating k8ssandracluster with Medusa and MedusaConfig but without a prefix in the cluster spec")
	err = f.Client.Create(ctx, kcSecondAttempt)
	require.Error(err, "creating a cluster without Medusa's storage prefix should not happen if the MedusaConfig object exists")

	// verify the cluster still doesn't get created
	require.Never(f.DatacenterExists(ctx, dc1Key), timeout, interval)
}

func controlPlaneContextKey(f *framework.Framework, object metav1.Object, contextName string) framework.ClusterKey {
	return framework.ClusterKey{NamespacedName: utils.GetKey(object), K8sContext: contextName}
}

func dataPlaneContextKey(f *framework.Framework, object metav1.Object, dataPlaneContextIndex int) framework.ClusterKey {
	return framework.ClusterKey{NamespacedName: utils.GetKey(object), K8sContext: f.DataPlaneContexts[dataPlaneContextIndex]}
}

func createMultiDcClusterWithReplicatedSecrets(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

	clusterName := "test-cluster"
	originalConfigName := "test-config"
	originalSecretName := fmt.Sprintf("%s-bucket-key", originalConfigName)
	clusterSecretName := fmt.Sprintf("%s-%s", clusterName, originalSecretName)

	// create a storage secret, then a MedusaConfiguration that points to it
	// the ReplicatedSecrets controller is not loaded in env tests, so we "mock" it by replicating the secrets manually
	medusaSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      originalSecretName,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"credentials": "some-credentials",
		},
	}
	// create the secret in the control plane
	cpMedusaSecret := medusaSecret.DeepCopy()
	err := f.Create(ctx, controlPlaneContextKey(f, cpMedusaSecret, f.ControlPlaneContext), cpMedusaSecret)
	require.NoError(err, fmt.Sprintf("failed to create secret in control plane %s", f.ControlPlaneContext))
	//create the secret in the data planes
	for i, n := range f.DataPlaneContexts {
		dpMedusaSecret := medusaSecret.DeepCopy()
		dpMedusaSecret.Name = clusterSecretName
		err := f.Create(ctx, dataPlaneContextKey(f, dpMedusaSecret, i), dpMedusaSecret)
		require.NoError(err, fmt.Sprintf("failed to create secret in context %d (%s)", i, n))
	}

	// create medusa config in the control plane only
	medusaConfig := MedusaConfig(originalConfigName, namespace)
	medusaConfig.Spec.StorageProperties.StorageSecretRef = corev1.LocalObjectReference{
		Name: originalSecretName,
	}
	cpMedusaConfig := medusaConfig.DeepCopy()
	err = f.Create(ctx, controlPlaneContextKey(f, cpMedusaConfig, f.ControlPlaneContext), cpMedusaConfig)
	require.NoError(err, fmt.Sprintf("failed to create MedusaConfiguration in control plane %s", f.ControlPlaneContext))

	// create a 2-dc K8ssandraCluster with Medusa featuring the reference to the above MedusaConfiguration
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				Datacenters: []api.CassandraDatacenterTemplate{
					dcTemplate("dc1", f.DataPlaneContexts[1]),
					dcTemplate("dc2", f.DataPlaneContexts[2]),
				},
			},
			Medusa: &medusaapi.MedusaClusterTemplate{
				MedusaConfigurationRef: corev1.ObjectReference{
					Namespace: namespace,
					Name:      originalConfigName,
				},
				StorageProperties: medusaapi.Storage{
					Prefix: "some-prefix",
				},
			},
		},
	}
	err = f.Client.Create(ctx, kc)
	require.NoError(err, "failed to create K8ssandraCluster")

	verifySuperuserSecretCreated(ctx, t, f, kc)
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	reconcileMedusaStandaloneDeployment(ctx, t, f, kc, "dc1", f.DataPlaneContexts[1])

	// crate the first DC
	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[1]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	// mark the first DC as ready
	t.Log("update dc1 status to ready")
	err = f.SetDatacenterStatusReady(ctx, dc1Key)
	require.NoError(err, "failed to update dc1 status to ready")

	// create the second DC
	reconcileMedusaStandaloneDeployment(ctx, t, f, kc, "dc2", f.DataPlaneContexts[2])
	dc2Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}, K8sContext: f.DataPlaneContexts[2]}
	require.Eventually(f.DatacenterExists(ctx, dc2Key), timeout, interval)

	// verify the copied secret is mounted in the pods
	verifyBucketSecretMounted(ctx, t, f, dc1Key, clusterSecretName)
	verifyBucketSecretMounted(ctx, t, f, dc2Key, clusterSecretName)

	// verify the cluster's spec still contains the correct value
	// which is empty because we used MedusaConfigRef
	// merged it at runtime but never persisted to the k8ssandraCluster object
	kc = &api.K8ssandraCluster{}
	err = f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: clusterName}, kc)
	require.NoError(err, "failed to get K8ssandraCluster")
	require.Equal("", kc.Spec.Medusa.StorageProperties.StorageSecretRef.Name)
}

func verifyBucketSecretMounted(ctx context.Context, t *testing.T, f *framework.Framework, dcKey framework.ClusterKey, clusterSecretName string) {
	require := require.New(t)

	// fetch the DC spec
	dc := &cassdcapi.CassandraDatacenter{}
	err := f.Get(ctx, dcKey, dc)
	require.NoError(err, fmt.Sprintf("failed to get %s", dcKey.Name))

	// fetch medusa container
	containerIndex, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "medusa")
	require.True(found, fmt.Sprintf("%s doesn't have medusa container", dc.Name))
	medusaContainer := dc.Spec.PodTemplateSpec.Spec.Containers[containerIndex]

	// check its mount
	assert.True(t, f.ContainerHasVolumeMount(medusaContainer, clusterSecretName, "/etc/medusa-secrets"), "Missing Volume Mount for Medusa bucket key")
}

func createSingleDcClusterWithManagementApiSecured(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	kc := &api.K8ssandraCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      k8ssandraClusterName,
		},
		Spec: api.K8ssandraClusterSpec{
			Cassandra: &api.CassandraClusterTemplate{
				DatacenterOptions: api.DatacenterOptions{
					ManagementApiAuth: &cassdcapi.ManagementApiAuthConfig{
						Manual: &cassdcapi.ManagementApiAuthManualConfig{
							ClientSecretName: "test-client-secret",
						},
					},
				},
				Datacenters: []api.CassandraDatacenterTemplate{
					dcTemplate("dc1", f.DataPlaneContexts[0]),
				},
			},
			Medusa: medusaTemplateWithoutConfigRef(),
		},
	}

	require.NoError(f.Client.Create(ctx, kc))
	verifyReplicatedSecretReconciled(ctx, t, f, kc)

	reconcileMedusaStandaloneDeployment(ctx, t, f, kc, "dc1", f.DataPlaneContexts[0])

	dc1Key := framework.ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}, K8sContext: f.DataPlaneContexts[0]}
	require.Eventually(f.DatacenterExists(ctx, dc1Key), timeout, interval)

	dc := &cassdcapi.CassandraDatacenter{}
	require.NoError(f.Get(ctx, dc1Key, dc))

	checkMedusaObjectsCompliance(t, f, dc, kc)

	containerIndex, found := cassandra.FindContainer(dc.Spec.PodTemplateSpec, "medusa")
	require.True(found, fmt.Sprintf("%s doesn't have medusa container", dc.Name))
	mainContainer := dc.Spec.PodTemplateSpec.Spec.Containers[containerIndex]

	require.True(f.ContainerHasVolumeMount(mainContainer, "mgmt-encryption", "/etc/encryption/mgmt"))

	volumeIndex, foundMgmtEncryptionClient := cassandra.FindVolume(dc.Spec.PodTemplateSpec, "mgmt-encryption")
	require.True(foundMgmtEncryptionClient)
	vol := dc.Spec.PodTemplateSpec.Spec.Volumes[volumeIndex]
	require.Equal(kc.Spec.Cassandra.DatacenterOptions.ManagementApiAuth.Manual.ClientSecretName, vol.Secret.SecretName)
}
