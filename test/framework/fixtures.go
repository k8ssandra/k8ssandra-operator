package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	fixturesDefinitionsRoot = filepath.Join("..", "testdata", "fixtures")
)

func (f *E2eFramework) DeployFixture(fixture, testProfile string) error {
	fixtureDeploymentRoot := filepath.Join(fixturesDefinitionsRoot, fixture, "fixture-deployment")
	dirs, err := os.ReadDir(fixtureDeploymentRoot)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		var context string
		switch {
		case dir.Name() == "control-plane":
			context = f.ControlPlaneContext
		case strings.HasPrefix(dir.Name(), "data-plane-"):
			idx, _ := strconv.Atoi(dir.Name()[strings.LastIndex(dir.Name(), "-")+1:])
			context = f.DataPlaneContexts[idx]
		default:
			return fmt.Errorf("invalid fixture %s subfolder: %v", fixture, dir.Name())
		}
		kustomizeDir := filepath.Join(fixtureDeploymentRoot, dir.Name(), "overlays", testProfile)
		if err := f.kustomizeAndApply(kustomizeDir, "", context); err != nil {
			return fmt.Errorf("failed to deploy fixture %s in context %v with test profile %v: %v", fixture, context, testProfile, err)
		}
	}
	return nil
}
