package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
)

func controllerRestart(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	operatorPod, err := getOperatorPod(ctx, f.Client, namespace)
	require.NoError(err)
	require.NotNil(operatorPod)

	clientConfigList := &configapi.ClientConfigList{}
	err = f.Client.List(ctx, clientConfigList, client.InNamespace(namespace))
	require.NoError(err)
	require.True(len(clientConfigList.Items) > 0)

	currentRestartCount := operatorPod.Status.ContainerStatuses[0].RestartCount

	clientConfig := &configapi.ClientConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configlist-to-crash-them-all",
			Namespace: namespace,
		},
		Spec: configapi.ClientConfigSpec{
			ContextName: "so-fatally-wrong",
		},
	}

	err = f.Client.Create(ctx, clientConfig)
	require.NoError(err)

	require.Eventually(func() bool {
		operatorPod, err := getOperatorPod(ctx, f.Client, namespace)
		if err != nil {
			return false
		}
		return operatorPod.Status.ContainerStatuses[0].RestartCount > currentRestartCount
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "Pod restartCount hasn't changed")
}

func getOperatorPod(ctx context.Context, anyClient client.Client, namespace string) (*corev1.Pod, error) {
	podList := &corev1.PodList{}
	err := anyClient.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels{"control-plane": "k8ssandra-operator"})

	if err != nil {
		return nil, err
	}

	if len(podList.Items) < 1 {
		return nil, fmt.Errorf("No operator pod found")
	}

	return &podList.Items[0], nil
}
