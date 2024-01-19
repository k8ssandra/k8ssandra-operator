package medusa

import (
	"context"
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func testMedusaConfiguration(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	t.Run("testMedusaConfigurationOk", func(t *testing.T) {
		testMedusaConfigurationOk(t, ctx, f, namespace)
	})
	t.Run("testMedusaConfigurationKo", func(t *testing.T) {
		testMedusaConfigurationKo(t, ctx, f, namespace)
	})
}

func testMedusaConfigurationOk(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

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
	err := f.Client.Create(ctx, bucketKeySecret)
	require.NoError(err, "failed to create medusa bucket key secret")

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
	err = f.Client.Create(ctx, medusaConfig)
	require.NoError(err, "failed to create medusa configuration")
	require.Eventually(func() bool {
		updated := &api.MedusaConfiguration{}
		err := f.Client.Get(ctx, types.NamespacedName{Name: "medusa-config", Namespace: namespace}, updated)
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
	}, timeout, interval)
}

func testMedusaConfigurationKo(t *testing.T, ctx context.Context, f *framework.Framework, namespace string) {
	require := require.New(t)

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
	err := f.Client.Create(ctx, medusaConfig)
	require.NoError(err, "failed to create medusa configuration")
	require.Eventually(func() bool {
		updated := &api.MedusaConfiguration{}
		err := f.Client.Get(ctx, types.NamespacedName{Name: "medusa-config-ko", Namespace: namespace}, updated)
		if err != nil {
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
		err := f.Client.Get(ctx, types.NamespacedName{Name: "medusa-config-ko", Namespace: namespace}, updated)
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
	}, timeout, interval)
}
