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

func getFakeManifest(_ string) (*model.Manifest, error) {
	return fakeManifest, nil
}

func TestBuildIsManifestV2(t *testing.T) {
	bc := &Command{
		GetManifest: getFakeManifest,
	}

	manifest, isV2 := bc.getManifestAndBuildVersion(&types.BuildOptions{})
	assert.True(t, isV2)
	assert.Equal(t, manifest, fakeManifest)
}

func TestBuildIsDockerfile(t *testing.T) {
	bc := &Command{
		GetManifest: getManifestWithError,
	}

	manifest, isV2 := bc.getManifestAndBuildVersion(&types.BuildOptions{})
	assert.False(t, isV2)
	assert.Nil(t, manifest, fakeManifest)
}
