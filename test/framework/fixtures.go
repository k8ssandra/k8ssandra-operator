package framework

import (
	"fmt"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	coreapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"github.com/k8ssandra/k8ssandra-operator/test/yq"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
)

var (
	fixturesDefinitionsRoot    = filepath.Join("..", "testdata", "fixtures")
	fixturesKustomizationsRoot = filepath.Join("..", "..", "build", "test-config", "fixtures")
)

// TestFixture is a set of one or more yaml manifests, typically a manifest for a K8ssandraCluster. They are
// automatically deployed right before the test itself is executed.
type TestFixture struct {

	// Name specifies the name of the fixture. The fixture name resolves to a subdirectory under the
	// test/testdata/fixtures directory.
	Name string

	// K8sContext is the name of the context where the fixture should be deployed.
	K8sContext string

	definitionsDir string
	kustomizeDir   string
}

func NewTestFixture(name, k8sContext string) *TestFixture {
	return &TestFixture{
		Name:           name,
		K8sContext:     k8sContext,
		definitionsDir: filepath.Join(fixturesDefinitionsRoot, name),
		kustomizeDir:   filepath.Join(fixturesKustomizationsRoot, name),
	}
}

func (f *E2eFramework) DeployFixture(namespace string, fixture *TestFixture, zones map[string]string, storage string, hostNetwork bool) error {
	contexts := make(map[string]string)
	for i, context := range f.DataPlaneContexts {
		contexts[fmt.Sprintf("kind-k8ssandra-%v", i)] = context
	}
	if fixturesKustomizeTemplate, err := os.ReadFile(filepath.Join("..", "framework", "fixtures.tmpl")); err != nil {
		return err
	} else if kustomization, err := generateFixtureKustomization(namespace, fixture, contexts, zones, storage, hostNetwork); err != nil {
		return err
	} else if err = generateKustomizationFile("fixtures/"+fixture.Name, kustomization, string(fixturesKustomizeTemplate)); err != nil {
		return err
	}
	return f.kustomizeAndApply(fixture.kustomizeDir, namespace, fixture.K8sContext)
}

type fixtureKustomization struct {
	Namespace           string
	Fixture             string
	FixtureDepth        string
	Contexts            map[string]string
	Zones               map[string]string
	Storage             string
	HostNetwork         bool
	K8ssandraCluster    *coreapi.K8ssandraClusterSpec
	CassandraDatacenter *cassdcapi.CassandraDatacenterSpec
	Stargate            *stargateapi.StargateSpec
}

// For now, we only read and kustomize:
// - the single K8ssandraCluster declared in k8ssandra.yaml, if present;
// - the single (standalone) CassandraDatacenter declared in cassdc.yaml, if present;
// - the single (standalone) Stargate declared in stargate.yaml, if present.
// If some fixtures in the future decide to create more resources, we'll have to revisit this and
// create more fine-grained kustomizations.
func generateFixtureKustomization(namespace string, fixture *TestFixture, contexts map[string]string, zones map[string]string, storage string, hostNetwork bool) (*fixtureKustomization, error) {
	kustomization := &fixtureKustomization{
		Namespace:    namespace,
		Fixture:      fixture.Name,
		FixtureDepth: strings.Repeat("../", 4+strings.Count(fixture.Name, "/")), Contexts: contexts,
		Zones:       zones,
		Storage:     storage,
		HostNetwork: hostNetwork,
	}
	if k8c, err := getFixtureK8ssandraCluster(fixture); err != nil {
		return nil, err
	} else if k8c != nil {
		kustomization.K8ssandraCluster = &k8c.Spec
	}
	if dc, err := getFixtureCassandraDatacenter(fixture); err != nil {
		return nil, err
	} else if dc != nil {
		kustomization.CassandraDatacenter = &dc.Spec
	}
	if sg, err := getFixtureStargate(fixture); err != nil {
		return nil, err
	} else if sg != nil {
		kustomization.Stargate = &sg.Spec
	}
	return kustomization, nil
}

func getFixtureK8ssandraCluster(fixture *TestFixture) (*coreapi.K8ssandraCluster, error) {
	obj, err := evalAndUnmarshal(fixture, "K8ssandraCluster", &coreapi.K8ssandraCluster{})
	if obj == nil {
		return nil, err
	}
	return obj.(*coreapi.K8ssandraCluster), err
}

func getFixtureCassandraDatacenter(fixture *TestFixture) (*cassdcapi.CassandraDatacenter, error) {
	obj, err := evalAndUnmarshal(fixture, "CassandraDatacenter", &cassdcapi.CassandraDatacenter{})
	if obj == nil {
		return nil, err
	}
	return obj.(*cassdcapi.CassandraDatacenter), err
}

func getFixtureStargate(fixture *TestFixture) (*stargateapi.Stargate, error) {
	obj, err := evalAndUnmarshal(fixture, "Stargate", &stargateapi.Stargate{})
	if obj == nil {
		return nil, err
	}
	return obj.(*stargateapi.Stargate), err
}

func evalAndUnmarshal(fixture *TestFixture, kind string, obj interface{}) (interface{}, error) {
	if files, err := utils.ListFiles(fixture.definitionsDir, "*.yaml"); err != nil {
		return nil, err
	} else if eval, err := yq.Eval(". | select(.kind == \""+kind+"\") ", yq.Options{All: true}, files...); err != nil {
		return nil, err
	} else if len(eval) == 0 {
		return nil, nil
	} else if err = yaml.Unmarshal([]byte(eval), obj); err != nil {
		return nil, err
	}
	return obj, nil
}
