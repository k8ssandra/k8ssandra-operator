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
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type E2eFramework struct {
	*Framework
}

func NewE2eFramework(cl client.Client) (*E2eFramework, error) {
	configFile, err := filepath.Abs("../../build/kubeconfig")
	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(configFile); err != nil {
		return nil, err
	}

	config, err := clientcmd.LoadFromFile(configFile)
	if err != nil {
		return nil, err
	}

	remoteClients := make(map[string]client.Client, 0)

	for name, _ := range config.Contexts {
		clientCfg := clientcmd.NewNonInteractiveClientConfig(*config, name, &clientcmd.ConfigOverrides{}, nil)
		restCfg, err := clientCfg.ClientConfig()

		if err != nil {
			return nil, err
		}

		remoteClient, err := client.New(restCfg, client.Options{Scheme: scheme.Scheme})
		if err != nil {
			return nil, err
		}

		remoteClients[name] = remoteClient
	}

	f := NewFramework(cl, remoteClients)

	return &E2eFramework{Framework: f}, nil
}

func (f *E2eFramework) getK8sContexts() []string {
	contexts := make([]string, 0, len(f.remoteClients))
	for ctx, _ := range f.remoteClients {
		contexts = append(contexts, ctx)
	}
	return contexts
}

func (f *E2eFramework) kustomizeAndApply(dir, namespace string, contexts ...string) error {
	kdir, err := filepath.Abs(dir)
	if err != nil {
		f.logger.Error(err, "failed to get full path", "dir", dir)
		return err
	}

	if err := kustomize.SetNamespace(kdir, namespace); err != nil {
		f.logger.Error(err, "failed to set namespace for kustomization directory", "dir", kdir)
		return err
	}

	if len(contexts) == 0 {
		buf, err := kustomize.Build(kdir)
		if err != nil {
			f.logger.Error(err, "kustomize build failed", "dir", kdir)
			return err
		}

		options := kubectl.Options{Namespace: namespace}
		return kubectl.Apply(options, buf)
	}

	for _, ctx := range contexts {
		buf, err := kustomize.Build(kdir)
		if err != nil {
			f.logger.Error(err, "kustomize build failed", "dir", kdir)
			return err
		}

		options := kubectl.Options{Namespace: namespace, Context: ctx}
		if err := kubectl.Apply(options, buf); err != nil {
			return err
		}
	}

	return nil
}

// DeployK8ssandraOperator Deploys k8ssandra-operator, cass-operator, and CRDs. This
// function blocks until CRDs are ready.
func (f *E2eFramework) DeployK8ssandraOperator(namespace string) error {
	dir := "../testdata/k8ssandra-operator"

	return f.kustomizeAndApply(dir, namespace);
}

func (f *E2eFramework) DeployCassOperator(namespace string) error {
	dir := "../testdata/cass-operator"
	return f.kustomizeAndApply(dir, namespace, f.getK8sContexts()...)
}

func (f *E2eFramework) DeployK8sContextsSecret(namespace string) error {
	dir := "../testdata/k8s-contexts"

	return f.kustomizeAndApply(dir, namespace)
}

func (f *E2eFramework) DeleteNamespace(name string, timeout, interval time.Duration) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for k8sContext, remoteClient := range f.remoteClients {
		f.logger.WithValues("deleting namespace", "Namespace", name, "Context", k8sContext)
		if err := remoteClient.Delete(context.Background(), namespace.DeepCopy()); err != nil {
			return err
		}
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		for _, remoteClient := range f.remoteClients {
			err := remoteClient.Get(context.TODO(), types.NamespacedName{Name: name}, namespace.DeepCopy())

			if err == nil || !apierrors.IsNotFound(err) {
				return false, nil
			}
		}

		return true, nil
	})
}

func (f *E2eFramework) WaitForCrdsToBecomeActive() error {
	return kubectl.WaitForCondition("established", "--timeout=60s", "--all", "crd")
}

func (f *E2eFramework) WaitForK8ssandraOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{
		K8sContext: f.controlPlaneContext,
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: "k8ssandra-operator"},
	}
	return f.WaitForDeploymentToBeReady(key, timeout, interval)
}

func (f *E2eFramework) WaitForCassOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cass-operator"}}
	return f.WaitForDeploymentToBeReady(key, timeout, interval)
}

func (f *E2eFramework) DumpClusterInfo(test, namespace string) error {
	// TODO Dump cluster info for each cluster
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

	for _, remoteClient := range f.remoteClients {
		ctx := context.TODO()
		dc := &cassdcapi.CassandraDatacenter{}

		if err := remoteClient.DeleteAllOf(ctx, dc, client.InNamespace(namespace)); err != nil {
			return err
		}
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		for k8sContext, remoteClient := range f.remoteClients {
			list := &corev1.PodList{}
			if err := remoteClient.List(context.TODO(), list, client.InNamespace(namespace), client.HasLabels{cassdcapi.ClusterLabel}); err != nil {
				f.logger.Error(err, "failed to list datacenter pods", "Context", k8sContext)
				return false, err
			}

			if len(list.Items) > 0 {
				return false, nil
			}
		}

		return true, nil
	})
}

func (f *E2eFramework) UndeployK8ssandraOperator(namespace string) error {
	dir, err := filepath.Abs("../testdata/k8ssandra-operator")
	if err != nil {
		return err
	}

	buf, err := kustomize.Build(dir)
	if err != nil {
		return err
	}

	options := kubectl.Options{Namespace: namespace}

	return kubectl.Delete(options, buf)
}
