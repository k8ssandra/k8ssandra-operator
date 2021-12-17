package e2e

import (
	"context"
	"testing"

	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"k8s.io/apimachinery/pkg/types"
)

func createSingleMedusa(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}

	checkDatacenterReady(t, ctx, dcKey, f)
}
