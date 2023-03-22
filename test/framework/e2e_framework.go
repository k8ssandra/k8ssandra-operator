package framework

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"
	"time"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/stretchr/testify/require"

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
	repoName  = "k8ssandra"
	relName   = "k8ssandra-operator"
	chartName = "k8ssandra-operator"
	repoURL   = "https://helm.k8ssandra.io/stable"
)

type E2eFramework struct {
	*Framework
	cqlshBin string
}

var (
	nodeToolStatusUN = regexp.MustCompile("UN\\s\\s")
	nodeToolStatusDN = regexp.MustCompile("DN\\s\\s")
)

func NewE2eFramework(t *testing.T, kubeconfigFile string, useDse bool, controlPlane string, dataPlanes ...string) (*E2eFramework, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigFile)
	if err != nil {
		return nil, err
	}

	// Specify if DSE is used to adjust paths to binaries (cqlsh, ...)
	cqlshBinLocation := "/opt/cassandra/bin/cqlsh"
	if useDse {
		cqlshBinLocation = "/opt/dse/bin/cqlsh"
	}

	remoteClients := make(map[string]client.Client, 0)
	t.Logf("Using config file: %s", kubeconfigFile)

	if remoteClient, err := newRemoteClient(config, controlPlane); err != nil {
		return nil, err
	} else if remoteClient == nil {
		return nil, fmt.Errorf("control plane context %s does not exist", controlPlane)
	} else {
		remoteClients[controlPlane] = remoteClient
		t.Logf("Using control plane: %v", controlPlane)
	}

	var validDataPlanes []string
	for _, name := range dataPlanes {
		if remoteClient, err := newRemoteClient(config, name); err != nil {
			return nil, err
		} else if remoteClient == nil {
			t.Logf("ignoring invalid data plane context %v", name)
		} else {
			remoteClients[name] = remoteClient
			validDataPlanes = append(validDataPlanes, name)
		}
	}
	if len(validDataPlanes) == 0 {
		return nil, errors.New("no valid data planes found")
	}
	t.Logf("Using data planes: %v", validDataPlanes)

	f := NewFramework(remoteClients[controlPlane], controlPlane, validDataPlanes, remoteClients)

	return &E2eFramework{Framework: f, cqlshBin: cqlshBinLocation}, nil
}

func newRemoteClient(config *clientcmdapi.Config, context string) (client.Client, error) {
	if len(context) == 0 {
		// empty context name loads the current context: prevent that for security reasons
		return nil, errors.New("empty context name provided")
	}
	clientCfg := clientcmd.NewNonInteractiveClientConfig(*config, context, &clientcmd.ConfigOverrides{}, nil)
	if restCfg, err := clientCfg.ClientConfig(); err != nil {
		return nil, nil
	} else if remoteClient, err := client.New(restCfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		return nil, err
	} else {
		return remoteClient, nil
	}
}

func generateK8ssandraOperatorKustomization(config OperatorDeploymentConfig) error {
	controlPlaneDir := "control-plane"
	dataPlaneDir := "data-plane"
	config.ControlPlaneComponent = "../../../../config/deployments/control-plane"
	config.DataPlaneComponent = "../../../../config/deployments/data-plane"

	if config.GithubKustomization {
		config.ControlPlaneComponent = "github.com/k8ssandra/k8ssandra-operator/config/deployments/control-plane?ref=" + config.ImageTag
		config.DataPlaneComponent = "github.com/k8ssandra/k8ssandra-operator/config/deployments/data-plane?ref=" + config.ImageTag
	}
	controlPlaneTmpl := `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- {{ .ControlPlaneComponent }}

images:
  - name: k8ssandra/k8ssandra-operator
    newName: {{ .ImageName }}
    newTag: {{ .ImageTag }}

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
    name: {{ .Namespace }}
    fieldPath: metadata.name
  targets:
  - select:
      namespace: cass-operator
    fieldPaths:
      - metadata.namespace
  - select:
      namespace: k8ssandra-operator
    fieldPaths:
      - metadata.namespace
  - select:
      kind: ClusterRoleBinding
    fieldPaths:
      - subjects.0.namespace
  - select:
      name: cass-operator-validating-webhook-configuration
      kind: ValidatingWebhookConfiguration
    fieldPaths:
      - webhooks.0.clientConfig.service.namespace
  - select:
      name: k8ssandra-operator-validating-webhook-configuration
      kind: ValidatingWebhookConfiguration
    fieldPaths:
      - webhooks.0.clientConfig.service.namespace
  - select:
      name: k8ssandra-operator-validating-webhook-configuration
      kind: ValidatingWebhookConfiguration
    fieldPaths:
      - webhooks.1.clientConfig.service.namespace
  - select:
      name: k8ssandra-operator-mutating-webhook-configuration
      kind: MutatingWebhookConfiguration
    fieldPaths:
      - webhooks.0.clientConfig.service.namespace
`

	dataPlaneTmpl := `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- {{ .DataPlaneComponent }}

images:
  - name: k8ssandra/k8ssandra-operator
    newName: {{ .ImageName }}
    newTag: {{ .ImageTag }}

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
    name: {{ .Namespace }}
    fieldPath: metadata.name
  targets:
  - select:
      namespace: cass-operator
    fieldPaths:
      - metadata.namespace
  - select:
      namespace: k8ssandra-operator
    fieldPaths:
      - metadata.namespace
  - select:
      kind: ClusterRoleBinding
    fieldPaths:
      - subjects.0.namespace
  - select:
      name: cass-operator-validating-webhook-configuration
      kind: ValidatingWebhookConfiguration
    fieldPaths:
      - webhooks.0.clientConfig.service.namespace
  - select:
      name: k8ssandra-operator-validating-webhook-configuration
      kind: ValidatingWebhookConfiguration
    fieldPaths:
      - webhooks.0.clientConfig.service.namespace
  - select:
      name: k8ssandra-operator-validating-webhook-configuration
      kind: ValidatingWebhookConfiguration
    fieldPaths:
      - webhooks.1.clientConfig.service.namespace
  - select:
      name: k8ssandra-operator-mutating-webhook-configuration
      kind: MutatingWebhookConfiguration
    fieldPaths:
      - webhooks.0.clientConfig.service.namespace
`

	err := generateKustomizationFile(fmt.Sprintf("k8ssandra-operator/%s", controlPlaneDir), config, controlPlaneTmpl)
	if err != nil {
		return err
	}

	return generateKustomizationFile(fmt.Sprintf("k8ssandra-operator/%s", dataPlaneDir), config, dataPlaneTmpl)
}

// generateKustomizationFile Creates the directory <project-root>/build/test-config/<name>
// and generates a kustomization.yaml file using the template tmpl. k defines values that
// will be substituted in the template.
func generateKustomizationFile(name string, data interface{}, tmpl string) error {
	dir := filepath.Join("..", "..", "build", "test-config", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	parsed, err := template.New(name).Parse(tmpl)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		return err
	}

	return parsed.Execute(file, data)
}

func (f *E2eFramework) kustomizeAndApply(dir, namespace string, context string) error {
	kdir, err := filepath.Abs(dir)
	if err != nil {
		f.logger.Error(err, "failed to get full path", "dir", dir)
		return err
	}

	if len(context) == 0 {
		return errors.New("no context provided")
	}

	f.logger.Info("kustomize build | kubectl apply", "Dir", dir, "Namespace", namespace, "Context", context)

	buf, err := kustomize.BuildDir(kdir)
	if err != nil {
		f.logger.Error(err, "kustomize build failed", "dir", kdir)
		return err
	}

	options := kubectl.Options{Context: context, ServerSide: true, Namespace: namespace}
	if err := kubectl.Apply(options, buf); err != nil {
		return err
	}

	return nil
}

func (f *E2eFramework) DeployCassandraConfigMap(namespace string) error {
	path := filepath.Join("..", "testdata", "fixtures", "cassandra-config.yaml")

	for _, k8sContext := range f.DataPlaneContexts {
		options := kubectl.Options{Namespace: namespace, Context: k8sContext}
		f.logger.Info("Create Cassandra ConfigMap", "Namespace", namespace, "Context", k8sContext)
		if err := kubectl.Apply(options, path); err != nil {
			return err
		}
	}

	return nil
}

func (f *E2eFramework) CreateCassandraEncryptionStoresSecret(namespace string) error {
	for _, storeType := range []encryption.StoreType{encryption.StoreTypeServer, encryption.StoreTypeClient} {
		path := filepath.Join("..", "testdata", "fixtures", fmt.Sprintf("%s-encryption-secret.yaml", storeType))

		for _, k8sContext := range f.DataPlaneContexts {
			options := kubectl.Options{Namespace: namespace, Context: k8sContext}
			f.logger.Info("Create Cassandra Encryption secrets", "Namespace", namespace, "Context", k8sContext)
			if err := kubectl.Apply(options, path); err != nil {
				return err
			}
		}
	}

	// Create client certificates secret
	path := filepath.Join("..", "testdata", "fixtures", "client-certificates-secret.yaml")

	for _, k8sContext := range f.DataPlaneContexts {
		options := kubectl.Options{Namespace: namespace, Context: k8sContext}
		f.logger.Info("Create client certificates secrets", "Namespace", namespace, "Context", k8sContext)
		if err := kubectl.Apply(options, path); err != nil {
			return err
		}
	}

	return nil
}

type OperatorDeploymentConfig struct {
	Namespace             string
	ClusterScoped         bool
	ImageName             string
	ImageTag              string
	GithubKustomization   bool // If true, use the kustomization.yaml from the github repo
	ControlPlaneComponent string
	DataPlaneComponent    string
}

// DeployK8ssandraOperator deploys k8ssandra-operator both in the control plane cluster and
// in the data plane cluster(s). Note that the control plane cluster can also be one of the
// data plane clusters. It then deploys the operator in the data plane clusters with the
// K8ssandraCluster controller disabled. When clusterScoped is true the operator is
// configured to watch all namespaces and is deployed in the k8ssandra-operator namespace.
func (f *E2eFramework) DeployK8ssandraOperator(config OperatorDeploymentConfig) error {
	var (
		baseDir      string
		controlPlane string
		dataPlane    string
	)

	// Kustomize based installation
	if config.ClusterScoped {
		baseDir = filepath.Join("..", "..", "config", "deployments")
		controlPlane = filepath.Join(baseDir, "control-plane", "cluster-scope")
		dataPlane = filepath.Join(baseDir, "data-plane", "cluster-scope")
	} else {
		baseDir = filepath.Join("..", "..", "build", "test-config", "k8ssandra-operator")
		controlPlane = filepath.Join(baseDir, "control-plane")
		dataPlane = filepath.Join(baseDir, "data-plane")

		if err := generateK8ssandraOperatorKustomization(config); err != nil {
			return err
		}
	}

	f.logger.Info("Deploying operator in control plane", "Namespace", config.Namespace, "Context", f.ControlPlaneContext)
	err := f.kustomizeAndApply(controlPlane, config.Namespace, f.ControlPlaneContext)
	if err != nil {
		return err
	}

	for _, dataPlaneContext := range f.DataPlaneContexts {
		if dataPlaneContext != f.ControlPlaneContext {
			f.logger.Info("Deploying operator in data plane", "Namespace", config.Namespace, "Context", dataPlaneContext)
			err = f.kustomizeAndApply(dataPlane, config.Namespace, dataPlaneContext)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *E2eFramework) DeployK8sClientConfigs(namespace, srcKubeconfig, destKubeconfig, destContext string) error {
	for _, srcContext := range f.DataPlaneContexts {
		f.logger.Info("Creating ClientConfig", "src-context", srcContext)
		cmd := exec.Command(
			filepath.Join("..", "..", "scripts", "create-clientconfig.sh"),
			"--src-kubeconfig", srcKubeconfig,
			"--dest-kubeconfig", destKubeconfig,
			"--src-context", srcContext,
			"--dest-context", destContext,
			"--namespace", namespace)

		fmt.Println(cmd)

		output, err := cmd.CombinedOutput()

		if err != nil {
			fmt.Println(string(output))
			return err
		}
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
	for k8sContext := range f.remoteClients {
		err := kubectl.WaitForCondition(kubectl.Options{Context: k8sContext}, "established", "--timeout=60s", "--all", "crd")
		if err != nil {
			return err
		}
	}
	return nil
}

// WaitForK8ssandraOperatorToBeReady blocks until the k8ssandra-operator deployment is
// ready in the control plane and all data planes.
func (f *E2eFramework) WaitForK8ssandraOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := NewClusterKey("", namespace, "k8ssandra-operator")
	return f.WaitForDeploymentToBeReady(key, timeout, interval)
}

// WaitForCassOperatorToBeReady blocks until the cass-operator deployment is ready in the control
// plane and all data-planes.
func (f *E2eFramework) WaitForCassOperatorToBeReady(namespace string, timeout, interval time.Duration) error {
	key := NewClusterKey("", namespace, "cass-operator-controller-manager")
	return f.WaitForDeploymentToBeReady(key, timeout, interval)
}

// DumpClusterInfo Executes `kubectl cluster-info dump -o yaml` on each cluster. The output
// is stored under <project-root>/build/test.
func (f *E2eFramework) DumpClusterInfo(test string, namespaces ...string) error {
	f.logger.Info("dumping cluster info")
	f.logger.Info("Namespaces", "namespaces", namespaces)

	now := time.Now()
	testDir := strings.ReplaceAll(test, "/", "_")
	baseDir := fmt.Sprintf("../../build/test/%s/%d-%d-%d-%d-%d", testDir, now.Year(), now.Month(), now.Day(), now.Hour(), now.Second())
	errs := make([]error, 0)

	for ctx := range f.remoteClients {
		outputDir := filepath.Join(baseDir, ctx)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}

		opts := kubectl.ClusterInfoOptions{Options: kubectl.Options{Context: ctx}, OutputDirectory: outputDir}
		if len(namespaces) == 1 {
			opts.Namespace = namespaces[0]
		} else {
			opts.Namespaces = namespaces
		}

		if err := kubectl.DumpClusterInfo(opts); err != nil {
			errs = append(errs, fmt.Errorf("failed to dump cluster info for cluster %s: %w", ctx, err))
		}

		for _, namespace := range namespaces {
			// Store the list of pods in an easy to read format.
			output, err := kubectl.Get(kubectl.Options{Context: ctx, Namespace: namespace}, "pods", "-o", "wide")
			if err != nil {
				f.logger.Info("dump failed", "output", output, "error", err)
				errs = append(errs, fmt.Errorf("failed to dump %s for cluster %s: %w", "pods", ctx, err))
			}
			f.storeOutput(outputDir, namespace, "pods", "out", output)

			// Dump all objects that we need to investigate failures as a flat list and as yaml manifests
			for _, objectType := range []string{"K8ssandraCluster", "CassandraDatacenter", "Stargate", "Reaper", "StatefulSet", "Secrets",
				"ReplicatedSecret", "ClientConfig", "CassandraTask", "MedusaBackup", "MedusaBackupJob", "MedusaRestoreJob", "MedusaTask"} {
				if err := os.MkdirAll(fmt.Sprintf("%s/%s/objects/%s", outputDir, namespace, objectType), 0755); err != nil {
					return err
				}

				// Get the list of objects
				output, err := kubectl.Get(kubectl.Options{Context: ctx, Namespace: namespace}, objectType, "-o", "wide")
				if err != nil {
					f.logger.Info("dump failed", "output", output, "error", err)
					errs = append(errs, fmt.Errorf("failed to dump %s for cluster %s: %w", objectType, ctx, err))
				}
				f.storeOutput(outputDir, fmt.Sprintf("%s/objects/%s", namespace, objectType), objectType, "out", output)

				// Get the yamls for each object
				outputYaml, err := kubectl.Get(kubectl.Options{Context: ctx, Namespace: namespace}, objectType, "-o", "yaml")
				if err != nil {
					f.logger.Info("dump failed", "output", output, "error", err)
					errs = append(errs, fmt.Errorf("failed to dump %s for cluster %s: %w", objectType, ctx, err))
				}
				f.storeOutput(outputDir, fmt.Sprintf("%s/objects/%s", namespace, objectType), objectType, "yaml", outputYaml)
			}
		}
	}

	if len(errs) > 0 {
		return k8serrors.NewAggregate(errs)
	}
	return nil
}

func (f *E2eFramework) storeOutput(outputDir, subdirectory, objectType, ext, output string) error {
	filePath := fmt.Sprintf("%s/%s/%s.%s", outputDir, subdirectory, objectType, ext)
	outputFile, err := os.Create(filePath)
	if err != nil {
		f.logger.Error(err, "failed to create output file")
		return err
	}
	defer outputFile.Close()
	outputFile.WriteString(output)
	outputFile.Sync()
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
	f.logger.Info("Deleting all k8ssandra-operator pods in control plane", "Context", f.ControlPlaneContext, "Namespace", namespace)
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

// GetNodeToolStatus Executes nodetool status against the Cassandra pod and returns a
// count of the matching lines reporting a status of Up/Normal and Down/Normal.
func (f *E2eFramework) GetNodeToolStatus(k8sContext, namespace, pod string, additionalArgs ...string) (int, int, error) {
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
		return -1, -1, err
	}
	countUN := len(nodeToolStatusUN.FindAllString(output, -1))
	countDN := len(nodeToolStatusDN.FindAllString(output, -1))
	return countUN, countDN, nil
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

func (f *E2eFramework) GetContainerLogs(k8sContext, namespace, pod, container string) (string, error) {
	opts := kubectl.Options{Namespace: namespace, Context: k8sContext}
	output, err := kubectl.Logs(opts, pod, "-c", container)
	if err != nil {
		err = fmt.Errorf("%s (%w)", output, err)
		f.logger.Error(err, fmt.Sprintf("failed to get logs for %s/%s: %s", pod, container, err))
		return "", err
	}
	return output, nil
}

// GetCassandraPodIPs returns the Cassandra pods for a given cassdc.
func (f *E2eFramework) GetCassandraDatacenterPods(t *testing.T, ctx context.Context, dcKey ClusterKey) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	labels := client.MatchingLabels{cassdcapi.DatacenterLabel: dcKey.Name}
	err := f.List(ctx, dcKey, podList, labels)
	require.NoError(t, err, "failed to get pods for cassandradatacenter", "CassandraDatacenter", dcKey.Name)

	pods := make([]corev1.Pod, 0)
	pods = append(pods, podList.Items...)

	return pods, nil
}
