/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEnsureDeploymentMode(t *testing.T) {
	rct := &ReaperClusterTemplate{
		ReaperTemplate: ReaperTemplate{
			StorageType: StorageTypeLocal,
		},
		DeploymentMode: DeploymentModePerDc,
	}
	changed := rct.EnsureDeploymentMode()
	assert.True(t, changed)
	assert.Equal(t, DeploymentModeSingle, rct.DeploymentMode)

	rct = &ReaperClusterTemplate{
		ReaperTemplate: ReaperTemplate{
			StorageType: StorageTypeCassandra,
		},
		DeploymentMode: DeploymentModePerDc,
	}
	changed = rct.EnsureDeploymentMode()
	assert.False(t, changed)
	assert.Equal(t, DeploymentModePerDc, rct.DeploymentMode)

	cpReaper := rct.DeepCopy()
	cpReaper.DeploymentMode = DeploymentModeControlPlane
	changed = cpReaper.EnsureDeploymentMode()
	assert.False(t, changed)
	assert.Equal(t, DeploymentModeControlPlane, cpReaper.DeploymentMode)
}
