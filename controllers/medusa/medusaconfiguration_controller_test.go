package medusa

import (
	"context"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testMedusaConfiguration(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	t.Run("testMedusaConfigurationOk", func(t *testing.T) {
		testMedusaConfigurationOk(t, ctx, f, namespace)
	})
	t.Run("testMedusaConfigurationKo", func(t *testing.T) {
		testMedusaConfigurationKo(t, ctx, f, namespace)
	})
	t.Run("testMedusaConfigurationNoSecret", func(t *testing.T) {
		testMedusaConfigurationNoSecret(t, ctx, f, namespace)
	})
}

func testMedusaConfigurationOk(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")

	bucketKeySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "medusa-bucket-key",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"credentials": []byte("test"),
		},
	}

	t.Log("Creating medusa bucket key secret")
	require.NoError(f.Create(ctx, dc1Key, bucketKeySecret), "failed to create medusa bucket key secret")

	medusaConfig := &api.MedusaConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "medusa-config",
			Namespace: namespace,
		},
		Spec: api.MedusaConfigurationSpec{
			StorageProperties: api.Storage{
				StorageSecretRef: corev1.LocalObjectReference{
					Name: "medusa-bucket-key",
				},
				BucketName:      "test",
				StorageProvider: "s3",
			},
		},
	}
	require.NoError(f.Create(ctx, dc1Key, medusaConfig), "failed to create medusa configuration")
	require.Eventually(func() bool {
		updated := &api.MedusaConfiguration{}
		configKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, "medusa-config")

		if err := f.Get(ctx, configKey, updated); err != nil {
			t.Logf("failed to get medusa configuration: %v", err)
			return false
		}
		updatedSecret := &corev1.Secret{}
		bucketKeySecretKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, "medusa-bucket-key")
		require.NoError(f.Get(ctx, bucketKeySecretKey, updatedSecret))
		//Ensure that the unique label has been added to the secret.
		if updatedSecret.Labels[api.MedusaStorageSecretIdentifierLabel] != utils.HashNameNamespace(updatedSecret.Name, updatedSecret.Namespace) {
			return false
		}
		for _, condition := range updated.Status.Conditions {
			t.Logf("medusa configuration condition: %v", condition)
			if condition.Type == string(api.ControlStatusReady) {
				return condition.Status == metav1.ConditionTrue
			}
		}
		t.Logf("medusa configuration not ready yet")
		return false
	}, timeout, interval)
}

func testMedusaConfigurationKo(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")

	medusaConfig := &api.MedusaConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "medusa-config-ko",
			Namespace: namespace,
		},
		Spec: api.MedusaConfigurationSpec{
			StorageProperties: api.Storage{
				StorageSecretRef: corev1.LocalObjectReference{
					Name: "medusa-bucket-key-ko",
				},
				BucketName:      "test",
				StorageProvider: "s3",
			},
		},
	}
	configKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, "medusa-config-ko")
	require.NoError(f.Create(ctx, dc1Key, medusaConfig), "failed to create medusa configuration")
	require.Eventually(func() bool {
		updated := &api.MedusaConfiguration{}
		if err := f.Get(ctx, configKey, updated); err != nil {
			t.Logf("failed to get medusa configuration: %v", err)
			return false
		}
		for _, condition := range updated.Status.Conditions {
			t.Logf("medusa configuration condition: %v", condition)
			if condition.Type == string(api.ControlStatusSecretAvailable) {
				return condition.Status == metav1.ConditionFalse
			}
		}
		t.Logf("medusa configuration not ready yet")
		return false
	}, timeout, interval)

	require.Never(func() bool {
		updated := &api.MedusaConfiguration{}
		err := f.Get(ctx, dc1Key, updated)
		if err != nil {
			t.Logf("failed to get medusa configuration: %v", err)
			return false
		}
		for _, condition := range updated.Status.Conditions {
			t.Logf("medusa configuration condition: %v", condition)
			if condition.Type == string(api.ControlStatusReady) {
				return condition.Status == metav1.ConditionTrue
			}
		}
		t.Logf("medusa configuration not ready yet")
		return false
	}, neverTimeout, interval)
}

// Testing that the medusa configuration is ready even if no secret is provided.
// The secret can be defined in the K8ssandraCluster object directly without being referenced by the medusa configuration.
func testMedusaConfigurationNoSecret(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)
	dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")

	medusaConfig := &api.MedusaConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "medusa-config-no-secret",
			Namespace: namespace,
		},
		Spec: api.MedusaConfigurationSpec{
			StorageProperties: api.Storage{
				BucketName:      "test",
				StorageProvider: "s3",
			},
		},
	}
	configKey := framework.NewClusterKey(dc1Key.K8sContext, dc1Key.Namespace, "medusa-config-no-secret")
	require.NoError(f.Create(ctx, dc1Key, medusaConfig), "failed to create medusa configuration")
	require.Eventually(func() bool {
		updated := &api.MedusaConfiguration{}
		if err := f.Get(ctx, configKey, updated); err != nil {
			t.Logf("failed to get medusa configuration: %v", err)
			return false
		}
		for _, condition := range updated.Status.Conditions {
			t.Logf("medusa configuration condition: %v", condition)
			if condition.Type == string(api.ControlStatusReady) {
				return condition.Status == metav1.ConditionTrue
			}
		}
		t.Logf("medusa configuration not ready yet")
		return false
	}, timeout, interval)
}
