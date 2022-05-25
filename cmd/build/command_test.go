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

package build

import (
	"testing"

	buildV1 "github.com/okteto/okteto/cmd/build/v1"
	buildV2 "github.com/okteto/okteto/cmd/build/v2"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

var fakeManifestV2 *model.Manifest = &model.Manifest{
	Build: model.ManifestBuild{
		"test-1": &model.BuildInfo{
			Image: "test/test-1",
		},
		"test-2": &model.BuildInfo{
			Image: "test/test-2",
		},
	},
	IsV2: true,
}

func getManifestWithError(_ string) (*model.Manifest, error) {
	return nil, assert.AnError
}

func getManifestWithInvalidManifestError(_ string) (*model.Manifest, error) {
	return nil, oktetoErrors.ErrInvalidManifest
}

func getFakeManifestV1(_ string) (*model.Manifest, error) {
	manifestV1 := *fakeManifestV2
	manifestV1.IsV2 = false
	return &manifestV1, nil
}

func getFakeManifestV2(_ string) (*model.Manifest, error) {
	return fakeManifestV2, nil
}

func TestIsBuildV2(t *testing.T) {
	tests := []struct {
		name           string
		manifest       *model.Manifest
		expectedAnswer bool
	}{
		{
			name: "manifest v1 is build v1",
			manifest: &model.Manifest{
				IsV2: false,
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v2 with no build section is build v1",
			manifest: &model.Manifest{
				IsV2:  true,
				Build: model.ManifestBuild{},
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v1 with build section is build v1",
			manifest: &model.Manifest{
				IsV2: false,
				Build: model.ManifestBuild{
					"test-1": &model.BuildInfo{
						Image: "test/test-1",
					},
					"test-2": &model.BuildInfo{
						Image: "test/test-2",
					},
				},
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v1 with build section is build v1",
			manifest: &model.Manifest{
				IsV2: false,
				Build: model.ManifestBuild{
					"test-1": &model.BuildInfo{
						Image: "test/test-1",
					},
					"test-2": &model.BuildInfo{
						Image: "test/test-2",
					},
				},
			},
			expectedAnswer: false,
		},
		{
			name: "manifest v2 with build section is build v2",
			manifest: &model.Manifest{
				IsV2: true,
				Build: model.ManifestBuild{
					"test-1": &model.BuildInfo{
						Image: "test/test-1",
					},
					"test-2": &model.BuildInfo{
						Image: "test/test-2",
					},
				},
			},
			expectedAnswer: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			answer := isBuildV2(tt.manifest)
			assert.Equal(t, answer, tt.expectedAnswer)
		})
	}
}

func TestBuildIsManifestV2(t *testing.T) {
	bc := &Command{
		GetManifest: getFakeManifestV2,
	}

	manifest, err := bc.GetManifest("")
	assert.Nil(t, err)
	assert.Equal(t, manifest, fakeManifestV2)
}

func TestBuildFromDockerfile(t *testing.T) {
	bc := &Command{
		GetManifest: getManifestWithError,
	}

	manifest, err := bc.GetManifest("")
	assert.NotNil(t, err)
	assert.Nil(t, manifest)
}

func TestBuildErrIfInvalidManifest(t *testing.T) {
	bc := &Command{
		GetManifest: getManifestWithInvalidManifestError,
	}

	manifest, err := bc.GetManifest("")
	assert.NotNil(t, err)
	assert.Nil(t, manifest)
}

func TestBuilderIsProperlyGenerated(t *testing.T) {
	tests := []struct {
		name              string
		buildCommand      *Command
		expectedError     bool
		isBuildV2Expected bool
	}{
		{
			name: "Builder error. Invalid manifest",
			buildCommand: &Command{
				GetManifest: getManifestWithInvalidManifestError,
			},
			expectedError:     true,
			isBuildV2Expected: false,
		},
		{
			name: "BuilderV2 called.",
			buildCommand: &Command{
				GetManifest: getFakeManifestV2,
			},
			expectedError:     false,
			isBuildV2Expected: true,
		},
		{
			name: "Manifest valid but BuilderV1 fallback.",
			buildCommand: &Command{
				GetManifest: getFakeManifestV1,
			},
			expectedError:     false,
			isBuildV2Expected: false,
		},
		{
			name: "Manifest error. BuilderV1 fallback.",
			buildCommand: &Command{
				GetManifest: getManifestWithError,
			},
			expectedError:     false,
			isBuildV2Expected: false,
		},
	}

	options := &types.BuildOptions{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder, err := tt.buildCommand.getBuilder(options)
			if err != nil && !tt.expectedError {
				t.Errorf("getBuilder() fail on '%s'. Expected nil error, got %s", tt.name, err.Error())
			}

			if builder == nil {
				if !tt.expectedError {
					t.Errorf("getBuilder() fail on '%s'. Expected builder, got nil", tt.name)
				}
			} else {
				switch builder.(type) {
				case *buildV1.OktetoBuilder:
					if tt.isBuildV2Expected {
						t.Errorf("getBuilder() fail on '%s'. Expected builderv2, got builderv1", tt.name)
					}
				case *buildV2.OktetoBuilder:
					if !tt.isBuildV2Expected {
						t.Errorf("getBuilder() fail on '%s'. Expected builderv1, got builderv2", tt.name)

					}
				}
			}
		})
	}

}
