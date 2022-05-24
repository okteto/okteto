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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

var fakeManifest *model.Manifest = &model.Manifest{
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

func getFakeManifest(_ string) (*model.Manifest, error) {
	return fakeManifest, nil
}

func TestIsBuildV2(t *testing.T) {
	tests := []struct {
		name           string
		manifest       *model.Manifest
		expectedAnswer bool
	}{
		{
			name:           "nil manifest is not build v2",
			manifest:       nil,
			expectedAnswer: false,
		},
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
			answer := isBuildV2(tt.manifest)
			assert.Equal(t, answer, tt.expectedAnswer)
		})
	}
}

func TestBuildIsManifestV2(t *testing.T) {
	bc := &Command{
		GetManifest: getFakeManifest,
	}

	manifest, err := bc.getManifest(&types.BuildOptions{})
	assert.Nil(t, err)
	assert.Equal(t, manifest, fakeManifest)
}

func TestBuildFromDockerfile(t *testing.T) {
	bc := &Command{
		GetManifest: getManifestWithError,
	}

	manifest, err := bc.getManifest(&types.BuildOptions{})
	assert.Nil(t, err)
	assert.Nil(t, manifest)
}

func TestBuildErrIfInvalidManifest(t *testing.T) {
	bc := &Command{
		GetManifest: getManifestWithInvalidManifestError,
	}

	manifest, err := bc.getManifest(&types.BuildOptions{})
	assert.NotNil(t, err)
	assert.Nil(t, manifest)
}
