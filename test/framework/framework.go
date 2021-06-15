package framework

import (
	"bytes"
	"context"
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"os/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

var (
	Client client.Client
)

func Init(t *testing.T) {
	var err error

	err = api.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for k8ssandra-operator")

	err = cassdcapi.AddToScheme(scheme.Scheme)
	require.NoError(t, err, "failed to register scheme for cass-operator")

	cfg, err := ctrl.GetConfig()
	require.NoError(t, err, "failed to get *rest.Config")

	Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err, "failed to create controller-runtime client")

	DeployK8ssandraOperator(t)
}

func DeployK8ssandraOperator(t *testing.T) {
	err := KustomizeAndApply(t, "../../config/default", "k8ssandra-operator")
	require.NoError(t, err, "failed to run kustomize")

	key := types.NamespacedName{Namespace: "k8ssandra-operator", Name: "k8ssandra-operator"}
	require.Eventually(t, func() bool {
		deployment := &appsv1.Deployment{}
		if err := Client.Get(context.TODO(), key, deployment); err != nil {
			t.Logf("failed to get k8ssandra-operator deployment: %v", err)
			return false
		}
		return deployment.Status.Replicas == deployment.Status.ReadyReplicas
	}, 60 * time.Second, 3 * time.Second)
}

func KustomizeAndApply(t *testing.T, dir, namespace string) error {
	retries := 3

	kustomize := exec.Command("/usr/local/bin/kustomize", "build", dir)
	var stdout, stderr bytes.Buffer
	kustomize.Stdout = &stdout
	kustomize.Stderr = &stderr
	err := kustomize.Run()

	require.NoError(t, err, "kustomize build failed")

	for i := 0; i < retries; i++ {
		kubectl := exec.Command("kubectl", "-n", namespace, "apply", "-f", "-")
		kubectl.Stdin = &stdout
		out, err := kubectl.CombinedOutput()

		t.Log(string(out))

		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("kubectl apply failed")
}
