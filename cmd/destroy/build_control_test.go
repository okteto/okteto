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

package destroy

import (
	"context"
	"testing"

	v2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBuilderV2 struct {
	build   error
	getSvcs fakeGetSvcs
}

type fakeGetSvcs struct {
	err  error
	svcs []string
}

func (fb fakeBuilderV2) GetServicesToBuild(ctx context.Context, manifest *model.Manifest, svcToDeploy []string) ([]string, error) {
	return fb.getSvcs.svcs, fb.getSvcs.err
}

func (fb fakeBuilderV2) Build(ctx context.Context, options *types.BuildOptions) error {
	return fb.build
}

func TestBuildNecessaryImages(t *testing.T) {
	type input struct {
		manifest *model.Manifest
		builder  fakeBuilderV2
	}
	tt := []struct {
		expected error
		name     string
		input    input
	}{
		{
			name: "image is not okteto env variable",
			input: input{
				manifest: &model.Manifest{
					Build: build.ManifestBuild{},
					Destroy: &model.DestroyInfo{
						Image: "image",
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{},
					},
				},
			},
			expected: nil,
		},
		{
			name: "image is okteto variable but not defined in build section",
			input: input{
				manifest: &model.Manifest{
					Build: build.ManifestBuild{
						"okteto": {},
					},
					Destroy: &model.DestroyInfo{
						Image: "$OKTETO_BUILD_TEST_IMAGE",
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{},
					},
				},
			},
			expected: nil,
		},
		{
			name: "image is okteto variable/fails to get services",
			input: input{
				manifest: &model.Manifest{
					Build: build.ManifestBuild{
						"test": {},
					},
					Destroy: &model.DestroyInfo{
						Image: "$OKTETO_BUILD_TEST_IMAGE",
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						err: assert.AnError,
					},
				},
			},
			expected: assert.AnError,
		},
		{
			name: "image is okteto variable/is already build",
			input: input{
				manifest: &model.Manifest{
					Build: build.ManifestBuild{
						"test": {},
					},
					Destroy: &model.DestroyInfo{
						Image: "$OKTETO_BUILD_TEST_IMAGE",
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{},
					},
				},
			},
			expected: nil,
		},
		{
			name: "image is okteto variable/build fails",
			input: input{
				manifest: &model.Manifest{
					Build: build.ManifestBuild{
						"test": {},
					},
					Destroy: &model.DestroyInfo{
						Image: "$OKTETO_BUILD_TEST_IMAGE",
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{"test"},
					},
					build: assert.AnError,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "image is okteto variable/build succeeds",
			input: input{
				manifest: &model.Manifest{
					Build: build.ManifestBuild{
						"test": {},
					},
					Destroy: &model.DestroyInfo{
						Image: "$OKTETO_BUILD_TEST_IMAGE",
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{"test"},
					},
					build: nil,
				},
			},
			expected: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			buildCtrl := buildCtrl{
				builder: tc.input.builder,
			}
			err := buildCtrl.buildImageIfNecessary(context.Background(), tc.input.manifest)
			assert.ErrorIs(t, err, tc.expected)
		})
	}

}

type fakeAnalyticsTracker struct{}

func (fakeAnalyticsTracker) TrackImageBuild(...*analytics.ImageBuildMetadata) {}
func (fakeAnalyticsTracker) TrackDestroy(analytics.DestroyMetadata)           {}

func Test_newBuildCtrl(t *testing.T) {
	got := newBuildCtrl("test-control", &fakeAnalyticsTracker{}, io.NewIOController())

	require.Equal(t, "test-control", got.name)
	require.IsType(t, got.builder, &v2.OktetoBuilder{})
}
