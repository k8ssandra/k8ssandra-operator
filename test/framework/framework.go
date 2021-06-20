package framework

import (
	"context"
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
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
}

func CreateNamespace(t *testing.T, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	t.Logf("creating namespace %s", name)
	if err := Client.Create(context.Background(), namespace); err != nil {
		return err
	}
	return nil
}

func DeleteNamespace(name string, timeout, interval time.Duration) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := Client.Delete(context.TODO(), namespace); err != nil {
		return err
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		err := Client.Get(context.TODO(), types.NamespacedName{Name: name}, namespace)

		if err == nil {
			return false, nil
		}

		if apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	})
}

func DeployK8ssandraOperator(t *testing.T, namespace string) error {
	dir := "../testdata/k8ssandra-operator"
	kdir, err := GetAbsPath(dir)
	if err != nil {
		t.Logf("failed to get full path for %s: %v", dir, err)
		return err
	}

	if err := kustomize.SetNamespace(t, kdir, namespace); err != nil {
		t.Logf("failed to set namespace for %s: %v", kdir, err)
		return err
	}

	buf, err := kustomize.Build(t, kdir)
	if err != nil {
		t.Logf("kustomize build failed for %s: %v", kdir, err)
		return err
	}

	if err = kubectl.ApplyBuffer(t, buf); err != nil {
		return err
	}

	if err = WaitForCrdsToBecomeActive(t); err != nil {
		return err
	}

	return nil
}

func UndeployK8ssandraOperator(t *testing.T) error {
	dir, err := GetAbsPath("../testdata/k8ssandra-operator")
	if err != nil {
		return err
	}

	buf, err := kustomize.Build(t, dir)
	if err != nil {
		return err
	}

	if err = kubectl.DeleteBuffer(t, buf); err != nil {
		return err
	}

	return nil
}

func GetAbsPath(dir string) (string, error) {
	if path, err := filepath.Abs(dir); err == nil {
		return filepath.Clean(path), nil
	} else {
		return "", err
	}
}

func WaitForDeploymentToBeReady(t *testing.T, key types.NamespacedName, timeout, interval time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		deployment := &appsv1.Deployment{}
		if err := Client.Get(context.TODO(), key, deployment); err != nil {
			t.Logf("failed to get deployment %s: %v", key, err)
			return false, err
		}
		return deployment.Status.Replicas == deployment.Status.ReadyReplicas, nil
	})
}

func WaitForCrdsToBecomeActive(t *testing.T) error {
	return kubectl.WaitForCondition(t, "established", "--timeout=60s", "--all", "crd")
}

func DeleteK8ssandraClusters(t *testing.T, namespace string) error {
	// TODO This will need to be updated when we add a finalizer in K8ssandraCluster
    // We will want to block until that finalizer is removed.

	t.Logf("deleting k8ssandra clusters in namespace %s", namespace)
	k8ssandra := &api.K8ssandraCluster{}
	return Client.DeleteAllOf(context.TODO(), k8ssandra, client.InNamespace(namespace))
}

func DeleteDatacenters(t *testing.T, namespace string) error {
	t.Logf("deleting datacenters in namespace %s", namespace)

	ctx := context.TODO()
	dc := &cassdcapi.CassandraDatacenter{}

	if err := Client.DeleteAllOf(ctx, dc, client.InNamespace(namespace)); err != nil {
		return err
	}

	timeout := 3 * time.Minute
	interval := 10 * time.Second

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &corev1.PodList{}
		if err := Client.List(ctx, list, client.InNamespace(namespace), client.HasLabels{cassdcapi.ClusterLabel}); err != nil {
			t.Logf("failed to list datacenter pods: %v", err)
			return false, err
		}
		return len(list.Items) == 0, nil
	})
}

func DumpClusterInfo(t *testing.T, namespace string) error {
	t.Log("dumping cluster info")

	now := time.Now()
	outputDir := fmt.Sprintf("../../build/test/%s/%d-%d-%d-%d-%d", t.Name(), now.Year(), now.Month(), now.Day(), now.Hour(), now.Second())
	
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create test output directory %s: %s", outputDir, err)
	}

	return kubectl.DumpClusterInfo(t, namespace, outputDir)
}

