package e2e

import (
	"context"
	cassdcapi "github.com/k8ssandra/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/k8ssandra/k8ssandra-operator/test/kubectl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	framework.Init(t)
}

// A TestFixture specifies the name of a subdirectory under the test/testdata/fixtures
// directory. It should consist of one or more yaml manifests.
type TestFixture string

type e2eTestFunc func(t *testing.T, ctx context.Context, namespace string)

func e2eTest(ctx context.Context, fixture TestFixture, test e2eTestFunc) func(*testing.T) {
	return func(t *testing.T) {
		namespace := getTestNamespace(fixture)
		fixtureDir, err := getTestFixtureDir(fixture)

		if err != nil {
			t.Fatalf("failed to get fixture directory for %s: %v", fixture, err)
		}

		err = beforeTest(t, namespace, fixtureDir)
		defer afterTest(t, namespace)

		if err == nil {
			test(t, ctx, namespace)
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

// beforeTest Creates the test namepace, deploys k8ssandra-operator, and then deploys the
// test fixture. Deploying k8ssandra-operator includes cass-operator and all of the CRDs
// required by both operators. dir should just be the base name of the text fixture
// directory that contains manifests to be deployed.
func beforeTest(t *testing.T, namespace, fixtureDir string) error {
	if err := framework.CreateNamespace(t, namespace); err != nil {
		t.Log("failed to create namespace")
		return err
	}

	if err := framework.DeployK8ssandraOperator(t, namespace); err != nil {
		t.Logf("failed to deploy k8ssandra-operator")
		return err
	}

	fixtureDir, err := framework.GetAbsPath(fixtureDir)
	if err != nil {
		return err
	}

	if err := kubectl.Apply(t, namespace, fixtureDir); err != nil {
		t.Log("kubectl apply failed")
		return err
	}

	return nil
}

func afterTest(t *testing.T, namespace string) {
	assert.NoError(t, cleanUp(t, namespace), "after test cleanup failed")
}

func cleanUp(t *testing.T, namespace string) error {
	if err := framework.DumpClusterInfo(t, namespace); err != nil {
		t.Logf("failed to dump cluster info: %v", err)
	}

	if err := framework.DeleteK8ssandraClusters(t, namespace); err != nil {
		return err
	}

	if err := framework.DeleteDatacenters(t, namespace); err != nil {
		return err
	}

	if err := framework.UndeployK8ssandraOperator(t); err != nil {
		return err
	}

	timeout := 1 * time.Minute
	interval := 5 * time.Second

	if err := framework.DeleteNamespace(namespace, timeout, interval); err != nil {
		return err
	}

	return nil
}

func createSingleDatacenterCluster(t *testing.T, ctx context.Context, namespace string) {
	time.Sleep(3 * time.Second)

	require := require.New(t)

	t.Log("check that the K8ssandraCluster was created")
	k8ssandra := &api.K8ssandraCluster{}
	err := framework.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "test"}, k8ssandra)
	require.NoError(err, "failed to get K8ssandraCluster in namespace %s", namespace)

	dcKey := types.NamespacedName{Namespace: namespace, Name: "dc1"}
	withDatacenter := newWithDatacenter(t, ctx, dcKey)

	require.Eventually(withDatacenter(func(dc *cassdcapi.CassandraDatacenter) bool {
		status := dc.GetConditionStatus(cassdcapi.DatacenterReady)
		return status == corev1.ConditionTrue && dc.Status.CassandraOperatorProgress == cassdcapi.ProgressReady
	}), 3 * time.Minute, 15 * time.Second, "timed out waiting for datacenter to become ready")

	t.Log("test passed!")
}

// newWithDatacenter is a function generator for withDatacenter that is bound to t, ctx, and key.
func newWithDatacenter(t *testing.T, ctx context.Context, key types.NamespacedName) func(func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func(condition func(dc *cassdcapi.CassandraDatacenter) bool) func() bool {
		return withDatacenter(t, ctx, key, condition)
	}
}

// withDatacenter Fetches the CassandraDatacenter specified by key and then calls condition.
func withDatacenter(t *testing.T, ctx context.Context, key types.NamespacedName, condition func(*cassdcapi.CassandraDatacenter) bool) func() bool {
	return func() bool {
		dc := &cassdcapi.CassandraDatacenter{}
		if err := framework.Client.Get(ctx, key, dc); err == nil {
			return condition(dc)
		} else {
			t.Logf("failed to get CassandraDatacenter: %s", err)
			return false
		}
	}
}
