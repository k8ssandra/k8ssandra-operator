package medusa

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRefreshSecrets_defaultSUSecret(t *testing.T) {
	fakeClient := test.NewFakeClientWRestMapper()
	cassDC := test.NewCassandraDatacenter("dc1", "test")
	cassDC.Spec.Users = []cassdcapi.CassandraUser{
		{SecretName: "custom-user"},
	}
	assert.NoError(t, fakeClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}))
	assert.NoError(t, fakeClient.Create(context.Background(), &cassDC))
	secrets := []corev1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "custom-user", Namespace: "test"}, Data: map[string][]byte{"username": []byte("test")}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-superuser", Namespace: "test"}, Data: map[string][]byte{"username": []byte("test")}},
	}
	for _, i := range secrets {
		assert.NoError(t, fakeClient.Create(context.Background(), &i))
	}
	assert.NoError(t, RefreshSecrets(&cassDC, context.Background(), fakeClient, logr.Logger{}, 0))
	suSecret := &corev1.Secret{}
	assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-cluster-superuser", Namespace: "test"}, suSecret))
	_, exists := suSecret.ObjectMeta.Annotations["k8ssandra.io/refresh"]
	assert.True(t, exists)
	userSecret := &corev1.Secret{}
	assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "custom-user", Namespace: "test"}, userSecret))
	_, exists = userSecret.ObjectMeta.Annotations["k8ssandra.io/refresh"]
	assert.True(t, exists)
}

func TestRefreshSecrets_customSecrets(t *testing.T) {
	fakeClient := test.NewFakeClientWRestMapper()
	cassDC := test.NewCassandraDatacenter("dc1", "test")
	cassDC.Spec.Users = []cassdcapi.CassandraUser{
		{SecretName: "custom-user"},
	}
	cassDC.Spec.SuperuserSecretName = "cass-custom-superuser"
	assert.NoError(t, fakeClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}))
	assert.NoError(t, fakeClient.Create(context.Background(), &cassDC))
	secrets := []corev1.Secret{
		{ObjectMeta: metav1.ObjectMeta{Name: "custom-user", Namespace: "test"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "cass-custom-superuser", Namespace: "test"}},
	}
	for _, i := range secrets {
		assert.NoError(t, fakeClient.Create(context.Background(), &i))
	}
	assert.NoError(t, RefreshSecrets(&cassDC, context.Background(), fakeClient, logr.Logger{}, 0))
	suSecret := &corev1.Secret{}
	assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "cass-custom-superuser", Namespace: "test"}, suSecret))
	_, exists := suSecret.ObjectMeta.Annotations["k8ssandra.io/refresh"]
	assert.True(t, exists)
	userSecret := &corev1.Secret{}
	assert.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: "custom-user", Namespace: "test"}, userSecret))
	_, exists = userSecret.ObjectMeta.Annotations["k8ssandra.io/refresh"]
	assert.True(t, exists)

}
