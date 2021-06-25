package framework

import (
	"context"
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type E2eFramework struct {
	*Framework
}

func NewE2eFramework(client client.Client) *E2eFramework {
	f := NewFramework(client)

	return &E2eFramework{Framework: f}
}

// DeployK8ssandraOperator Deploys k8ssandra-operator, cass-operator, and CRDs. This
// function blocks until CRDs are ready.
func (f *E2eFramework) DeployK8ssandraOperator(namespace string) error {
	dir := "../testdata/k8ssandra-operator"
	kdir, err := filepath.Abs(dir)
	if err != nil {
		f.logger.Error(err, "failed to get full path", "dir", dir)
		return err
	}

	if err := kustomize.SetNamespace(nil, kdir, namespace); err != nil {
		f.logger.Error(err, "failed to set namespace for kustomization directory", "dir", kdir)
		return err
	}

	buf, err := kustomize.Build(kdir)
	if err != nil {
		f.logger.Error(err, "kustomize build failed", "dir", kdir)
		return err
	}

	if err = kubectl.ApplyBuffer(buf); err != nil {
		return err
	}

	if err = f.WaitForCrdsToBecomeActive(); err != nil {
		return err
	}

	return nil
}

func (f *E2eFramework) DeleteNamespace(name string, timeout, interval time.Duration) error {
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

func (f *E2eFramework) WaitForCrdsToBecomeActive() error {
	return kubectl.WaitForCondition(nil, "established", "--timeout=60s", "--all", "crd")
}

func (f *E2eFramework) WaitForK8ssandraOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	return f.WaitForDeploymentToBeReady(types.NamespacedName{Namespace: namespace, Name: "k8ssandra-operator"}, timeout, interval)
}

func (f *E2eFramework) WaitForCassOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	return f.WaitForDeploymentToBeReady(types.NamespacedName{Namespace: namespace, Name: "cass-operator"}, timeout, interval)
}

func (f *E2eFramework) DumpClusterInfo(test, namespace string) error {
	f.logger.Info("dumping cluster info")

	now := time.Now()
	outputDir := fmt.Sprintf("../../build/test/%s/%d-%d-%d-%d-%d", test, now.Year(), now.Month(), now.Day(), now.Hour(), now.Second())

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create test output directory %s: %s", outputDir, err)
	}

	return kubectl.DumpClusterInfo(namespace, outputDir)
}

// DeleteDatacenters deletes all CassandraDatacenters in namespace. This function blocks
// all pods have terminated.
func (f *E2eFramework) DeleteDatacenters(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("deleting all CassandraDatacenters", "Namespace", namespace)

	ctx := context.TODO()
	dc := &cassdcapi.CassandraDatacenter{}

	if err := f.Client.DeleteAllOf(ctx, dc, client.InNamespace(namespace)); err != nil {
		return err
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &corev1.PodList{}
		if err := Client.List(ctx, list, client.InNamespace(namespace), client.HasLabels{cassdcapi.ClusterLabel}); err != nil {
			f.logger.Error(err, "failed to list datacenter pods")
			return false, err
		}
		return len(list.Items) == 0, nil
	})
}

func (f *E2eFramework) UndeployK8ssandraOperator() error {
	dir, err := filepath.Abs("../testdata/k8ssandra-operator")
	if err != nil {
		return err
	}

	buf, err := kustomize.Build(dir)
	if err != nil {
		return err
	}

	if err = kubectl.DeleteBuffer(buf); err != nil {
		return err
	}

	return nil
}
