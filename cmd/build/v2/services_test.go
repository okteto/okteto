// Copyright 2022 The Okteto Authors
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
	bc := &OktetoBuilder{
		Registry: fakeReg,
	}
	alreadyBuilt := []string{}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotAreAlreadyBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &OktetoBuilder{
		Registry: fakeReg,
	}
	alreadyBuilt := []string{"test/test-1"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestNoServiceBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &OktetoBuilder{
		Registry: fakeReg,
	}
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1", "test-2"})
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotInStack(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &OktetoBuilder{
		Registry: fakeReg,
	}
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
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestAllServicesAlreadyBuiltWithSubset(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &OktetoBuilder{
		Registry: fakeReg,
	}
	alreadyBuilt := []string{}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, 1, len(toBuild))
}

func TestServicesNotAreAlreadyBuiltWithSubset(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &OktetoBuilder{
		Registry: fakeReg,
	}
	alreadyBuilt := []string{"test/test-1"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, 0, len(toBuild))
}

func TestNoServiceBuiltWithSubset(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &OktetoBuilder{
		Registry: fakeReg,
	}
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest, []string{"test-1"})
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, 0, len(toBuild))
}

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
		buildInfo    *model.BuildInfo
		manifestName string
		svcName      string
		output       string
	}{
		{
			name: "image is set",
			buildInfo: &model.BuildInfo{
				Image: "nginx",
			},
			output: "nginx",
		},
		{
			name: "image inferred without volume mounts",
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			manifestName: "test",
			svcName:      "test",
			output:       "okteto.dev/test-test:okteto",
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
			manifestName: "test",
			svcName:      "test",
			output:       "okteto.dev/test-test:okteto-with-volume-mounts",
		},
		{
			name: "image is set without volume mounts",
			buildInfo: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Image:      "okteto.dev/test:test",
			},
			manifestName: "test",
			svcName:      "test",
			output:       "okteto.dev/test:test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getToBuildTag(tt.manifestName, tt.svcName, tt.buildInfo)
			assert.Equal(t, tt.output, result)
		})
	}
}
