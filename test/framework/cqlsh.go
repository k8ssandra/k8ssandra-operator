package framework

import (
	"context"

	"github.com/k8ssandra/k8ssandra-operator/pkg/secret"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (f *E2eFramework) RetrieveSuperuserSecret(ctx context.Context, k8sContext, namespace, clusterName string) (*corev1.Secret, error) {
	var superUserSecret *corev1.Secret
	superUserSecret = &corev1.Secret{}
	superUserSecretKey := ClusterKey{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      secret.DefaultSuperuserSecretName(clusterName),
		},
		K8sContext: k8sContext,
	}
	err := f.Get(ctx, superUserSecretKey, superUserSecret)
	return superUserSecret, err
}

func (f *E2eFramework) RetrieveDatabaseCredentials(ctx context.Context, k8sContext, namespace, clusterName string) (string, string, error) {
	superUserSecret, err := f.RetrieveSuperuserSecret(ctx, k8sContext, namespace, clusterName)
	var username, password string
	if err == nil {
		username = string(superUserSecret.Data["username"])
		password = string(superUserSecret.Data["password"])
	}
	return username, password, err
}

func (f *E2eFramework) ExecuteCql(ctx context.Context, k8sContext, namespace, clusterName, pod, query string) (string, error) {
	username, password, err := f.RetrieveDatabaseCredentials(ctx, k8sContext, namespace, clusterName)
	if err != nil {
		return "", err
	}

	options := kubectl.Options{Context: k8sContext, Namespace: namespace}
	return kubectl.Exec(options, pod,
		f.cqlshBin,
		"--username",
		username,
		"--password",
		password,
		"-e",
		query,
	)
}
