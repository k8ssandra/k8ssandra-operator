package framework

import (
	"errors"
	"github.com/k8ssandra/k8ssandra-operator/test/yq"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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
}

func NewTestFixture(name string) *TestFixture {
	return &TestFixture{Name: name}
}

func (f *TestFixture) definitionsDir() string {
	return filepath.Join(fixturesDefinitionsRoot, f.Name)
}

func (f *TestFixture) kustomizationDir() string {
	return filepath.Join(fixturesKustomizationsRoot, f.Name)
}

// countK8ssandraDatacenters counts the number of dc definitions in the K8ssandraCluster resource of
// this fixture. For now, we only read one single K8ssandraCluster declared in k8ssandra.yaml. If
// some fixtures in the future decide to create more than one K8ssandraCluster, we'll have to
// revisit this and create per-K8ssandraCluster kustomizations.
func (f *TestFixture) countK8ssandraDatacenters() (int, error) {
	k8ssandraYamlFile := filepath.Join(f.definitionsDir(), "k8ssandra.yaml")
	if _, err := os.Stat(k8ssandraYamlFile); err == nil {
		result, err := yq.Eval(".spec.cassandra.datacenters.[] as $item ireduce (0; . +1)", yq.Options{}, k8ssandraYamlFile)
		if err != nil {
			return -1, err
		}
		return strconv.Atoi(result)
	} else if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	} else {
		return -1, err
	}
}

func (f *E2eFramework) DeployFixture(namespace string, fixture *TestFixture) error {
	numDcs, err := fixture.countK8ssandraDatacenters()
	if err != nil {
		return err
	}
	dcContexts := f.AllK8sContexts()[:numDcs]
	data := &fixtureKustomization{
		Namespace:  namespace,
		Fixture:    fixture.Name,
		DcContexts: dcContexts,
	}
	err = generateKustomizationFile("fixtures/"+fixture.Name, data, fixtureKustomizeTemplate)
	if err != nil {
		return err
	}
	return f.kustomizeAndApply(fixture.kustomizationDir(), namespace, f.ControlPlaneContext)
}

type fixtureKustomization struct {
	Namespace  string
	Fixture    string
	DcContexts []string
}

const fixtureKustomizeTemplate = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{.Namespace}}
resources:
- ../../../../test/testdata/fixtures/{{ .Fixture }}
{{if .DcContexts}}
patches:
  - patch: |-
{{range $index, $dcContext := .DcContexts}}
      - op: replace
        path: /spec/cassandra/datacenters/{{ $index }}/k8sContext
        value: {{ $dcContext }}
{{end}}
    target:
      kind: K8ssandraCluster
{{end}}
`
