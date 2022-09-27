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

func TestGetServicesToBuildWithManifestV2(t *testing.T) {

	testCases := []struct {
		name                  string
		manifestBuildImages   []string
		alreadyBuilt          []string
		servicesToDeploy      []string
		expectedImagesToBuild []string
	}{
		{
			name:                  "all services already built and deploy all",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{"1", "2", "3"},
			servicesToDeploy:      []string{"1", "2", "3"},
			expectedImagesToBuild: []string{},
		},
		{
			name:                  "all services already built and deploy subset",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{"1", "2", "3"},
			servicesToDeploy:      []string{"1", "2"},
			expectedImagesToBuild: []string{},
		},
		{
			name:                  "Same services to deploy and already built",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{"1", "2"},
			servicesToDeploy:      []string{"1", "2"},
			expectedImagesToBuild: []string{},
		},
		{
			name:                  "some services already built",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{"1", "2"},
			servicesToDeploy:      []string{"2", "3"},
			expectedImagesToBuild: []string{"3"},
		},
		{
			name:                  "No intesection between services to deploy and already built",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{"1"},
			servicesToDeploy:      []string{"2", "3"},
			expectedImagesToBuild: []string{"2", "3"},
		},
		{
			name:                  "subset are already built",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{"1"},
			servicesToDeploy:      []string{"1", "2"},
			expectedImagesToBuild: []string{"2"},
		},
		{
			name:                  "no services to deploy",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{"1"},
			servicesToDeploy:      []string{},
			expectedImagesToBuild: []string{},
		},
		{
			name:                  "no services already built",
			manifestBuildImages:   []string{"1", "2", "3"},
			alreadyBuilt:          []string{},
			servicesToDeploy:      []string{"1", "2"},
			expectedImagesToBuild: []string{"1", "2"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			fakeReg := test.NewFakeOktetoRegistry(nil)
			bc := NewBuilder(nil, fakeReg)
			fakeReg.AddImageByName(testCase.alreadyBuilt...)
			ctx := context.Background()
			var fakeManifestV2 *model.Manifest = &model.Manifest{
				Build: model.ManifestBuild{},
				IsV2:  true}

			for _, image := range testCase.manifestBuildImages {
				fakeManifestV2.Build[image] = &model.BuildInfo{Image: image}
			}

			toBuild, err := bc.GetServicesToBuild(ctx, fakeManifestV2, testCase.servicesToDeploy)
			assert.NoError(t, err)

			assert.Equal(t, sliceToSet(testCase.expectedImagesToBuild), sliceToSet(toBuild))
		})
	}
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
