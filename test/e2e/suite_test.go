package e2e

import (
	"context"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"path/filepath"
	"testing"
	"time"
)

func TestOperator(t *testing.T) {
	beforeSuite(t)

	ctx := context.Background()

	t.Run("CreateSingleDatacenterCluster", e2eTest(ctx, "single-dc", createSingleDatacenterCluster))
}

func beforeSuite(t *testing.T) {
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

	buf, err := ioutil.ReadFile(inClusterCfgFile)
	if err != nil {
		t.Fatalf("failed to read %s: %v", inClusterCfgFile, err)
	}

	dest, err := filepath.Abs("../testdata/k8s-contexts/kubeconfig")
	if err != nil {
		t.Fatalf("failed to get path of dest kind kubeconfig file: %v", err)
	}

	err = ioutil.WriteFile(dest, buf, 0644)
	if err != nil {
		t.Fatalf("failed to write %s: %v", dest, err)
	}

	framework.Init(t)
}

// A TestFixture specifies the name of a subdirectory under the test/testdata/fixtures
// directory. It should consist of one or more yaml manifests.
type TestFixture string

type e2eTestFunc func(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework)

func e2eTest(ctx context.Context, fixture TestFixture, test e2eTestFunc) func(*testing.T) {
	return func(t *testing.T) {
		f, err := framework.NewE2eFramework(framework.Client)
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
			//test(t, ctx, namespace, f)
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
		return  err
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

	timeout := 1 * time.Minute
	interval := 1 * time.Second

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

	if err := kubectl.Apply(kubectl.Options{Namespace: namespace}, fixtureDir); err != nil {
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

	if err := f.DeleteDatacenters(namespace, timeout, interval); err != nil {
		return err
	}

	if err := f.DeleteNamespace(namespace, timeout, interval); err != nil {
		return err
	}

	return nil
}

func createSingleDatacenterCluster(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {
	time.Sleep(3 * time.Second)

	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := f.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := framework.ClusterKey{K8sContext: "kind-k8ssandra-1", NamespacedName: types.NamespacedName{Namespace: namespace, Name: "dc1"}}
	withDatacenter := f.NewWithDatacenter(ctx, dcKey)

	require.Eventually(withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), 3*time.Minute, 15*time.Second, "timed out waiting for datacenter to become ready")

}
