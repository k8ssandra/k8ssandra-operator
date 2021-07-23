package framework

import (
	"context"
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"text/template"
	"time"
)

const (
	defaultControlPlaneContext = "kind-k8ssandra-0"
)

type E2eFramework struct {
	*Framework

	nodeToolStatusUN *regexp.Regexp
}

func NewE2eFramework() (*E2eFramework, error) {
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

	controlPlaneContext := ""
	var controlPlaneClient client.Client
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

		// TODO Add a flag or option to allow the user to specify the control plane cluster
		//if len(ControlPlaneContext) == 0 {
		//	ControlPlaneContext = name
		//	controlPlaneClient = remoteClient
		//}
		remoteClients[name] = remoteClient
	}

	if remoteClient, found := remoteClients[defaultControlPlaneContext]; found {
		controlPlaneContext = defaultControlPlaneContext
		controlPlaneClient = remoteClient
	} else {
		for k8sContext, remoteClient := range remoteClients {
			controlPlaneContext = k8sContext
			controlPlaneClient = remoteClient
			break
		}
	}

	f := NewFramework(controlPlaneClient, controlPlaneContext, remoteClients)

	re := regexp.MustCompile("UN\\s\\s")

	return &E2eFramework{Framework: f, nodeToolStatusUN: re}, nil
}

// getClusterContexts returns all contexts, including both control plane and data plane.
func (f *E2eFramework) getClusterContexts() []string {
	contexts := make([]string, 0, len(f.remoteClients))
	for ctx, _ := range f.remoteClients {
		contexts = append(contexts, ctx)
	}
	return contexts
}

func (f *E2eFramework) getDataPlaneContexts() []string {
	contexts := make([]string, 0, len(f.remoteClients))
	for ctx, _ := range f.remoteClients {
		if ctx != f.ControlPlaneContext {
			contexts = append(contexts, ctx)
		}
	}
	return contexts
}

type Kustomization struct {
	Namespace string
}

func generateCassOperatorKustomization(namespace string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../config/cass-operator
namespace: {{ .Namespace }}
`
	k := Kustomization{Namespace: namespace}

	return generateKustomizationFile("cass-operator", k, tmpl)
}

func generateContextsKustomization(namespace string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

generatorOptions:
  disableNameSuffixHash: true

secretGenerator:
- files:
  - kubeconfig
  name: k8s-contexts
namespace: {{ .Namespace }}
`
	k := Kustomization{Namespace: namespace}

	if err := generateKustomizationFile("k8s-contexts", k, tmpl); err != nil {
		return err
	}

	src := filepath.Join("..", "..", "build", "in_cluster_kubeconfig")
	dest := filepath.Join("..", "..", "build", "test-config", "k8s-contexts", "kubeconfig")

	buf, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(dest, buf, 0644)
}

func generateK8ssandraOperatorKustomization(namespace string) error {
	tmpl := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/default
namespace: {{ .Namespace }}
`
	k := Kustomization{Namespace: namespace}

	err := generateKustomizationFile("k8ssandra-operator/control-plane", k, tmpl)
	if err != nil {
		return err
	}

	tmpl = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/default
namespace: {{ .Namespace }}

patchesJson6902:
  - target:
      group: apps
      version: v1
      kind: Deployment
      name: k8ssandra-operator
    patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/env/1/value
        value: "false"
`
	return generateKustomizationFile("k8ssandra-operator/data-plane", k, tmpl)
}

// generateKustomizationFile Creates the directory <project-root>/build/test-config/<name>
// and generates a kustomization.yaml file using the template tmpl. k defines values that
// will be substituted in the template.
func generateKustomizationFile(name string, k Kustomization, tmpl string) error {
	dir := filepath.Join("..", "..", "build", "test-config", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	parsed, err := template.New(name).Parse(tmpl)
	if err != nil {
		return nil
	}

	file, err := os.Create(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		return err
	}

	return parsed.Execute(file, k)
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

		options := kubectl.Options{Namespace: namespace, Context: defaultControlPlaneContext}
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

// DeployK8ssandraOperator Deploys k8ssandra-operator in the control plane cluster. Note
// that the control plane cluster can also be one of the data plane clusters. It then
// deploys the operator in the data plane clusters with the K8ssandraCluster controller
// disabled.
func (f *E2eFramework) DeployK8ssandraOperator(namespace string) error {
	if err := generateK8ssandraOperatorKustomization(namespace); err != nil {
		return err
	}

	baseDir := filepath.Join("..", "..", "build", "test-config", "k8ssandra-operator")
	controlPlane := filepath.Join(baseDir, "control-plane")
	dataPlane := filepath.Join(baseDir, "data-plane")

	err := f.kustomizeAndApply(controlPlane, namespace, f.ControlPlaneContext)
	if err != nil {
		return nil
	}

	dataPlaneContexts := f.getDataPlaneContexts()
	if len(dataPlaneContexts) > 0 {
		return f.kustomizeAndApply(dataPlane, namespace, dataPlaneContexts...)
	}

	return nil
}

// DeployCassOperator deploys cass-operator in all remote clusters.
func (f *E2eFramework) DeployCassOperator(namespace string) error {
	if err := generateCassOperatorKustomization(namespace); err != nil {
		return err
	}

	dir := filepath.Join("..", "..", "build", "test-config", "cass-operator")

	return f.kustomizeAndApply(dir, namespace, f.getClusterContexts()...)
}

// DeployK8sContextsSecret Deploys the contexts secret in the control plane cluster.
func (f *E2eFramework) DeployK8sContextsSecret(namespace string) error {
	if err := generateContextsKustomization(namespace); err != nil {
		return err
	}

	dir := filepath.Join("..", "..", "build", "test-config", "k8s-contexts")

	return f.kustomizeAndApply(dir, namespace, f.ControlPlaneContext)
}

// DeleteNamespace Deletes the namespace from all remote clusters and blocks until they
// have completely terminated.
func (f *E2eFramework) DeleteNamespace(name string, timeout, interval time.Duration) error {
	// TODO Make sure we delete from the control plane cluster as well

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

	// Should this wait.Poll call be per cluster?
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
	// TODO Add multi-cluster support.
	// By default this should wait for all clusters including the control plane cluster.

	return kubectl.WaitForCondition("established", "--timeout=60s", "--all", "crd")
}

// WaitForK8ssandraOperatorToBeReady blocks until the k8ssandra-operator deployment is
// ready in the control plane cluster.
func (f *E2eFramework) WaitForK8ssandraOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{
		K8sContext:     f.ControlPlaneContext,
		NamespacedName: types.NamespacedName{Namespace: namespace, Name: "k8ssandra-operator"},
	}
	return f.WaitForDeploymentToBeReady(key, timeout, interval)
}

// WaitForCassOperatorToBeReady blocks until the cass-operator deployment is ready in all
// clusters.
func (f *E2eFramework) WaitForCassOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cass-operator"}}
	return f.WaitForDeploymentToBeReady(key, timeout, interval)
}

// DumpClusterInfo Executes `kubectl cluster-info dump -o yaml` on each cluster. The output
// is stored under <project-root>/build/test.
func (f *E2eFramework) DumpClusterInfo(test, namespace string) error {
	f.logger.Info("dumping cluster info")

	now := time.Now()
	baseDir := fmt.Sprintf("../../build/test/%s/%d-%d-%d-%d-%d", test, now.Year(), now.Month(), now.Day(), now.Hour(), now.Second())
	errs := make([]error, 0)

	for ctx, _ := range f.remoteClients {
		outputDir := filepath.Join(baseDir, ctx)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("failed to make output for cluster %s: %w", ctx, err))
			return err
		}

		opts := kubectl.ClusterInfoOptions{Options: kubectl.Options{Namespace: namespace, Context: ctx}, OutputDirectory: outputDir}
		if err := kubectl.DumpClusterInfo(opts); err != nil {
			errs = append(errs, fmt.Errorf("failed to dump cluster info for cluster %s: %w", ctx, err))
		}
	}

	if len(errs) > 0 {
		return k8serrors.NewAggregate(errs)
	}
	return nil
}

// DeleteDatacenters deletes all CassandraDatacenters in namespace in all remote clusters.
// This function blocks until all pods from all CassandraDatacenters have terminated.
func (f *E2eFramework) DeleteDatacenters(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("deleting all CassandraDatacenters", "Namespace", namespace)
	return f.deleteAllResources(
		namespace,
		&cassdcapi.CassandraDatacenter{},
		timeout,
		interval,
		client.HasLabels{cassdcapi.ClusterLabel},
	)
}

// DeleteStargates deletes all Stargates in namespace in all remote clusters.
// This function blocks until all pods from all Stargates have terminated.
func (f *E2eFramework) DeleteStargates(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("deleting all Stargates", "Namespace", namespace)
	return f.deleteAllResources(
		namespace,
		&api.Stargate{},
		timeout,
		interval,
		client.HasLabels{api.StargateLabel},
	)
}

func (f *E2eFramework) deleteAllResources(
	namespace string,
	resource client.Object,
	timeout, interval time.Duration,
	podListOptions ...client.ListOption,
) error {
	for _, remoteClient := range f.remoteClients {
		if err := remoteClient.DeleteAllOf(context.TODO(), resource, client.InNamespace(namespace)); err != nil {
			// If the CRD wasn't deployed at all to this cluster, keep going
			if _, ok := err.(*meta.NoKindMatchError); !ok {
				return err
			}
		}
	}
	// Check that all pods created by resources are terminated
	// FIXME Should there be a separate wait.Poll call per cluster?
	return wait.Poll(interval, timeout, func() (bool, error) {
		podListOptions = append(podListOptions, client.InNamespace(namespace))
		for k8sContext, remoteClient := range f.remoteClients {
			list := &corev1.PodList{}
			if err := remoteClient.List(context.TODO(), list, podListOptions...); err != nil {
				f.logger.Error(err, "failed to list pods", "Context", k8sContext)
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

// GetNodeToolStatusUN Executes nodetool status against the Cassandra pod and returns a
// count of the matching lines reporting a status of Up/Normal.
func (f *E2eFramework) GetNodeToolStatusUN(opts kubectl.Options, pod string) (int, error) {
	output, err := kubectl.Exec(opts, pod, "nodetool", "status")
	if err != nil {
		return -1, err
	}



	matches := f.nodeToolStatusUN.FindAllString(output, -1)

	return len(matches), nil
}

// WaitForNodeToolStatusUN polls until nodetool status reports UN for count nodes.
func (f *E2eFramework) WaitForNodeToolStatusUN(opts kubectl.Options, pod string, count int, timeout, interval time.Duration) error {
	return wait.Poll(interval, timeout, func() (bool, error) {
		actual, err := f.GetNodeToolStatusUN(opts, pod)
		if err != nil {
			f.logger.Error(err, "failed to execute nodetool status for %s: %s", pod, err)
			return false, err
		}
		return actual == count, nil
	})

}
