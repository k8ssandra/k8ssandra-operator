package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	}

	logKustomizeOutput = flag.Bool("logKustomizeOutput", false, "")
	logKubectlOutput   = flag.Bool("logKubectlOutput", false, "")
)

func TestOperator(t *testing.T) {
	beforeSuite(t)

	ctx := context.Background()

	applyPollingDefaults()

	t.Run("CreateSingleDatacenterCluster", e2eTest(ctx, "single-dc", true, createSingleDatacenterCluster))
	t.Run("CreateStargateAndDatacenter", e2eTest(ctx, "stargate", true, createStargateAndDatacenter))
	// TODO enable after https://github.com/k8ssandra/k8ssandra-operator/issues/156 is fixed
	// t.Run("CreateMultiStargateAndDatacenter", e2eTest(ctx, "multi-stargate", true, createStargateAndDatacenter))
	t.Run("CreateMultiDatacenterCluster", e2eTest(ctx, "multi-dc", false, createMultiDatacenterCluster))
	t.Run("CheckStargateApisWithMultiDcCluster", e2eTest(ctx, "multi-dc-stargate", true, checkStargateApisWithMultiDcCluster))
}

func beforeSuite(t *testing.T) {
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

// A TestFixture specifies the name of a subdirectory under the test/testdata/fixtures
// directory. It should consist of one or more yaml manifests.
type TestFixture string

type e2eTestFunc func(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework)

func e2eTest(ctx context.Context, fixture TestFixture, deployTraefik bool, test e2eTestFunc) func(*testing.T) {
	return func(t *testing.T) {
		f, err := framework.NewE2eFramework()
		if err != nil {
			t.Fatalf("failed to initialize test framework: %v", err)
		}

		namespace := getTestNamespace(fixture)
		fixtureDir, err := getTestFixtureDir(fixture)

		if err != nil {
			t.Fatalf("failed to get fixture directory for %s: %v", fixture, err)
		}

		err = beforeTest(t, namespace, fixtureDir, f, deployTraefik)
		defer afterTest(t, namespace, f, deployTraefik)

		if err == nil {
			test(t, ctx, namespace, f)
		} else {
			t.Errorf("before test setup failed: %v", err)
		}
	}
}

func getTestNamespace(fixture TestFixture) string {
	return string(fixture) + "-" + rand.String(6)
}

func getTestFixtureDir(fixture TestFixture) (string, error) {
	path := filepath.Join("..", "testdata", "fixtures", string(fixture))
	return filepath.Abs(path)
}

// beforeTest Creates the test namespace, deploys k8ssandra-operator, and then deploys the
// test fixture. Deploying k8ssandra-operator includes cass-operator and all of the CRDs
// required by both operators.
func beforeTest(t *testing.T, namespace, fixtureDir string, f *framework.E2eFramework, deployTraefik bool) error {
	if err := f.CreateNamespace(namespace); err != nil {
		t.Log("failed to create namespace")
		return err
	}

	if err := f.DeployCertManager(); err != nil {
		t.Log("failed to deploy cert-manager")
		return err
	}

	if err := f.WaitForCertManagerToBeReady("cert-manager", polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Log("failed waiting for cert-manager to be ready")
		return err
	}

	if err := f.DeployCassOperator(namespace); err != nil {
		t.Log("failed to deploy cass-operator")
		return err
	}

	if err := f.DeployCassandraConfigMap(namespace); err != nil {
		t.Log("failed to deploy cassandra configmap")
		return err
	}

	if err := f.DeployK8sContextsSecret(namespace); err != nil {
		t.Logf("failed to deploy k8s contexts secret")
		return err
	}

	if err := f.DeployK8ssandraOperator(namespace); err != nil {
		t.Logf("failed to deploy k8ssandra-operator")
		return err
	}

	if err := f.WaitForCrdsToBecomeActive(); err != nil {
		t.Log("failed waiting for CRDs to become active")
		return err
	}

	if err := f.DeployK8sClientConfigs(namespace); err != nil {
		t.Logf("failed to deploy client configs to point to secret")
		return err
	}

	// Kill K8ssandraOperator pod to cause restart and load the client configs
	if err := f.DeleteK8ssandraOperatorPods(namespace, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Logf("failed to restart k8ssandra-operator")
		return err
	}

	if err := f.WaitForCassOperatorToBeReady(namespace, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Log("failed waiting for cass-operator to be ready")
		return err
	}

	if err := f.WaitForK8ssandraOperatorToBeReady(namespace, polling.operatorDeploymentReady.timeout, polling.operatorDeploymentReady.interval); err != nil {
		t.Log("failed waiting for k8ssandra-operator to be ready")
		return err
	}

	if deployTraefik {
		if err := f.DeployTraefik(t, namespace); err != nil {
			t.Logf("failed to deploy Traefik")
			return err
		}
	}

	fixtureDir, err := filepath.Abs(fixtureDir)
	if err != nil {
		return err
	}

	if err := kubectl.Apply(kubectl.Options{Namespace: namespace, Context: f.ControlPlaneContext}, fixtureDir); err != nil {
		t.Log("kubectl apply failed")
		return err
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
}

func afterTest(t *testing.T, namespace string, f *framework.E2eFramework, deployTraefik bool) {
	assert.NoError(t, cleanUp(t, namespace, f, deployTraefik), "after test cleanup failed")
}

func cleanUp(t *testing.T, namespace string, f *framework.E2eFramework, deployTraefik bool) error {

	if t.Failed() {
		f.DumpOperatorLogs(namespace)
		f.DumpCassandraLogs(namespace)
		f.DumpStargateLogs(namespace)
	}

	if err := f.DumpClusterInfo(t.Name(), namespace); err != nil {
		t.Logf("failed to dump cluster info: %v", err)
	}

	if err := f.DeleteK8ssandraClusters(namespace); err != nil {
		return err
	}

	if deployTraefik {
		if err := f.UndeployTraefik(t, namespace); err != nil {
			return err
		}
	}

	timeout := 3 * time.Minute
	interval := 10 * time.Second

	if err := f.DeleteStargates(namespace, timeout, interval); err != nil {
		return err
	}

	if err := f.DeleteDatacenters(namespace, timeout, interval); err != nil {
		return err
	}

	if err := f.DeleteReplicatedSecrets(namespace, timeout, interval); err != nil {
		return err
	}

	if err := f.DeleteNamespace(namespace, timeout, interval); err != nil {
		return err
	}

	return nil
}

// createSingleDatacenterCluster creates a K8ssandraCluster with one CassandraDatacenter
// and one Stargate node that are deployed in the local cluster.
func createSingleDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("check k8ssandra cluster status updated for CassandraDatacenter")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
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

	t.Log("check that Stargate test-dc1-stargate is ready")
	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	withStargate := f.NewWithStargate(ctx, stargateKey)
	require.Eventually(withStargate(func(stargate *stargateapi.Stargate) bool {
		return stargate.Status.IsReady()
	}), polling.stargateReady.timeout, polling.stargateReady.interval, "timed out waiting for Stargate test-dc1-stargate to become ready")

	t.Log("check k8ssandra cluster status updated for Stargate")
	require.Eventually(func() bool {
		k8ssandra := &api.K8ssandraCluster{}
		err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
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

		if !cassandraDatacenterReady(kdcStatus.Cassandra) {
			return false
		}

		if kdcStatus.Stargate == nil {
			return false
		}
		return kdcStatus.Stargate.IsReady()
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval)

	t.Log("check that if Stargate is deleted directly it gets re-created")
	stargate := &stargateapi.Stargate{}
	err = f.Client.Get(ctx, stargateKey.NamespacedName, stargate)
	require.NoError(err, "failed to get Stargate in namespace %s", namespace)
	err = f.Client.Delete(ctx, stargate)
	require.NoError(err, "failed to delete Stargate in namespace %s", namespace)
	require.Eventually(withStargate(func(stargate *stargateapi.Stargate) bool {
		return stargate.Status.IsReady()
	}), polling.stargateReady.timeout, polling.stargateReady.interval, "timed out waiting for Stargate test-dc1-stargate to become ready")

	t.Log("delete Stargate in k8ssandracluster CRD")
	err = f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
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
		if err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra); err != nil {
			return false
		} else if kdcStatus, found := k8ssandra.Status.Datacenters[dcKey.Name]; !found {
			return false
		} else {
			return kdcStatus.Stargate == nil
		}
	}, polling.k8ssandraClusterStatus.timeout, polling.k8ssandraClusterStatus.interval)

	t.Log("re-create Stargate in k8ssandracluster resource")
	err = f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)
	patch = client.MergeFromWithOptions(k8ssandra.DeepCopy(), client.MergeFromWithOptimisticLock{})
	k8ssandra.Spec.Cassandra.Datacenters[0].Stargate = stargateTemplate.DeepCopy()
	err = f.Client.Patch(ctx, k8ssandra, patch)
	require.NoError(err, "failed to patch K8ssandraCluster in namespace %s", namespace)

	t.Log("check that Stargate test-dc1-stargate is ready")
	require.Eventually(withStargate(func(stargate *stargateapi.Stargate) bool {
		return stargate.Status.IsReady()
	}), polling.stargateReady.timeout, polling.stargateReady.interval, "timed out waiting for Stargate test-dc1-stargate to become ready")

	t.Log("retrieve database credentials")
	username, password := retrieveDatabaseCredentials(t, f, ctx, namespace, "test")

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployStargateIngresses(t, "kind-k8ssandra-0", namespace)

	replication := map[string]int{"dc1": 1}
	testStargateApis(t, ctx, "kind-k8ssandra-0", 0, username, password, replication)
}

// createStargateAndDatacenter creates a CassandraDatacenter with 3 nodes, one per rack. It also creates 1 or 3 Stargate
// nodes, one per rack, all deployed in the local cluster. Note that no K8ssandraCluster object is created.
func createStargateAndDatacenter(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	checkDatacenterReady(t, ctx, dcKey, f)

	t.Log("check that Stargate s1 is ready")
	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "s1"}}
	withStargate := f.NewWithStargate(ctx, stargateKey)
	require.Eventually(withStargate(func(stargate *stargateapi.Stargate) bool {
		return stargate.Status.IsReady()
	}), polling.stargateReady.timeout, polling.stargateReady.interval, "timed out waiting for Stargate s1 to become ready")

	t.Log("retrieve database credentials")
	username, password := retrieveDatabaseCredentials(t, f, ctx, namespace, "test")

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployStargateIngresses(t, "kind-k8ssandra-0", namespace)

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

	t.Log("check that nodes in dc1 see nodes in dc2")
	opts := kubectl.Options{Namespace: namespace, Context: "kind-k8ssandra-0"}
	pod := "test-dc1-rack1-sts-0"
	count := 6
	err = f.WaitForNodeToolStatusUN(opts, pod, count, polling.nodetoolStatus.timeout, polling.nodetoolStatus.interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	opts.Context = "kind-k8ssandra-1"
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

	t.Log("check that Stargate test-dc1-stargate is ready")
	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	withStargate := f.NewWithStargate(ctx, stargateKey)
	require.Eventually(withStargate(func(stargate *stargateapi.Stargate) bool {
		return stargate.Status.IsReady()
	}), polling.stargateReady.timeout, polling.stargateReady.interval, "timed out waiting for Stargate test-dc1-stargate to become ready")

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

	t.Log("check that Stargate test-dc2-stargate is ready")
	stargateKey = framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc2-stargate"}}
	withStargate = f.NewWithStargate(ctx, stargateKey)
	require.Eventually(withStargate(func(stargate *stargateapi.Stargate) bool {
		return stargate.Status.IsReady()
	}), polling.stargateReady.timeout, polling.stargateReady.interval, "timed out waiting for Stargate test-dc2-stargate to become ready")

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
	opts := kubectl.Options{Namespace: namespace, Context: "kind-k8ssandra-0"}
	pod := "test-dc1-rack1-sts-0"
	count := 4
	err = f.WaitForNodeToolStatusUN(opts, pod, count, polling.nodetoolStatus.timeout, polling.nodetoolStatus.interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	opts.Context = "kind-k8ssandra-1"
	pod = "test-dc2-rack1-sts-0"
	err = f.WaitForNodeToolStatusUN(opts, pod, count, polling.nodetoolStatus.timeout, polling.nodetoolStatus.interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("retrieve database credentials")
	username, password := retrieveDatabaseCredentials(t, f, ctx, namespace, "test")

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-0")
	f.DeployStargateIngresses(t, "kind-k8ssandra-0", 0, namespace, "test-dc1-stargate-service", username, password)
	defer f.UndeployStargateIngresses(t, "kind-k8ssandra-0", namespace)

	t.Log("deploying Stargate ingress routes in kind-k8ssandra-1")
	f.DeployStargateIngresses(t, "kind-k8ssandra-1", 1, namespace, "test-dc2-stargate-service", username, password)
	defer f.UndeployStargateIngresses(t, "kind-k8ssandra-1", namespace)

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

func configureZeroLog() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: zerolog.TimeFormatUnix,
	})
}
