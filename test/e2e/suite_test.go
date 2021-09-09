package e2e

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
)

type pollingConfig struct {
	timeout  time.Duration
	interval time.Duration
}

var (
	nodetoolStatusTimeout time.Duration

	polling struct {
		nodetoolStatus          pollingConfig
		datacenterReady         pollingConfig
		operatorDeploymentReady pollingConfig
		k8ssandraClusterStatus  pollingConfig
		stargateReady           pollingConfig
		reaperReady             pollingConfig
	}

	logKustomizeOutput  = flag.Bool("logKustomizeOutput", false, "")
	logKubectlOutput    = flag.Bool("logKubectlOutput", false, "")
	cassandraVersion    = flag.String("cassandraVersion", "4.0.0", "Set version of Cassandra to be used in the e2e tests")
	controlPlaneContext = flag.String("controlPlane", "", "Override control plane context name")
)

func TestOperator(t *testing.T) {
	beforeSuite(t)

	ctx := context.Background()

	applyPollingDefaults()

	t.Run("CreateSingleDatacenterCluster", e2eTest(ctx, &e2eTestOpts{
		testFunc:      createSingleDatacenterCluster,
		fixture:       "single-dc",
		deployTraefik: true,
	}))
	t.Run("CreateStargateAndDatacenter", e2eTest(ctx, &e2eTestOpts{
		testFunc:                     createStargateAndDatacenter,
		fixture:                      "stargate",
		deployTraefik:                true,
		skipK8ssandraClusterCleanup:  true,
		doCassandraDatacenterCleanup: true,
	}))
	t.Run("CreateMultiDatacenterCluster", e2eTest(ctx, &e2eTestOpts{
		testFunc: createMultiDatacenterCluster,
		fixture:  "multi-dc",
	}))
	t.Run("CreateMultiStargateAndDatacenter", e2eTest(ctx, &e2eTestOpts{
		testFunc:      createMultiDatacenterCluster,
		fixture:       "multi-dc",
		deployTraefik: true,
	}))
	t.Run("CheckStargateApisWithMultiDcCluster", e2eTest(ctx, &e2eTestOpts{
		testFunc:      checkStargateApisWithMultiDcCluster,
		fixture:       "multi-dc-stargate",
		deployTraefik: true,
	}))
	t.Run("CreateSingleReaper", e2eTest(ctx, &e2eTestOpts{
		testFunc:      createSingleReaper,
		fixture:       "single-dc-reaper",
		deployTraefik: true,
	}))
	t.Run("CreateMultiReaper", e2eTest(ctx, &e2eTestOpts{
		testFunc:      createMultiReaper,
		fixture:       "multi-dc-reaper",
		deployTraefik: true,
	}))
	t.Run("CreateReaperAndDatacenter", e2eTest(ctx, &e2eTestOpts{
		testFunc:                     createReaperAndDatacenter,
		fixture:                      "reaper",
		deployTraefik:                true,
		skipK8ssandraClusterCleanup:  true,
		doCassandraDatacenterCleanup: true,
	}))
	t.Run("ClusterScoped", func(t *testing.T) {
		t.Run("MultiDcMultiCluster", e2eTest(ctx, &e2eTestOpts{
			testFunc:             multiDcMultiCluster,
			fixture:              "multi-dc-cluster-scope",
			clusterScoped:        true,
			sutNamespace:         "test-0",
			additionalNamespaces: []string{"test-1", "test-2"},
		}))
	})
}

func beforeSuite(t *testing.T) {
	flag.Parse()

	if val, ok := os.LookupEnv("NODETOOL_STATUS_TIMEOUT"); ok {
		timeout, err := time.ParseDuration(val)
		require.NoError(t, err, fmt.Sprintf("failed to parse NODETOOL_STATUS_TIMEOUT value: %s", val))
		nodetoolStatusTimeout = timeout
	} else {
		nodetoolStatusTimeout = 1 * time.Minute
	}

	kustomize.LogOutput(*logKustomizeOutput)
	kubectl.LogOutput(*logKubectlOutput)

	cfgFile, err := filepath.Abs("../../build/kubeconfig")
	if err != nil {
		t.Fatalf("failed to get path of src kind kubeconfig file: %v", err)
	}
	require.FileExistsf(t, cfgFile, "kind kubeconfig file is missing", "path", cfgFile)

	inClusterCfgFile, err := filepath.Abs("../../build/in_cluster_kubeconfig")
	if err != nil {
		t.Fatalf("failed to get path of src kind kubeconfig file: %v", err)
	}
	require.FileExistsf(t, inClusterCfgFile, "in-cluster kind kubeconfig file is missing", "path", inClusterCfgFile)

	// TODO this needs to go away since we are create a Framework instance per test now
	framework.Init(t)

	// Used by the go-cassandra-native-protocol library
	configureZeroLog()
}

// e2eTestOpts configures an e2e test for execution.
type e2eTestOpts struct {
	// testFunc is the test function to be executed.
	testFunc e2eTestFunc

	// fixture specifies name of a directory containing test manifests to deploy. Only the
	// basename of the directory needs to be specified. It will be interpreted as a
	// subdirectory of the test/testdata/fixtures directory.
	fixture TestFixture

	// clusterScoped specifies whether the operator is configured to watch all namespaces.
	clusterScoped bool

	// deployTraefik specifies whether to deploy Traefik.
	deployTraefik bool

	// operatorNamespace is the namespace in which k8ssandra-operator is deployed. When the
	// operator is configured to only watch a single namespace, the test framework will
	// configure this to be the same as sutNamespace. When the operator is configured
	// to watch multiple namespaces, the test framework will configure this to be
	// k8ssandra-operator.
	operatorNamespace string

	// sutNamespace is the namespace in which the system under test (typically a
	// K8ssandraCluster) is deployed. When the operator is configured to watch a single
	// namespace, it will automatically set based on the fixture name. When the operator
	// is cluster-scoped, this needs to be explicitly set.
	sutNamespace string

	// additionalNamespaces provides an optional set of namespaces for use in tests where
	// the operator is cluster-scoped. For example, the K8ssandraCluster and each of its
	// CassandraDatacenters can be deployed in different namespaces. The K8ssandraCluster
	// namespace should be specified by sutNamespace and the CassandraDatacenter namespaces
	// should be specified here.
	additionalNamespaces []string

	// skipK8ssandraClusterCleanup is a flag that lets the framework know if deleting the
	// K8ssandraCluster should be skipped as would be the case for test that only involve
	// other components like Stargate and Reaper.
	skipK8ssandraClusterCleanup bool

	// doCassandraDatacenterCleanup is a flag that lets the framework know it should perform
	// deletions of CassandraDatacenters that are not part of a K8ssandraCluster.
	doCassandraDatacenterCleanup bool
}

// A TestFixture specifies the name of a subdirectory under the test/testdata/fixtures
// directory. It should consist of one or more yaml manifests, typically a manifest for a
// K8ssandraCluster.
type TestFixture string

type e2eTestFunc func(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework)

func e2eTest(ctx context.Context, opts *e2eTestOpts) func(*testing.T) {
	return func(t *testing.T) {
		f, err := framework.NewE2eFramework(*controlPlaneContext)
		if err != nil {
			t.Fatalf("failed to initialize test framework: %v", err)
		}

		setTestNamespaceNames(opts)

		fixtureDir, err := getTestFixtureDir(opts.fixture)
		if err != nil {
			t.Fatalf("failed to get fixture directory for %s: %v", opts.fixture, err)
		}

		err = beforeTest(t, f, fixtureDir, opts)
		defer afterTest(t, f, opts)

		if err == nil {
			opts.testFunc(t, ctx, opts.sutNamespace, f)
		} else {
			t.Errorf("before test setup failed: %v", err)
		}
	}
}

// setTestNamespaceNames initializes the operatorNamespace and sutNamespace fields. When
// the operator is cluster-scoped, it is always deployed in the k8ssandra-operator
// namespace. sutNamespace is not set because the K8ssadraCluster can be deployed in any
// namespace. When the operator is namespace-scoped both operatorNamespace and sutNamespace
// are set to the same value which is the fixture name plus a random suffix.
func setTestNamespaceNames(opts *e2eTestOpts) {
	if opts.clusterScoped {
		opts.operatorNamespace = "k8ssandra-operator"
	} else {
		opts.operatorNamespace = string(opts.fixture) + "-" + rand.String(6)
		opts.sutNamespace = opts.operatorNamespace
	}
}

func getTestFixtureDir(fixture TestFixture) (string, error) {
	path := filepath.Join("..", "testdata", "fixtures", string(fixture))
	return filepath.Abs(path)
}

// beforeTest Creates the test namespace, deploys k8ssandra-operator, and then deploys the
// test fixture. Deploying k8ssandra-operator includes cass-operator and all of the CRDs
// required by both operators.
func beforeTest(t *testing.T, f *framework.E2eFramework, fixtureDir string, opts *e2eTestOpts) error {
	namespaces := make([]string, 0)

	if opts.clusterScoped {
		namespaces = append(namespaces, opts.sutNamespace)
	}

	namespaces = append(namespaces, opts.operatorNamespace)

	if len(opts.additionalNamespaces) > 0 {
		namespaces = append(namespaces, opts.additionalNamespaces...)
	}

	for _, namespace := range namespaces {
		if err := f.CreateNamespace(namespace); err != nil {
			t.Logf("failed to create namespace %s", namespace)
			return err
		}

		if err := f.DeployCassandraConfigMap(namespace); err != nil {
			t.Log("failed to deploy cassandra configmap")
			return err
		}
	}

	if err := f.DeployCertManager(); err != nil {
		t.Log("failed to deploy cert-manager")
		return err
	}

	if err := f.WaitForCertManagerToBeReady("cert-manager", polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Log("failed waiting for cert-manager to be ready")
		return err
	}

	if err := f.DeployK8sContextsSecret(opts.operatorNamespace); err != nil {
		t.Logf("failed to deploy k8s contexts secret")
		return err
	}

	if err := f.DeployK8ssandraOperator(opts.operatorNamespace, opts.clusterScoped); err != nil {
		t.Logf("failed to deploy k8ssandra-operator")
		return err
	}

	if err := f.WaitForCrdsToBecomeActive(); err != nil {
		t.Log("failed waiting for CRDs to become active")
		return err
	}

	if err := f.DeployK8sClientConfigs(opts.operatorNamespace); err != nil {
		t.Logf("failed to deploy client configs to point to secret")
		return err
	}

	// Kill K8ssandraOperator pod to cause restart and load the client configs
	if err := f.DeleteK8ssandraOperatorPods(opts.operatorNamespace, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Logf("failed to restart k8ssandra-operator")
		return err
	}

	if err := f.WaitForCassOperatorToBeReady(opts.operatorNamespace, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Log("failed waiting for cass-operator to be ready")
		return err
	}

	if err := f.WaitForK8ssandraOperatorToBeReady(opts.operatorNamespace, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Log("failed waiting for k8ssandra-operator to be ready")
		return err
	}

	if opts.deployTraefik {
		if err := f.DeployTraefik(t, opts.operatorNamespace); err != nil {
			t.Logf("failed to deploy Traefik")
			return err
		}
	}

	fixtureDir, err := filepath.Abs(fixtureDir)
	if err != nil {
		return err
	}

	// Substitute values in Kustomize templates for tests and apply them
	fixtureFiles, err := os.ReadDir(fixtureDir)
	if err != nil {
		return err
	}

	versionSpecificDir := filepath.Join(fixtureDir, getCassandraFixtureDir())
	if _, err := os.Stat(versionSpecificDir); !os.IsNotExist(err) {
		additionalFixtureFiles, err := os.ReadDir(versionSpecificDir)
		if err != nil {
			return err
		}

		fixtureFiles = append(fixtureFiles, additionalFixtureFiles...)
	}

	for _, filu := range fixtureFiles {
		path := filepath.Join(fixtureDir, filu.Name())
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		output, err := runEnvSubst(strings.NewReader(string(b)), f)
		if err != nil {
			t.Log("failed to run envsubst")
			return err
		}

		if err := kubectl.Apply(kubectl.Options{Namespace: opts.sutNamespace, Context: f.ControlPlaneContext}, output); err != nil {
			t.Log("kubectl apply failed")
			return err
		}
	}

	return nil
}

func getCassandraFixtureDir() string {
	parts := strings.Split(*cassandraVersion, ".")
	return fmt.Sprintf("%s.%s.x", parts[0], parts[1])
}

func runEnvSubst(input *strings.Reader, f *framework.E2eFramework) (*bytes.Buffer, error) {
	// TODO Maybe text/template would make more sense? Pure go solution
	cmd := exec.Command("envsubst")
	cmd.Stdin = input
	var out bytes.Buffer
	cmd.Stdout = &out

	// Set environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("CASSANDRA_VERSION=%s", *cassandraVersion))
	cmd.Env = append(cmd.Env, fmt.Sprintf("CONTROL_PLANE_CONTEXT=%s", f.ControlPlaneContext))
	for i, dataContext := range f.GetDataPlaneContexts() {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DATA_PLANE_CONTEXT_%d=%s", i, dataContext))
	}

	err := cmd.Run()
	return &out, err
}

func applyPollingDefaults() {
	polling.operatorDeploymentReady.timeout = 1 * time.Minute
	polling.operatorDeploymentReady.interval = 1 * time.Second

	polling.datacenterReady.timeout = 15 * time.Minute
	polling.datacenterReady.interval = 15 * time.Second

	polling.nodetoolStatus.timeout = 2 * time.Minute
	polling.nodetoolStatus.interval = 5 * time.Second

	polling.k8ssandraClusterStatus.timeout = 1 * time.Minute
	polling.k8ssandraClusterStatus.interval = 3 * time.Second

	polling.stargateReady.timeout = 5 * time.Minute
	polling.stargateReady.interval = 5 * time.Second

	polling.reaperReady.timeout = 5 * time.Minute
	polling.reaperReady.interval = 5 * time.Second
}

func afterTest(t *testing.T, f *framework.E2eFramework, opts *e2eTestOpts) {
	assert.NoError(t, cleanUp(t, f, opts), "after test cleanup failed")
}

func cleanUp(t *testing.T, f *framework.E2eFramework, opts *e2eTestOpts) error {

	namespaces := make([]string, 0)
	namespaces = append(namespaces, opts.operatorNamespace)
	namespaces = append(namespaces, opts.sutNamespace)
	if len(opts.additionalNamespaces) > 0 {
		namespaces = append(namespaces, opts.additionalNamespaces...)
	}

	if err := f.DumpClusterInfo(t.Name(), namespaces...); err != nil {
		t.Logf("failed to dump cluster info: %v", err)
	}

	timeout := 3 * time.Minute
	interval := 10 * time.Second

	if !opts.skipK8ssandraClusterCleanup {
		if err := f.DeleteK8ssandraClusters(opts.sutNamespace, timeout, interval); err != nil {
			t.Logf("failed to delete K8sandraCluster: %v", err)
			return err
		}
	}

	if opts.doCassandraDatacenterCleanup {
		if err := f.DeleteCassandraDatacenters(opts.sutNamespace, timeout, interval); err != nil {
			t.Logf("failed to delete CassandraDatacenter: %v", err)
		}
	}

	if opts.deployTraefik {
		if err := f.UndeployTraefik(t, opts.operatorNamespace); err != nil {
			t.Logf("failed to undeploy Traefik: %v", err)
		}
	}

	for _, namespace := range namespaces {
		if err := f.DeleteNamespace(namespace, timeout, interval); err != nil {
			t.Logf("failed to delete namespace: %v", err)
		}
	}

	return nil
}

// createSingleDatacenterCluster creates a K8ssandraCluster with one CassandraDatacenter
// and one Stargate node that are deployed in the local cluster.
func createSingleDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("check k8ssandra cluster status updated for CassandraDatacenter")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, kcKey, k8ssandra)
		if err != nil {
			return false
		}

		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		if !found {
			return false
		}
		if kdcStatus.Cassandra == nil {
			return false
		}
		return cassandraDatacenterReady(kdcStatus.Cassandra)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	stargateKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	checkStargateReady(t, f, ctx, stargateKey)

	checkStargateK8cStatusReady(t, f, ctx, kcKey, dcKey)

	t.Log("check that if Stargate is deleted directly it gets re-created")
	stargate := &stargateapi.Stargate{}
	err = f.Client.Get(ctx, stargateKey.NamespacedName, stargate)
	require.NoError(err, "failed to get Stargate in namespace %s", namespace)
	err = f.Client.Delete(ctx, stargate)
	require.NoError(err, "failed to delete Stargate in namespace %s", namespace)
	checkStargateReady(t, f, ctx, stargateKey)

	t.Log("delete Stargate in k8ssandracluster CRD")
	err = f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)
	patch := client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	stargateTemplate := k8ssandra.Spec.Cassandra.Datacenters[0].Stargate
	k8ssandra.Spec.Cassandra.Datacenters[0].Stargate = nil
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(err, "failed to patch K8ssandraCluster in namespace %s", namespace)

	t.Log("check Stargate deleted")
	require.Eventually(func() bool {
		stargate := &stargateapi.Stargate{}
		err := f.Client.Get(ctx, stargateKey.NamespacedName, stargate)
		if err == nil || !errors.IsNotFound(err) {
			return false
		}
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		} else if kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]; !found {
			return false
		} else {
			return kdcStatus.Stargate == nil
		}
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval)

	t.Log("re-create Stargate in k8ssandracluster resource")
	err = f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)
	patch = client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	k8ssandra.Spec.Cassandra.Datacenters[0].Stargate = stargateTemplate.DeepCopy()
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(err, "failed to patch K8ssandraCluster in operatorNamespace %s", namespace)
	checkStargateReady(t, f, ctx, stargateKey)

	t.Log("retrieve database credentials")
	username, password := f.RetrieveDatabaseCredentials(t, ctx, namespace, "test")

	t.Logf("deploying Stargate ingress routes in %s", f.ControlPlaneContext)
	f.DeployStargateIngresses(t, f.ControlPlaneContext, 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, f.ControlPlaneContext, namespace)

	replication := map[string]int{"dc1": 1}
	testStargateApis(t, ctx, f.ControlPlaneContext, 0, username, password, replication)
}

// createStargateAndDatacenter creates a CassandraDatacenter with 3 nodes, one per rack. It also creates 1 or 3 Stargate
// nodes, one per rack, all deployed in the local cluster. Note that no K8ssandraCluster object is created.
func createStargateAndDatacenter(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	dcKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)

	stargateKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "s1"}}
	checkStargateReady(t, f, ctx, stargateKey)

	t.Log("retrieve database credentials")
	username, password := f.RetrieveDatabaseCredentials(t, ctx, namespace, "test")

	t.Logf("deploying Stargate ingress routes in %s", f.ControlPlaneContext)
	f.DeployStargateIngresses(t, f.ControlPlaneContext, 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, f.ControlPlaneContext, namespace)

	replication := map[string]int{"dc1": 3}
	testStargateApis(t, ctx, f.ControlPlaneContext, 0, username, password, replication)
}

// createMultiDatacenterCluster creates a K8ssandraCluster with two CassandraDatacenters,
// one running locally and the other running in a remote cluster.
func createMultiDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dc1Key := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dc1Key, f)

	t.Log("check k8ssandra cluster status")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		cassandraStatus := getCassandraDatacenterStatus(k8ssandra, dc1Key.Name)
		if cassandraStatus == nil {
			return false
		}
		return cassandraDatacenterReady(cassandraStatus)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	dc2Key := framework.ClusterKey{K8sContext: f.GetDataPlaneContexts()[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	checkDatacenterReady(t, ctx, dc2Key, f)

	t.Log("check k8ssandra cluster status")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		cassandraStatus := getCassandraDatacenterStatus(k8ssandra, dc1Key.Name)
		if cassandraStatus == nil {
			return false
		}
		if !cassandraDatacenterReady(cassandraStatus) {
			return false
		}

		cassandraStatus = getCassandraDatacenterStatus(k8ssandra, dc2Key.Name)
		if cassandraStatus == nil {
			return false
		}
		return cassandraDatacenterReady(cassandraStatus)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	t.Log("check that nodes in dc1 see nodes in dc2")
	opts := kubectl.Options{Namespace: namespace, Context: f.ControlPlaneContext}
	pod := "test-dc1-rack1-sts-0"
	count := 6
	err = f.WaitForNodeToolStatusUN(opts, pod, count, polling.nodetoolStatus.timeout, polling.nodetoolStatus.interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	opts.Context = f.GetDataPlaneContexts()[0]
	pod = "test-dc2-rack1-sts-0"
	err = f.WaitForNodeToolStatusUN(opts, pod, count, polling.nodetoolStatus.timeout, polling.nodetoolStatus.interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)
}

func checkStargateApisWithMultiDcCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dc1Key := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dc1Key, f)

	t.Log("check k8ssandra cluster status")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		cassandraStatus := getCassandraDatacenterStatus(k8ssandra, dc1Key.Name)
		if cassandraStatus == nil {
			return false
		}
		return cassandraDatacenterReady(cassandraStatus)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	dc2Key := framework.ClusterKey{K8sContext: f.GetDataPlaneContexts()[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	checkDatacenterReady(t, ctx, dc2Key, f)

	t.Log("check k8ssandra cluster status")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		cassandraStatus := getCassandraDatacenterStatus(k8ssandra, dc1Key.Name)
		if cassandraStatus == nil {
			return false
		}
		if !cassandraDatacenterReady(cassandraStatus) {
			return false
		}

		cassandraStatus = getCassandraDatacenterStatus(k8ssandra, dc2Key.Name)
		if cassandraStatus == nil {
			return false
		}
		return cassandraDatacenterReady(cassandraStatus)
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")

	stargateKey := framework.ClusterKey{K8sContext: f.ControlPlaneContext, NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	checkStargateReady(t, f, ctx, stargateKey)

	t.Log("check k8ssandra cluster status updated for Stargate test-dc1-stargate")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		kdcStatus, found := k8ssandra.Status.Datacenters[dc1Key.Name]
		if !found {
			return false
		}
		if kdcStatus.Cassandra == nil {
			return false
		}

		if !cassandraDatacenterReady(kdcStatus.Cassandra) {
			return false
		}

		if kdcStatus.Stargate == nil {
			return false
		}
		return kdcStatus.Stargate.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval)

	stargateKey = framework.ClusterKey{K8sContext: f.GetDataPlaneContexts()[0], NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-stargate"}}
	checkStargateReady(t, f, ctx, stargateKey)

	t.Log("check k8ssandra cluster status updated for Stargate test-dc2-stargate")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
		if err != nil {
			return false
		}

		kdcStatus, found := k8ssandra.Status.Datacenters[dc2Key.Name]
		if !found {
			return false
		}
		if kdcStatus.Cassandra == nil {
			return false
		}

		if !cassandraDatacenterReady(kdcStatus.Cassandra) {
			return false
		}

		if kdcStatus.Stargate == nil {
			return false
		}
		return kdcStatus.Stargate.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval)

	t.Log("check that nodes in dc1 see nodes in dc2")
	opts := kubectl.Options{Namespace: namespace, Context: f.ControlPlaneContext}
	pod := "test-dc1-rack1-sts-0"
	count := 4
	err = f.WaitForNodeToolStatusUN(opts, pod, count, polling.nodetoolStatus.timeout, polling.nodetoolStatus.interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	opts.Context = f.GetDataPlaneContexts()[0]
	pod = "test-dc2-rack1-sts-0"
	err = f.WaitForNodeToolStatusUN(opts, pod, count, polling.nodetoolStatus.timeout, polling.nodetoolStatus.interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("retrieve database credentials")
	username, password := f.RetrieveDatabaseCredentials(t, ctx, namespace, "test")

	t.Logf("deploying Stargate ingress routes in %s", f.ControlPlaneContext)
	f.DeployStargateIngresses(t, f.ControlPlaneContext, 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, f.ControlPlaneContext, namespace)

	t.Logf("deploying Stargate ingress routes in %s", f.GetDataPlaneContexts()[0])
	f.DeployStargateIngresses(t, f.GetDataPlaneContexts()[0], 1, namespace, "test-dc2-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, f.GetDataPlaneContexts()[0], namespace)

	replication := map[string]int{"dc1": 1, "dc2": 1}

	testStargateApis(t, ctx, f.ControlPlaneContext, 0, username, password, replication)
	testStargateApis(t, ctx, f.GetDataPlaneContexts()[0], 1, username, password, replication)
}

func checkDatacenterReady(t *testing.T, ctx context.Context, key framework.ClusterKey, f *framework.E2eFramework) {
	t.Logf("check that datacenter %s in cluster %s is ready", key.Name, key.K8sContext)
	withDatacenter := f.NewWithDatacenter(ctx, key)
	require.Eventually(t, withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), polling.datacenterReady.timeout, polling.datacenterReady.interval, fmt.Sprintf("timed out waiting for datacenter %s to become ready", key.Name))
}

func getCassandraDatacenterStatus(k8ssandra *api.K8ssandraCluster, dc string) *cassdcapi.CassandraDatacenterStatus {
	kdcStatus, found := k8ssandra.Status.Datacenters[dc]
	if !found {
		return nil
	}
	return kdcStatus.Cassandra
}

func cassandraDatacenterReady(status *cassdcapi.CassandraDatacenterStatus) bool {
	return status.GetConditionStatus(cassdcapi.DatacenterReady) == corev1.ConditionTrue &&
		status.CassandraOperatorProgress == cassdcapi.ProgressReady
}

func checkStargateReady(t *testing.T, f *framework.E2eFramework, ctx context.Context, stargateKey framework.ClusterKey) {
	t.Logf("check that Stargate %s in cluster %s is ready", stargateKey.Name, stargateKey.K8sContext)
	withStargate := f.NewWithStargate(ctx, stargateKey)
	require.Eventually(t, withStargate(func(stargate *stargateapi.Stargate) bool {
		return stargate.Status.IsReady()
	}), polling.stargateReady.timeout, polling.stargateReady.interval, "timed out waiting for Stargate %s to become ready", stargateKey.Name)
}

func checkStargateK8cStatusReady(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	kcKey types.NamespacedName,
	dcKey framework.ClusterKey,
) {
	t.Log("check k8ssandra cluster status updated for Stargate")
	assert.Eventually(t, func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		if err := f.Client.Get(ctx, kcKey, k8ssandra); err != nil {
			return false
		}
		kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]
		return found &&
			kdcStatus.Cassandra != nil &&
			cassandraDatacenterReady(kdcStatus.Cassandra) &&
			kdcStatus.Stargate != nil &&
			kdcStatus.Stargate.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval, "timed out waiting for K8ssandraCluster status to get updated")
}

func configureZeroLog() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: zerolog.TimeFormatUnix,
	})
}
