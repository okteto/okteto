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

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllServicesAlreadyBuilt(t *testing.T) {
	fakeReg := newFakeRegistry()
	bc := NewFakeBuilder(nil, fakeReg)
	alreadyBuilt := []string{}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotAreAlreadyBuilt(t *testing.T) {
	fakeReg := newFakeRegistry()
	bc := NewFakeBuilder(nil, fakeReg)
	alreadyBuilt := []string{"test/test-1"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestNoServiceBuilt(t *testing.T) {
	fakeReg := newFakeRegistry()
	bc := NewFakeBuilder(nil, fakeReg)
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotInStack(t *testing.T) {
	fakeReg := newFakeRegistry()
	bc := NewFakeBuilder(nil, fakeReg)
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
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestAllServicesAlreadyBuiltWithSubset(t *testing.T) {
	fakeReg := newFakeRegistry()
	bc := NewFakeBuilder(nil, fakeReg)
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
	bc := NewFakeBuilder(nil, fakeReg)
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
	bc := NewFakeBuilder(nil, fakeReg)
	alreadyBuilt := []string{}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	fakeManifest.Build = map[string]*model.BuildInfo{}
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{})
	// should not throw error
	require.NoError(t, err)
	require.Empty(t, toBuild)
}

func TestNoServiceBuiltWithSubset(t *testing.T) {
	fakeReg := newFakeRegistry()
	bc := NewFakeBuilder(nil, fakeReg)
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	require.NoError(t, fakeReg.AddImageByName(alreadyBuilt...))
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	// should not throw error
	require.NoError(t, err)
	require.Equal(t, 0, len(toBuild))
}

type fakeConfig struct {
	isClean   bool
	hasAccess bool
	sha       string
}

func (fc fakeConfig) HasGlobalAccess() bool { return fc.hasAccess }
func (fc fakeConfig) IsCleanProject() bool  { return fc.isClean }
func (fc fakeConfig) GetHash() string       { return fc.sha }

func TestGetToBuildTag(t *testing.T) {
	okteto.CurrentStore = &okteto.OktetoContextStore{
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
			},
		},
		CurrentContext: "test",
	}
	tests := []struct {
		name         string
		buildConfig  OktetoBuilderConfigInterface
		buildInfo    *model.BuildInfo
		manifestName string
		svcName      string
		output       []string
	}{
		{
			name: "image is set",
			buildInfo: &model.BuildInfo{
				Image: "nginx",
			},
			buildConfig: fakeConfig{},
			output:      []string{"nginx"},
		},
		{
			name:        "image inferred without volume mounts",
			buildConfig: fakeConfig{},
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			manifestName: "test",
			svcName:      "test",
			output:       []string{"okteto.dev/test-test:okteto"},
		},
		{
			name:        "image inferred with volume mounts",
			buildConfig: fakeConfig{},
			buildInfo: &model.BuildInfo{
				Image: "nginx",
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  "",
						RemotePath: "",
					},
				},
			},
			manifestName: "test",
			svcName:      "test",
			output:       []string{"okteto.dev/test-test:okteto-with-volume-mounts"},
		},
		{
			name:        "image is set without volume mounts",
			buildConfig: fakeConfig{},
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Image:      "okteto.dev/test:test",
			},
			manifestName: "test",
			svcName:      "test",
			output:       []string{"okteto.dev/test:test"},
		},
		{
			name: "access to global but no repo clean",
			buildConfig: fakeConfig{
				hasAccess: true,
			},
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			manifestName: "test",
			svcName:      "test",
			output:       []string{"okteto.dev/test-test:okteto"},
		},
		{
			name: "access to global and isClean",
			buildConfig: fakeConfig{
				hasAccess: true,
				isClean:   true,
				sha:       "hello-this-is-a-test",
			},
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			manifestName: "test",
			svcName:      "test",
			output: []string{
				"okteto.global/test-test:hello-this-is-a-test",
				"okteto.dev/test-test:hello-this-is-a-test",
				"okteto.global/test-test:okteto",
				"okteto.dev/test-test:okteto",
			},
		},
		{
			name: "access to global and isClean with volumes",
			buildConfig: fakeConfig{
				hasAccess: true,
				isClean:   true,
				sha:       "hello-this-is-a-test",
			},
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Image:      "nginx",
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  "",
						RemotePath: "",
					},
				},
			},
			manifestName: "test",
			svcName:      "test",
			output: []string{
				"okteto.global/test-test:hello-this-is-a-test",
				"okteto.dev/test-test:hello-this-is-a-test",
				"okteto.global/test-test:okteto-with-volume-mounts",
				"okteto.dev/test-test:okteto-with-volume-mounts",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ob := OktetoBuilder{
				Config: tt.buildConfig,
			}
			result := ob.tagsToCheck(tt.manifestName, tt.svcName, tt.buildInfo)
			require.Equal(t, tt.output, result)
		})
	}
}

func TestCheckIfCommitIsAlreadyBuilt(t *testing.T) {
	type config struct {
		cfg        OktetoBuilderConfigInterface
		cmdOptions types.BuildOptions
	}
	type expected struct {
		image     string
		hasAccess bool
	}
	tests := []struct {
		name     string
		config   config
		expected expected
	}{
		{
			name: "no access",
			config: config{
				cfg: fakeConfig{
					isClean:   true,
					hasAccess: false,
					sha:       "",
				},
			},
			expected: expected{
				image:     "",
				hasAccess: false,
			},
		},
		{
			name: "no clean commit",
			config: config{
				cfg: fakeConfig{
					isClean:   false,
					hasAccess: true,
					sha:       "",
				},
			},
			expected: expected{
				image:     "",
				hasAccess: false,
			},
		},
		{
			name: "no access no clean commit",
			config: config{
				cfg: fakeConfig{
					isClean:   false,
					hasAccess: false,
					sha:       "",
				},
			},
			expected: expected{
				image:     "",
				hasAccess: false,
			},
		},
		{
			name: "no cache option enabled",
			config: config{
				cfg: fakeConfig{
					isClean:   true,
					hasAccess: true,
					sha:       "",
				},
				cmdOptions: types.BuildOptions{
					NoCache: true,
				},
			},
			expected: expected{
				image:     "",
				hasAccess: false,
			},
		},
		{
			name: "registry find image",
			config: config{
				cfg: fakeConfig{
					isClean:   true,
					hasAccess: true,
					sha:       "thishashexists",
				},
				cmdOptions: types.BuildOptions{
					NoCache: false,
				},
			},
			expected: expected{
				image:     "",
				hasAccess: true,
			},
		},
		{
			name: "registry doesn't find image",
			config: config{
				cfg: fakeConfig{
					isClean:   true,
					hasAccess: true,
					sha:       "thishashdoesntexist",
				},
				cmdOptions: types.BuildOptions{
					NoCache: false,
				},
			},
			expected: expected{
				image:     "",
				hasAccess: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ob := OktetoBuilder{
				Registry: fakeRegistry{registry: map[string]fakeImage{
					"okteto.global/test-test:thishashexists": {},
				}},
				Config: tt.config.cfg,
			}
			_, hasAccess := ob.checkIfCommitIsAlreadyBuilt(context.Background(), "test", "test", &tt.config.cmdOptions)
			assert.Equal(t, tt.expected.hasAccess, hasAccess)
		})
	}
}
