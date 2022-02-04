package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	"github.com/k8ssandra/k8ssandra-operator/pkg/stargate"

	"github.com/k8ssandra/k8ssandra-operator/test/kustomize"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
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
	polling struct {
		nodetoolStatus          pollingConfig
		datacenterReady         pollingConfig
		operatorDeploymentReady pollingConfig
		k8ssandraClusterStatus  pollingConfig
		stargateReady           pollingConfig
		reaperReady             pollingConfig
		medusaBackupDone        pollingConfig
		medusaRestoreDone       pollingConfig
		datacenterUpdating      pollingConfig
	}

	logKustomizeOutput = flag.Bool("logKustomizeOutput", false, "")
	logKubectlOutput   = flag.Bool("logKubectlOutput", false, "")
)

func TestOperator(t *testing.T) {
	beforeSuite(t)

	ctx := context.Background()

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
	t.Run("AddDcToCluster", e2eTest(ctx, &e2eTestOpts{
		testFunc: addDcToCluster,
		fixture:  "add-dc",
	}))
	t.Run("CreateMultiStargateAndDatacenter", e2eTest(ctx, &e2eTestOpts{
		testFunc:                     createStargateAndDatacenter,
		fixture:                      "multi-stargate",
		deployTraefik:                true,
		skipK8ssandraClusterCleanup:  true,
		doCassandraDatacenterCleanup: true,
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
	t.Run("CreateSingleMedusa", e2eTest(ctx, &e2eTestOpts{
		testFunc:                     createSingleMedusa,
		fixture:                      "single-dc-medusa",
		deployTraefik:                false,
		skipK8ssandraClusterCleanup:  false,
		doCassandraDatacenterCleanup: false,
	}))
	t.Run("CreateMultiMedusa", e2eTest(ctx, &e2eTestOpts{
		testFunc:                     createMultiMedusa,
		fixture:                      "multi-dc-medusa",
		deployTraefik:                false,
		skipK8ssandraClusterCleanup:  false,
		doCassandraDatacenterCleanup: false,
	}))
	t.Run("MultiDcAuthOnOff", e2eTest(ctx, &e2eTestOpts{
		testFunc:      multiDcAuthOnOff,
		fixture:       "multi-dc-auth",
		deployTraefik: true,
	}))
	t.Run("ConfigControllerRestarts", e2eTest(ctx, &e2eTestOpts{
		testFunc: controllerRestart,
	}))
	t.Run("SingleDcEncryptionWithStargate", e2eTest(ctx, &e2eTestOpts{
		testFunc:      createSingleDatacenterClusterWithEncryption,
		fixture:       "single-dc-encryption-stargate",
		deployTraefik: true,
	}))
	t.Run("SingleDcEncryptionWithReaper", e2eTest(ctx, &e2eTestOpts{
		testFunc:      createSingleReaperWithEncryption,
		fixture:       "single-dc-encryption-reaper",
		deployTraefik: true,
	}))
	t.Run("MultiDcEncryptionWithStargate", e2eTest(ctx, &e2eTestOpts{
		testFunc:      checkStargateApisWithMultiDcEncryptedCluster,
		fixture:       "multi-dc-encryption-stargate",
		deployTraefik: true,
	}))
	t.Run("MultiDcEncryptionWithReaper", e2eTest(ctx, &e2eTestOpts{
		testFunc:      createMultiReaperWithEncryption,
		fixture:       "multi-dc-encryption-reaper",
		deployTraefik: true,
	}))
}

func beforeSuite(t *testing.T) {

	applyPollingDefaults()

	if val, ok := os.LookupEnv("NODETOOL_STATUS_TIMEOUT"); ok {
		timeout, err := time.ParseDuration(val)
		require.NoError(t, err, fmt.Sprintf("failed to parse NODETOOL_STATUS_TIMEOUT value: %s", val))
		polling.nodetoolStatus.timeout = timeout
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
		f, err := framework.NewE2eFramework(t)
		if err != nil {
			t.Fatalf("failed to initialize test framework: %v", err)
		}

		setTestNamespaceNames(opts)

		fixtureDir := ""
		if opts.fixture != "" {
			fixtureDir, err = getTestFixtureDir(opts.fixture)
			if err != nil {
				t.Fatalf("failed to get fixture directory for %s: %v", opts.fixture, err)
			}
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
		if opts.fixture != "" {
			opts.operatorNamespace = string(opts.fixture) + "-" + rand.String(6)
		} else {
			opts.operatorNamespace = framework.CleanupForKubernetes(rand.String(9))
		}
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
	cassOperatorNS := opts.sutNamespace

	if opts.clusterScoped {
		namespaces = append(namespaces, opts.sutNamespace)
		cassOperatorNS = "cass-operator"
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

	if err := f.WaitForCassOperatorToBeReady(cassOperatorNS, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
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

	if fixtureDir != "" {
		fixtureDir, err := filepath.Abs(fixtureDir)
		if err != nil {
			return err
		}

		if err := kubectl.Apply(kubectl.Options{Namespace: opts.sutNamespace, Context: f.ControlPlaneContext}, fixtureDir); err != nil {
			t.Log("kubectl apply failed")
			return err
		}
	}

	return nil
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

	polling.medusaBackupDone.timeout = 2 * time.Minute
	polling.medusaBackupDone.interval = 5 * time.Second

	polling.medusaRestoreDone.timeout = 5 * time.Minute
	polling.medusaRestoreDone.interval = 15 * time.Second

	polling.datacenterUpdating.timeout = 1 * time.Minute
	polling.datacenterUpdating.interval = 1 * time.Second
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

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
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

	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
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
	username, password, err := f.RetrieveDatabaseCredentials(ctx, namespace, k8ssandra.Name)
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)

	replication := map[string]int{"dc1": 1}
	testStargateApis(t, ctx, "kind-k8ssandra-0", 0, username, password, replication)
}

// createSingleDatacenterCluster creates a K8ssandraCluster with one CassandraDatacenter
// and one Stargate node that are deployed in the local cluster.
func createSingleDatacenterClusterWithEncryption(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
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

	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	checkStargateReady(t, f, ctx, stargateKey)
	checkStargateK8cStatusReady(t, f, ctx, kcKey, dcKey)

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, namespace, k8ssandra.Name)
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)

	replication := map[string]int{"dc1": 1}
	testStargateApis(t, ctx, "kind-k8ssandra-0", 0, username, password, replication)
}

// createSingleDatacenterCluster creates a K8ssandraCluster with one CassandraDatacenter
// and one Reaper instance that are deployed in the local cluster with encryption on.
func createSingleDatacenterClusterReaperEncryption(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
	err := f.Client.Get(ctx, kcKey, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
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

	reaperKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-reaper"}}
	checkReaperReady(t, f, ctx, reaperKey)

	checkReaperK8cStatusReady(t, f, ctx, kcKey, dcKey)

	t.Log("check Reaper keyspace created")
	checkKeyspaceExists(t, f, ctx, "kind-k8ssandra-0", namespace, "test", "test-dc1-default-sts-0", "reaper_db")
}

// createStargateAndDatacenter creates a CassandraDatacenter with 3 nodes, one per rack. It also creates 1 or 3 Stargate
// nodes, one per rack, all deployed in the local cluster. Note that no K8ssandraCluster object is created.
func createStargateAndDatacenter(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)

	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "s1"}}
	checkStargateReady(t, f, ctx, stargateKey)

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, namespace, "test")
	require.NoError(t, err, "failed to retrieve database credentials")

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)

	replication := map[string]int{"dc1": 3}
	testStargateApis(t, ctx, "kind-k8ssandra-0", 0, username, password, replication)
}

// createMultiDatacenterCluster creates a K8ssandraCluster with two CassandraDatacenters,
// one running locally and the other running in a remote cluster.
func createMultiDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
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

	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
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

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, namespace, k8ssandra.Name)
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("check that nodes in dc1 see nodes in dc2")
	pod := "test-dc1-rack1-sts-0"
	count := 6
	checkNodeToolStatusUN(t, f, "kind-k8ssandra-0", namespace, pod, count, "-u", username, "-pw", password)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	pod = "test-dc2-rack1-sts-0"
	checkNodeToolStatusUN(t, f, "kind-k8ssandra-1", namespace, pod, count, "-u", username, "-pw", password)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)
}

func addDcToCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	assert := assert.New(t)

	t.Log("check that the K8ssandraCluster was created")
	kcKey := client.ObjectKey{Namespace: namespace, Name: "test"}
	kc := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	k8sCtx0 := "kind-k8ssandra-0"
	k8sCtx1 := "kind-k8ssandra-1"

	dc1Key := framework.ClusterKey{
		K8sContext: k8sCtx0,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      "dc1",
		},
	}
	checkDatacenterReady(t, ctx, dc1Key, f)

	sg1Key := framework.ClusterKey{
		K8sContext: k8sCtx0,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      "test-dc1-stargate",
		},
	}
	checkStargateReady(t, f, ctx, sg1Key)

	reaper1Key := framework.ClusterKey{
		K8sContext: k8sCtx0,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      "test-dc1-reaper",
		},
	}
	checkReaperReady(t, f, ctx, reaper1Key)

	t.Log("create keyspaces")
	_, err = f.ExecuteCql(ctx, k8sCtx0, namespace, "test", "test-dc1-default-sts-0",
		"CREATE KEYSPACE ks1 WITH REPLICATION = {'class' : 'NetworkTopologyStrategy', 'dc1' : 1}")
	require.NoError(err, "failed to create keyspace")

	_, err = f.ExecuteCql(ctx, k8sCtx0, namespace, "test", "test-dc1-default-sts-0",
		"CREATE KEYSPACE ks2 WITH REPLICATION = {'class' : 'NetworkTopologyStrategy', 'dc1' : 1}")
	require.NoError(err, "failed to create keyspace")

	t.Log("add dc2 to cluster")
	err = f.Client.Get(ctx, kcKey, kc)
	require.NoError(err, "failed to get K8ssandraCluster %s", kcKey)

	kc.Spec.Cassandra.Datacenters = append(kc.Spec.Cassandra.Datacenters, api.CassandraDatacenterTemplate{
		Meta: api.EmbeddedObjectMeta{
			Name: "dc2",
		},
		K8sContext: k8sCtx1,
		Size:       1,
	})
	annotations.AddAnnotation(kc, api.DcReplicationAnnotation, `{"dc2": {"ks1": 1, "ks2": 1}}`)

	err = f.Client.Update(ctx, kc)
	require.NoError(err, "failed to update K8ssandraCluster")

	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	checkDatacenterReady(t, ctx, dc2Key, f)

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, namespace, kc.Name)
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("check that nodes in dc1 see nodes in dc2")
	pod := "test-dc1-default-sts-0"
	count := 2
	checkNodeToolStatusUN(t, f, "kind-k8ssandra-0", namespace, pod, count, "-u", username, "-pw", password)

	assert.NoError(err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	pod = "test-dc2-default-sts-0"
	checkNodeToolStatusUN(t, f, "kind-k8ssandra-1", namespace, pod, count, "-u", username, "-pw", password)

	assert.NoError(err, "timed out waiting for nodetool status check against "+pod)

	keyspaces := []string{"system_auth", stargate.AuthKeyspace, reaperapi.DefaultKeyspace, "ks1", "ks2"}
	for _, ks := range keyspaces {
		assert.Eventually(func() bool {
			output, err := f.ExecuteCql(ctx, k8sCtx0, namespace, "test", "test-dc1-default-sts-0",
				fmt.Sprintf("SELECT replication FROM system_schema.keyspaces WHERE keyspace_name = '%s'", ks))
			if err != nil {
				t.Logf("replication check for keyspace %s failed: %v", ks, err)
				return false
			}
			return strings.Contains(output, "'dc1': '1'") && strings.Contains(output, "'dc2': '1'")
		}, 1*time.Minute, 5*time.Second, "failed to verify replication updated for keyspace %s", ks)
	}

	sg2Key := framework.ClusterKey{
		K8sContext: k8sCtx1,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      "test-dc2-stargate",
		},
	}
	checkStargateReady(t, f, ctx, sg2Key)

	reaper2Key := framework.ClusterKey{
		K8sContext: k8sCtx1,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      "test-dc2-reaper",
		},
	}
	checkReaperReady(t, f, ctx, reaper2Key)
}

func checkStargateApisWithMultiDcCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
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

	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
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

	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
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

	stargateKey = framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-stargate"}}
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

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, namespace, k8ssandra.Name)
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("check that nodes in dc1 see nodes in dc2")
	pod := "test-dc1-rack1-sts-0"
	count := 4
	checkNodeToolStatusUN(t, f, "kind-k8ssandra-0", namespace, pod, count, "-u", username, "-pw", password)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	pod = "test-dc2-rack1-sts-0"
	checkNodeToolStatusUN(t, f, "kind-k8ssandra-1", namespace, pod, count, "-u", username, "-pw", password)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-1")
	f.DeployStargateIngresses(t, "kind-k8ssandra-1", 1, namespace, "test-dc2-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-1", namespace)

	replication := map[string]int{"dc1": 1, "dc2": 1}

	testStargateApis(t, ctx, "kind-k8ssandra-0", 0, username, password, replication)
	testStargateApis(t, ctx, "kind-k8ssandra-1", 1, username, password, replication)
}

func checkStargateApisWithMultiDcEncryptedCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)
	require.NoError(f.CreateCassandraEncryptionStoresSecret(namespace), "Failed to create the encryption secrets")

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
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

	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
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

	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
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

	stargateKey = framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-stargate"}}
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

	t.Log("retrieve database credentials")
	username, password, err := f.RetrieveDatabaseCredentials(ctx, namespace, k8ssandra.Name)
	require.NoError(err, "failed to retrieve database credentials")

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-0", namespace)

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-1")
	f.DeployStargateIngresses(t, "kind-k8ssandra-1", 1, namespace, "test-dc2-stargate-service", username, password)
	defer f.UndeployAllIngresses(t, "kind-k8ssandra-1", namespace)

	replication := map[string]int{"dc1": 1, "dc2": 1}

	testStargateApis(t, ctx, "kind-k8ssandra-0", 0, username, password, replication)
	testStargateApis(t, ctx, "kind-k8ssandra-1", 1, username, password, replication)
}

func checkDatacenterReady(t *testing.T, ctx context.Context, key framework.ClusterKey, f *framework.E2eFramework) {
	t.Logf("check that datacenter %s in cluster %s is ready", key.Name, key.K8sContext)
	withDatacenter := f.NewWithDatacenter(ctx, key)
	require.Eventually(t, withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), polling.datacenterReady.timeout, polling.datacenterReady.interval, fmt.Sprintf("timed out waiting for datacenter %s to become ready", key.Name))
}

func checkDatacenterUpdating(t *testing.T, ctx context.Context, key framework.ClusterKey, f *framework.E2eFramework) {
	t.Logf("check that datacenter %s in cluster %s is updating", key.Name, key.K8sContext)
	withDatacenter := f.NewWithDatacenter(ctx, key)
	require.Eventually(t, withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterUpdating)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressUpdating
	}), polling.datacenterUpdating.timeout, polling.datacenterUpdating.interval, fmt.Sprintf("timed out waiting for datacenter %s to become updating", key.Name))
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

// checkNodeToolStatusUN polls until nodetool status reports UN for count nodes.
func checkNodeToolStatusUN(
	t *testing.T,
	f *framework.E2eFramework,
	k8sContext, namespace, pod string,
	count int,
	additionalArgs ...string,
) {
	require.Eventually(
		t,
		func() bool {
			actual, err := f.GetNodeToolStatusUN(k8sContext, namespace, pod, additionalArgs...)
			return err == nil && actual == count
		},
		polling.nodetoolStatus.timeout,
		polling.nodetoolStatus.interval,
		"timed out waiting for nodetool status to reach count %v",
		count,
	)
}

func checkSecretExists(t *testing.T, f *framework.E2eFramework, ctx context.Context, kcKey client.ObjectKey, secretKey framework.ClusterKey) {
	secret := getSecret(t, f, ctx, secretKey)
	require.True(t, labels.IsManagedBy(secret, kcKey), "secret %s/%s is not managed by %s", secretKey.Namespace, secretKey.Name, kcKey.Name)
}

// FIXME there is some overlap with E2eFramework.RetrieveDatabaseCredentials()
func retrieveCredentials(t *testing.T, f *framework.E2eFramework, ctx context.Context, secretKey framework.ClusterKey) (string, string) {
	secret := getSecret(t, f, ctx, secretKey)
	return string(secret.Data["username"]), string(secret.Data["password"])
}

func getSecret(t *testing.T, f *framework.E2eFramework, ctx context.Context, secretKey framework.ClusterKey) *corev1.Secret {
	secret := &corev1.Secret{}
	require.Eventually(
		t,
		func() bool {
			return f.Get(ctx, secretKey, secret) == nil
		},
		time.Minute,
		time.Second,
		"secret %s does not exist",
		secretKey,
	)
	return secret
}

func checkSecretDoesNotExist(t *testing.T, f *framework.E2eFramework, ctx context.Context, secretKey framework.ClusterKey) {
	secret := &corev1.Secret{}
	require.Never(
		t,
		func() bool {
			return f.Get(ctx, secretKey, secret) == nil
		},
		15*time.Second,
		time.Second,
		"secret %s exists",
		secretKey,
	)
}

func checkKeyspaceExists(
	t *testing.T,
	f *framework.E2eFramework,
	ctx context.Context,
	k8sContext, namespace, clusterName, pod, keyspace string,
) {
	assert.Eventually(t, func() bool {
		keyspaces, err := f.ExecuteCql(ctx, k8sContext, namespace, clusterName, pod, "describe keyspaces")
		if err != nil {
			t.Logf("failed to desctibe keyspaces: %v", err)
			return false
		}
		return strings.Contains(keyspaces, keyspace)
	}, 1*time.Minute, 3*time.Second)
}

func configureZeroLog() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: zerolog.TimeFormatUnix,
	})
}
