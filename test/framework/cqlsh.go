package framework

import (
	"context"
	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
	"time"
)

func (f *E2eFramework) RetrieveSuperuserSecret(t *testing.T, ctx context.Context, namespace, k8cName string) *corev1.Secret {
	var superUserSecret *corev1.Secret
	timeout := 2 * time.Minute
	interval := 1 * time.Second
	require.Eventually(t, func() bool {
		superUserSecret = &corev1.Secret{}
		superUserSecretKey := ClusterKey{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      secret.DefaultSuperuserSecretName(k8cName),
			},
			K8sContext: f.ControlPlaneContext,
		}
		err := f.Get(ctx, superUserSecretKey, superUserSecret)
		return err == nil &&
			len(superUserSecret.Data["username"]) >= 0 &&
			len(superUserSecret.Data["password"]) >= 0
	}, timeout, interval, "Failed to retrieve superuser secret")
	return superUserSecret
}

func (f *E2eFramework) RetrieveDatabaseCredentials(t *testing.T, ctx context.Context, namespace, k8cName string) (string, string) {
	superUserSecret := f.RetrieveSuperuserSecret(t, ctx, namespace, k8cName)
	username := string(superUserSecret.Data["username"])
	password := string(superUserSecret.Data["password"])
	return username, password
}

func (f *E2eFramework) ExecuteCql(t *testing.T, ctx context.Context, k8sContext, namespace, k8cName, pod, query string) string {
	username, password := f.RetrieveDatabaseCredentials(t, ctx, namespace, k8cName)
	options := kubectl.Options{Namespace: namespace, Context: k8sContext}
	output, _ := kubectl.Exec(options, pod,
		"--",
		"/opt/cassandra/bin/cqlsh",
		"--username",
		username,
		"--password",
		password,
		"-e",
		query,
	)
	return output
}

func (f *E2eFramework) CheckKeyspaceExists(t *testing.T, ctx context.Context, k8sContext, namespace, k8cName, pod, keyspace string) {
	keyspaces := f.ExecuteCql(t, ctx, k8sContext, namespace, k8cName, pod, "describe keyspaces")
	assert.Contains(t, keyspaces, keyspace)
}
