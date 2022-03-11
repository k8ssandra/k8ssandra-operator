package framework

import (
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"path/filepath"
	"strings"
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

var (
	fixturesSrcDir  = filepath.Join("..", "testdata", "fixtures")
	fixturesDestDir = filepath.Join("..", "..", "build", "test-config", "fixtures")
)

func (t *TestFixture) fixtureDir() string {
	path := filepath.Join(fixturesSrcDir, t.Name)
	dir, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return dir
}

func (f *E2eFramework) DeployFixture(namespace string, fixture *TestFixture) error {
	numDcContexts, err := getNumDcContexts(fixture)
	if err != nil {
		return err
	}
	dcContexts := f.AllK8sContexts()[:numDcContexts]
	err = generateFixtureKustomizationFile(namespace, fixture.Name, dcContexts)
	if err != nil {
		return err
	}
	destDir := filepath.Join(fixturesDestDir, fixture.Name)
	return f.kustomizeAndApply(destDir, namespace, f.ControlPlaneContext)
}

// getNumDcContexts computes the exact number of contexts to kustomize by counting the number of dcs/contexts in the
// K8ssandraCluster spec for this fixture, if any. This is a bit hacky but unfortunately kustomize errors out if we
// attempt to patch a non-existing path.
func getNumDcContexts(fixture *TestFixture) (int, error) {
	files, err := utils.ListFiles(fixture.fixtureDir(), "k8ssandra.yaml")
	if err != nil {
		return 0, err
	}
	dcContexts := 0
	if len(files) > 0 {
		lines, err := utils.ReadLines(files[0])
		if err != nil {
			return 0, err
		}
		for _, line := range lines {
			if strings.Contains(line, "k8sContext:") {
				dcContexts++
			}
		}
	}
	return dcContexts, nil
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
        path: /spec/cassandra/datacenters/{{ $index }}/K8sContext
        value: {{ $dcContext }}
{{end}}
    target:
      kind: K8ssandraCluster
{{end}}
`

type fixtureKustomization struct {
	Namespace  string
	Fixture    string
	DcContexts []string
}

func generateFixtureKustomizationFile(namespace string, fixtureName string, dcContexts []string) error {
	data := &fixtureKustomization{
		Namespace:  namespace,
		Fixture:    fixtureName,
		DcContexts: dcContexts,
	}
	return generateKustomizationFile("fixtures/"+fixtureName, data, fixtureKustomizeTemplate)
}
