package framework

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"time"

	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultControlPlaneContext = "kind-k8ssandra-0"
)

type E2eFramework struct {
	*Framework

	nodeToolStatusUN *regexp.Regexp
}

func NewE2eFramework(t *testing.T) (*E2eFramework, error) {
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
	t.Logf("Using config file: %s", configFile)

	for name, _ := range config.Contexts {
		clientCfg := clientcmd.NewNonInteractiveClientConfig(*config, name, &clientcmd.ConfigOverrides{}, nil)
		restCfg, err := clientCfg.ClientConfig()

		if err != nil {
			return nil, err
		}

		remoteClient, err := client.New(restCfg, client.Options{Scheme: scheme.Scheme})
		if err == nil {
			remoteClients[name] = remoteClient
		}

		// TODO Add a flag or option to allow the user to specify the control plane cluster
		// if len(ControlPlaneContext) == 0 {
		//	ControlPlaneContext = name
		//	controlPlaneClient = remoteClient
		// }
	}
	if len(remoteClients) == 0 {
		return nil, fmt.Errorf("no valid context found in kubeconfig file")
	}
	t.Logf("Using config remote clients: %v", remoteClients)

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
- github.com/k8ssandra/cass-operator/config/deployments/cluster?ref=v1.9.0
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

func generateK8ssandraOperatorKustomization(namespace string, clusterScoped bool) error {
	controlPlaneDir := ""
	dataPlaneDir := ""
	controlPlaneTmpl := ""
	dataPlaneTmpl := ""

	if clusterScoped {
		controlPlaneDir = "control-plane-cluster-scope"
		dataPlaneDir = "data-plane-cluster-scope"

		controlPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/deployments/control-plane/cluster-scope

components:
- ../../../../config/components/mgmt-api-heap-size
`

		dataPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/deployments/data-plane/cluster-scope

components:
- ../../../../config/components/mgmt-api-heap-size
`
	} else {
		controlPlaneDir = "control-plane"
		dataPlaneDir = "data-plane"

		controlPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/deployments/control-plane

components:
- ../../../../config/components/mgmt-api-heap-size


patches:
- target:
    kind: Namespace
    labelSelector: "control-plane=k8ssandra-operator"
  options:
    allowNameChange: true
  patch: |
    - op: replace
      path: /metadata/name
      value: {{ .Namespace }}
replacements:
- source: 
    kind: Namespace
    name: test-ns
    fieldPath: metadata.name
  targets:
  - select:
      namespace: k8ssandra-operator
    fieldPaths:
    - metadata.namespace
`

		dataPlaneTmpl = `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../../../config/deployments/data-plane

components:
- ../../../../config/components/mgmt-api-heap-size

patches:
- target:
    kind: Namespace
    labelSelector: "control-plane=k8ssandra-operator"
  options:
    allowNameChange: true
  patch: |
    - op: replace
      path: /metadata/name
      value: {{ .Namespace }}
replacements:
- source: 
    kind: Namespace
    name: test-ns
    fieldPath: metadata.name
  targets:
  - select:
      namespace: k8ssandra-operator
    fieldPaths:
    - metadata.namespace
`
	}

	k := Kustomization{Namespace: namespace}

	err := generateKustomizationFile(fmt.Sprintf("k8ssandra-operator/%s", controlPlaneDir), k, controlPlaneTmpl)
	if err != nil {
		return err
	}

	return generateKustomizationFile(fmt.Sprintf("k8ssandra-operator/%s", dataPlaneDir), k, dataPlaneTmpl)
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
		buf, err := kustomize.BuildDir(kdir)
		if err != nil {
			f.logger.Error(err, "kustomize build failed", "dir", kdir)
			return err
		}

		options := kubectl.Options{Namespace: namespace, Context: defaultControlPlaneContext, ServerSide: true}
		return kubectl.Apply(options, buf)
	}

	for _, ctx := range contexts {
		f.logger.Info("kustomize build | kubectl apply", "Dir", dir, "Namespace", namespace, "Context", ctx)

		buf, err := kustomize.BuildDir(kdir)
		if err != nil {
			f.logger.Error(err, "kustomize build failed", "dir", kdir)
			return err
		}

		options := kubectl.Options{Namespace: namespace, Context: ctx, ServerSide: true}
		if err := kubectl.Apply(options, buf); err != nil {
			return err
		}
	}

	return nil
}

func (f *E2eFramework) DeployCassandraConfigMap(namespace string) error {
	path := filepath.Join("..", "testdata", "fixtures", "cassandra-config.yaml")

	for _, k8sContext := range f.getClusterContexts() {
		options := kubectl.Options{Namespace: namespace, Context: k8sContext}
		f.logger.Info("Create Cassandra ConfigMap", "Namespace", namespace, "Context", k8sContext)
		if err := kubectl.Apply(options, path); err != nil {
			return err
		}
	}

	return nil
}

// DeployK8ssandraOperator deploys k8ssandra-operator both in the control plane cluster and
// in the data plane cluster(s). Note that the control plane cluster can also be one of the
// data plane clusters. It then deploys the operator in the data plane clusters with the
// K8ssandraCluster controller disabled. When clusterScoped is true the operator is
// configured to watch all namespaces and is deployed in the k8ssandra-operator namespace.
func (f *E2eFramework) DeployK8ssandraOperator(namespace string, clusterScoped bool) error {
	if err := generateK8ssandraOperatorKustomization(namespace, clusterScoped); err != nil {
		return err
	}

	baseDir := filepath.Join("..", "..", "build", "test-config", "k8ssandra-operator")
	controlPlane := ""
	dataPlane := ""

	if clusterScoped {
		controlPlane = filepath.Join(baseDir, "control-plane-cluster-scope")
		dataPlane = filepath.Join(baseDir, "data-plane-cluster-scope")
	} else {
		controlPlane = filepath.Join(baseDir, "control-plane")
		dataPlane = filepath.Join(baseDir, "data-plane")
	}

	err := f.kustomizeAndApply(controlPlane, namespace, f.ControlPlaneContext)
	if err != nil {
		return err
	}

	dataPlaneContexts := f.getDataPlaneContexts()
	if len(dataPlaneContexts) > 0 {
		return f.kustomizeAndApply(dataPlane, namespace, dataPlaneContexts...)
	}

	return nil
}

func (f *E2eFramework) DeployCertManager() error {
	dir := filepath.Join("..", "..", "config", "cert-manager", "cert-manager-1.3.1.yaml")

	for _, ctx := range f.getClusterContexts() {
		options := kubectl.Options{Context: ctx}
		f.logger.Info("Deploy cert-manager", "Context", ctx)
		if err := kubectl.Apply(options, dir); err != nil {
			return err
		}
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

func (f *E2eFramework) DeployK8sClientConfigs(namespace string) error {
	baseDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return err
	}

	i := 0
	for k8sContext, _ := range f.remoteClients {
		f.logger.Info("Creating ClientConfig", "cluster", k8sContext)
		srcCfg := fmt.Sprintf("k8ssandra-%d.yaml", i)
		cmd := exec.Command(
			filepath.Join("scripts", "create-clientconfig.sh"),
			"--src-kubeconfig", filepath.Join("build", "kubeconfigs", srcCfg),
			"--dest-kubeconfig", filepath.Join("build", "kubeconfigs", "k8ssandra-0.yaml"),
			"--in-cluster-kubeconfig", filepath.Join("build", "kubeconfigs", "updated", srcCfg),
			"--output-dir", filepath.Join("build", "clientconfigs", srcCfg),
			"--namespace", namespace)
		cmd.Dir = baseDir

		fmt.Println(cmd)

		output, err := cmd.CombinedOutput()
		fmt.Println(string(output))

		if err != nil {
			return err
		}

		i++
	}

	return nil
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
		f.logger.Info("deleting namespace", "Namespace", name, "Context", k8sContext)
		if err := remoteClient.Delete(context.Background(), namespace.DeepCopy()); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	// Should this wait.Poll call be per cluster?
	return wait.Poll(interval, timeout, func() (bool, error) {
		for _, remoteClient := range f.remoteClients {
			err := remoteClient.Get(context.TODO(), types.NamespacedName{Name: name}, namespace.DeepCopy())

			if err == nil || !apierrors.IsNotFound(err) {
				f.logger.Info("waiting for namespace deletion", "error", err)
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

func (f *E2eFramework) WaitForCertManagerToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cert-manager"}}
	if err := f.WaitForDeploymentToBeReady(key, timeout, interval); err != nil {
		return nil
	}

	key.NamespacedName.Name = "cert-manager-cainjector"
	if err := f.WaitForDeploymentToBeReady(key, timeout, interval); err != nil {
		return err
	}

	key.NamespacedName.Name = "cert-manager-webhook"
	if err := f.WaitForDeploymentToBeReady(key, timeout, interval); err != nil {
		return err
	}

	return nil
}

// WaitForCassOperatorToBeReady blocks until the cass-operator deployment is ready in all
// clusters.
func (f *E2eFramework) WaitForCassOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := ClusterKey{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "cass-operator-controller-manager"}}
	return f.WaitForDeploymentToBeReady(key, timeout, interval)
}

// DumpClusterInfo Executes `kubectl cluster-info dump -o yaml` on each cluster. The output
// is stored under <project-root>/build/test.
func (f *E2eFramework) DumpClusterInfo(test string, namespace ...string) error {
	f.logger.Info("dumping cluster info")

	now := time.Now()
	testDir := strings.ReplaceAll(test, "/", "_")
	baseDir := fmt.Sprintf("../../build/test/%s/%d-%d-%d-%d-%d", testDir, now.Year(), now.Month(), now.Day(), now.Hour(), now.Second())
	errs := make([]error, 0)

	for ctx, _ := range f.remoteClients {
		outputDir := filepath.Join(baseDir, ctx)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("failed to make output for cluster %s: %w", ctx, err))
			return err
		}

		opts := kubectl.ClusterInfoOptions{Options: kubectl.Options{Context: ctx}, OutputDirectory: outputDir}
		if len(namespace) == 1 {
			opts.Namespace = namespace[0]
		} else {
			opts.Namespaces = namespace
		}

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
		&stargateapi.Stargate{},
		timeout,
		interval,
		client.HasLabels{stargateapi.StargateLabel},
	)
}

// DeleteReapers deletes all Reapers in namespace in all remote clusters.
// This function blocks until all pods from all Reapers have terminated.
func (f *E2eFramework) DeleteReapers(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("deleting all Reapers", "Namespace", namespace)
	return f.deleteAllResources(
		namespace,
		&reaperapi.Reaper{},
		timeout,
		interval,
		client.HasLabels{reaperapi.ReaperLabel},
	)
}

// DeleteReplicatedSecrets deletes all the ReplicatedSecrets in the namespace. This causes
// some delay while secret controller removes the finalizers and clears the replicated secrets from
// remote clusters.
func (f *E2eFramework) DeleteReplicatedSecrets(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("deleting all ReplicatedSecrets", "Namespace", namespace)

	if err := f.Client.DeleteAllOf(context.Background(), &replicationapi.ReplicatedSecret{}, client.InNamespace(namespace)); err != nil {
		f.logger.Error(err, "failed to delete ReplicatedSecrets")
		return err
	}

	return wait.Poll(interval, timeout, func() (bool, error) {
		list := &replicationapi.ReplicatedSecretList{}
		if err := f.Client.List(context.Background(), list, client.InNamespace(namespace)); err != nil {
			f.logger.Error(err, "failed to list ReplicatedSecrets")
			return false, err
		}
		return len(list.Items) == 0, nil
	})
}

func (f *E2eFramework) DeleteK8ssandraOperatorPods(namespace string, timeout, interval time.Duration) error {
	f.logger.Info("deleting all k8ssandra-operator pods", "Namespace", namespace)
	if err := f.Client.DeleteAllOf(context.TODO(), &corev1.Pod{}, client.InNamespace(namespace), client.MatchingLabels{"control-plane": "k8ssandra-operator"}); err != nil {
		return err
	}
	return nil
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
				return false, nil
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

	f.logger.Info("Undeploy k8ssandra-operator", "Namespace", namespace)

	buf, err := kustomize.BuildDir(dir)
	if err != nil {
		return err
	}

	options := kubectl.Options{Namespace: namespace}

	return kubectl.Delete(options, buf)
}

// GetNodeToolStatusUN Executes nodetool status against the Cassandra pod and returns a
// count of the matching lines reporting a status of Up/Normal.
func (f *E2eFramework) GetNodeToolStatusUN(k8sContext, namespace, pod string, additionalArgs ...string) (int, error) {
	opts := kubectl.Options{Namespace: namespace, Context: k8sContext}
	args := []string{"nodetool"}
	args = append(args, additionalArgs...)
	args = append(args, "status")
	output, err := kubectl.Exec(opts, pod, args...)
	if err != nil {
		// remove stack traces and only keep the first line of stderr
		scanner := bufio.NewScanner(strings.NewReader(output))
		if scanner.Scan() {
			output = scanner.Text()
		}
		err = fmt.Errorf("%s (%w)", output, err)
		f.logger.Error(err, fmt.Sprintf("failed to execute nodetool status on %s: %s", pod, err))
		return -1, err
	}
	matches := f.nodeToolStatusUN.FindAllString(output, -1)
	return len(matches), nil
}

func (f *E2eFramework) GetPodIP(k8sContext, namespace, pod string) (string, error) {
	opts := kubectl.Options{Namespace: namespace, Context: k8sContext}
	output, err := kubectl.Get(opts, "pod", pod, "-o", "jsonpath={.status.podIP}")
	if err != nil {
		err = fmt.Errorf("%s (%w)", output, err)
		f.logger.Error(err, fmt.Sprintf("failed to get pod IP for %s: %s", pod, err))
		return "", err
	}
	return output, nil
}
