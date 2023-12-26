// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestNoneOfTheServicesBuilt(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestAllServicesAlreadyBuilt(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{"test/test-1", "okteto.dev/test-test-2:okteto-with-volume-mounts", "okteto.dev/test-test-3:okteto", "okteto.dev/test-test-4:okteto-with-volume-mounts"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotAreAlreadyBuilt(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{"test/test-1"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuildSvcsInput := []string{"test-1", "test-2"}
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, toBuildSvcsInput)
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(toBuildSvcsInput)-len(alreadyBuilt), len(toBuild))
}

func TestNoServiceBuilt(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{"test/test-1"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuildSvcsInput := []string{"test-1"}
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, toBuildSvcsInput)
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(toBuildSvcsInput)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotInStack(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: false,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	stack := &model.Stack{
		Services: map[string]*model.Service{
			"test-not-stack": {
				Build: &build.Info{
					Image:   "test",
					Context: ".",
				},
			},
			"test-1": {
				Build: &build.Info{
					Image:   "test-2",
					Context: ".",
					VolumesToInclude: []build.VolumeMounts{
						{
							LocalPath:  "test",
							RemotePath: "test",
						},
					},
				},
			},
		},
	}
	fakeManifest := &model.Manifest{
		Deploy: &model.DeployInfo{ComposeSection: &model.ComposeSectionInfo{
			Stack: stack,
		}},
	}
	toBuildSvcsInput := []string{}

	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, toBuildSvcsInput)
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, 0, len(toBuild))
}

func TestServicesNotOktetoWithStack(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{"test/test-1"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	stack := &model.Stack{
		Services: map[string]*model.Service{
			"test-not-stack": {},
			"test-1":         {},
		},
	}
	fakeManifest.Deploy = &model.DeployInfo{ComposeSection: &model.ComposeSectionInfo{
		Stack: stack,
	}}
	toBuildSvcsInput := []string{"test-1"}

	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, toBuildSvcsInput)
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(toBuildSvcsInput)-len(alreadyBuilt), len(toBuild))
}

func TestAllServicesAlreadyBuiltWithSubset(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, 1, len(toBuild))
}

func TestServicesNotAreAlreadyBuiltWithSubset(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{"test/test-1"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, 0, len(toBuild))
}

func TestServicesBuildSection(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	fakeManifest.Build = map[string]*build.Info{}
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{})
	// should not throw error
	require.NoError(t, err)
	require.Empty(t, toBuild)
}

func TestNoServiceBuiltWithSubset(t *testing.T) {
	fakeReg := newFakeRegistry()
	fakeConfig := fakeConfig{
		isOkteto: true,
	}
	bc := NewFakeBuilder(nil, fakeReg, fakeConfig, &fakeAnalyticsTracker{})
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, 0, len(toBuild))
}

type fakeConfig struct {
	sha                 string
	repoURL             string
	isClean             bool
	hasAccess           bool
	isOkteto            bool
	isSmartBuildsEnable bool
}

func (fc fakeConfig) HasGlobalAccess() bool                  { return fc.hasAccess }
func (fc fakeConfig) IsCleanProject() bool                   { return fc.isClean }
func (fc fakeConfig) GetGitCommit() string                   { return fc.sha }
func (fc fakeConfig) IsOkteto() bool                         { return fc.isOkteto }
func (fc fakeConfig) GetAnonymizedRepo() string              { return fc.repoURL }
func (fc fakeConfig) GetBuildContextHash(*build.Info) string { return "" }
func (fc fakeConfig) IsSmartBuildsEnabled() bool             { return fc.isSmartBuildsEnable }
