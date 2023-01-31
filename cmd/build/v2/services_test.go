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

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

func TestAllServicesAlreadyBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	alreadyBuilt := []string{}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	// should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotAreAlreadyBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)

	alreadyBuilt := []string{"test/test-1"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	// should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestNoServiceBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	// should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotInStack(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	alreadyBuilt := []string{"test/test-1"}
	fakeReg.AddImageByName(alreadyBuilt...)
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
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestAllServicesAlreadyBuiltWithSubset(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	alreadyBuilt := []string{}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	// should not throw error
	assert.NoError(t, err)
	assert.Equal(t, 1, len(toBuild))
}

func TestServicesNotAreAlreadyBuiltWithSubset(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	alreadyBuilt := []string{"test/test-1"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	// should not throw error
	assert.NoError(t, err)
	assert.Equal(t, 0, len(toBuild))
}

func TestServicesBuildSection(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	alreadyBuilt := []string{}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	fakeManifest.Build = map[string]*model.BuildInfo{}
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{})
	// should not throw error
	assert.NoError(t, err)
	assert.Empty(t, toBuild)
}

func TestNoServiceBuiltWithSubset(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	// should not throw error
	assert.NoError(t, err)
	assert.Equal(t, 0, len(toBuild))
}

func TestGetToBuildTags(t *testing.T) {
	tests := []struct {
		name         string
		buildInfo    *model.BuildInfo
		manifestName string
		svcName      string
		isOkteto     bool
		output       []string
	}{
		{
			name: "image is set not okteto cluster",
			buildInfo: &model.BuildInfo{
				Image: "nginx",
			},
			isOkteto: false,
			output:   []string{"nginx"},
		},
		{
			name: "image is set not okteto cluster",
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			isOkteto: false,
			output:   []string{},
		},
		{
			name: "image inferred without volume mounts",
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			isOkteto:     true,
			manifestName: "test",
			svcName:      "test",
			output: []string{
				"okteto.dev/test-test:okteto",
				"okteto.global/test-test:okteto",
			},
		},
		{
			name: "image inferred with volume mounts",
			buildInfo: &model.BuildInfo{
				Image: "nginx",
				VolumesToInclude: []model.StackVolume{
					{
						LocalPath:  "",
						RemotePath: "",
					},
				},
			},
			isOkteto:     true,
			manifestName: "test",
			svcName:      "test",
			output: []string{
				"okteto.dev/test-test:okteto-with-volume-mounts",
				"okteto.global/test-test:okteto-with-volume-mounts",
			},
		},
		{
			name: "image is set without volume mounts",
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Image:      "okteto.dev/test:test",
			},
			isOkteto:     true,
			manifestName: "test",
			svcName:      "test",
			output:       []string{"okteto.dev/test:test"},
		},
		{
			name: "image is set without volume mounts",
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Image:      "test/test:test",
			},
			isOkteto:     true,
			manifestName: "test",
			svcName:      "test",
			output: []string{
				"okteto.dev/test-test:okteto",
				"okteto.global/test-test:okteto",
				"test/test:test",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOkteto,
						Registry:  "https://registry.test",
					},
				},
				CurrentContext: "test",
			}
			result := getToBuildTags(tt.manifestName, tt.svcName, tt.buildInfo)
			assert.Equal(t, tt.output, result)
		})
	}
}

func TestGetDigestFromService(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := NewBuilder(nil, fakeReg)
	tests := []struct {
		name           string
		manifest       *model.Manifest
		isOkteto       bool
		expectedErr    bool
		expectedDigest bool
	}{
		{
			name: "image is set not okteto cluster",
			manifest: &model.Manifest{
				Build: model.ManifestBuild{
					"test": {
						Dockerfile: "Dockerfile",
						Context:    ".",
					},
				},
			},
			isOkteto:       false,
			expectedErr:    true,
			expectedDigest: false,
		},
		{
			name: "image not in registry",
			manifest: &model.Manifest{
				Build: model.ManifestBuild{
					"test": {
						Dockerfile: "Dockerfile",
						Context:    ".",
					},
				},
			},
			isOkteto:       true,
			expectedErr:    false,
			expectedDigest: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tt.isOkteto,
						Registry:  "https://registry.test",
					},
				},
				CurrentContext: "test",
			}

			digest, err := bc.getDigestFromService("test", tt.manifest)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.expectedDigest {
				assert.NotEmpty(t, digest)
			} else {
				assert.Empty(t, digest)
			}

		})
	}
}
