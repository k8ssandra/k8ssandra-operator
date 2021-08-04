package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
)

var (
	nodetoolStatusTimeout time.Duration
)

func TestOperator(t *testing.T) {
	beforeSuite(t)

	ctx := context.Background()

	t.Run("CreateSingleDatacenterCluster", e2eTest(ctx, "single-dc", createSingleDatacenterCluster))
	t.Run("CreateStandAloneStargate", e2eTest(ctx, "stargate", createStargateAndDatacenter))
	t.Run("CreateMultiDatacenterCluster", e2eTest(ctx, "multi-dc", createMultiDatacenterCluster))
}

func beforeSuite(t *testing.T) {
	if val, ok := os.LookupEnv("NODETOOL_STATUS_TIMEOUT"); ok {
		timeout, err := time.ParseDuration(val)
		require.NoError(t, err, fmt.Sprintf("failed to parse NODETOOL_STATUS_TIMEOUT value: %s", val))
		nodetoolStatusTimeout = timeout
	} else {
		nodetoolStatusTimeout = 1 * time.Minute
	}

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
}

// A TestFixture specifies the name of a subdirectory under the test/testdata/fixtures
// directory. It should consist of one or more yaml manifests.
type TestFixture string

type e2eTestFunc func(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework)

func e2eTest(ctx context.Context, fixture TestFixture, test e2eTestFunc) func(*testing.T) {
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

		err = beforeTest(t, namespace, fixtureDir, f)
		defer afterTest(t, namespace, f)

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
func beforeTest(t *testing.T, namespace, fixtureDir string, f *framework.E2eFramework) error {
	if err := f.CreateNamespace(namespace); err != nil {
		t.Log("failed to create namespace")
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

	timeout := 1 * time.Minute
	interval := 1 * time.Second

	// Kill K8ssandraOperator pod to cause restart and load the client configs
	if err := f.DeleteK8ssandraOperatorPods(namespace, timeout, interval); err != nil {
		t.Logf("failed to restart k8ssandra-operator")
		return err
	}

	if err := f.WaitForCassOperatorToBeReady(namespace, timeout, interval); err != nil {
		t.Log("failed waiting for cass-operator to be ready")
		return err
	}

	if err := f.WaitForK8ssandraOperatorToBeReady(namespace, timeout, interval); err != nil {
		t.Log("failed waiting for k8ssandra-operator to be ready")
		return err
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

func afterTest(t *testing.T, namespace string, f *framework.E2eFramework) {
	assert.NoError(t, cleanUp(t, namespace, f), "after test cleanup failed")
}

func cleanUp(t *testing.T, namespace string, f *framework.E2eFramework) error {
	if err := f.DumpClusterInfo(t.Name(), namespace); err != nil {
		t.Logf("failed to dump cluster info: %v", err)
	}

	if err := f.DeleteK8ssandraClusters(namespace); err != nil {
		return err
	}

	timeout := 3 * time.Minute
	interval := 10 * time.Second

	if err := f.DeleteStargates(namespace, timeout, interval); err != nil {
		return err
	}

	if err := f.DeleteDatacenters(namespace, timeout, interval); err != nil {
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

	t.Log("check that datacenter dc1 is ready")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	withDatacenter := f.NewWithDatacenter(ctx, dcKey)

	timeout := 8 * time.Minute
	interval := 15 * time.Second

	require.Eventually(withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), timeout, interval, "timed out waiting for datacenter to become ready")

	t.Log("check k8ssandra cluster status")
	timeout = 1 * time.Minute
	interval = 5 * time.Second
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
		return cassandraDatacenteReady(kdcStatus.Cassandra)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status to get updated")

	t.Log("check that Stargate test-dc1-stargate is ready")
	timeout = 4 * time.Minute
	interval = 15 * time.Second
	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	withStargate := f.NewWithStargate(ctx, stargateKey)
	require.Eventually(withStargate(func(stargate *api.Stargate) bool {
		return stargate.Status.ReadyReplicas == 1
	}), timeout, interval, "timed out waiting for Stargate test-dc1-stargate to become ready")
}

// createStargateAndDatacenter creates a CassandraDatacenter and a Stargate node both of
// which are deployed in the local cluster. Note that no K8ssandraCluster object is created.
func createStargateAndDatacenter(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	timeout := 8 * time.Minute
	interval := 15 * time.Second

	t.Log("check that CassandraDatacenter dc1 is ready")
	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	withDatacenter := f.NewWithDatacenter(ctx, dcKey)
	require.Eventually(withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), timeout, interval, "timed out waiting for datacenter dc1 to become ready")

	t.Log("check that Stargate s1 is ready")
	timeout = 3 * time.Minute
	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "s1"}}
	withStargate := f.NewWithStargate(ctx, stargateKey)
	require.Eventually(withStargate(func(stargate *api.Stargate) bool {
		return stargate.Status.ReadyReplicas == 1
	}), timeout, interval, "timed out waiting for Stargate s1 to become ready")
}

// createMultiDatacenterCluster creates a K8ssandraCluster with two CassandraDatacenters,
// one running locally and the other running in a remote cluster. There is a Stargate node
// per DC.
func createMultiDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	timeout := 8 * time.Minute
	interval := 15 * time.Second

	t.Log("check that datacenter dc1 is ready")
	dc1Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	withDatacenter := f.NewWithDatacenter(ctx, dc1Key)
	require.Eventually(withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), timeout, interval, "timed out waiting for datacenter dc1 to become ready")

	t.Log("check k8ssandra cluster status")
	timeout = 1 * time.Minute
	interval = 5 * time.Second
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
		return cassandraDatacenteReady(cassandraStatus)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status to get updated")

	t.Log("check that Stargate test-dc1-stargate is ready")
	timeout = 4 * time.Minute
	interval = 15 * time.Second
	stargateKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-0", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "test-dc1-stargate"}}
	withStargate := f.NewWithStargate(ctx, stargateKey)
	require.Eventually(withStargate(func(stargate *api.Stargate) bool {
		return stargate.Status.ReadyReplicas == 1
	}), timeout, interval, "timed out waiting for Stargate test-dc1-stargate to become ready")

	t.Log("check that datacenter dc2 is ready")
	timeout = 8 * time.Minute
	interval = 15 * time.Second
	dc2Key := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc2"}}
	withDatacenter = f.NewWithDatacenter(ctx, dc2Key)
	require.Eventually(withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), timeout, interval, "timed out waiting for datacenter dc2 to become ready")

	t.Log("check k8ssandra cluster status")
	// We use a larger timeout for this status check because the additional seeds will be
	// updated for dc1 and dc1's status will be set to updating when cass-operator creates
	// the endpoints. cass-operator will update the status back to ready when it reaches the
	// end of its reconciliation.
	timeout = 3 * time.Minute
	interval = 5 * time.Second
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
		if !cassandraDatacenteReady(cassandraStatus) {
			return false
		}

		cassandraStatus = getCassandraDatacenterStatus(k8ssandra, dc2Key.Name)
		if cassandraStatus == nil {
			return false
		}
		return cassandraDatacenteReady(cassandraStatus)
	}, timeout, interval, "timed out waiting for K8ssandraCluster status to get updated")

	t.Log("check that nodes in dc1 see nodes in dc2")
	interval = 5 * time.Second
	opts := kubectl.Options{Namespace: namespace, Context: "kind-k8ssandra-0"}
	pod := "test-dc1-default-sts-0"
	count := 6
	err = f.WaitForNodeToolStatusUN(opts, pod, count, nodetoolStatusTimeout, interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)

	t.Log("check nodes in dc2 see nodes in dc1")
	opts.Context = "kind-k8ssandra-1"
	pod = "test-dc2-default-sts-0"
	err = f.WaitForNodeToolStatusUN(opts, pod, count, nodetoolStatusTimeout, interval)

	assert.NoError(t, err, "timed out waiting for nodetool status check against "+pod)
}

func getCassandraDatacenterStatus(k8ssandra *api.K8ssandraCluster, dc string) *cassdcapi.CassandraDatacenterStatus {
	kdcStatus, found := k8ssandra.Status.Datacenters[dc]
	if !found {
		return nil
	}
	return kdcStatus.Cassandra
}

func cassandraDatacenteReady(status *cassdcapi.CassandraDatacenterStatus) bool {
	return status.GetConditionStatus(cassdcapi.DatacenterReady) == corev1.ConditionTrue &&
		status.CassandraOperatorProgress == cassdcapi.ProgressReady
}
