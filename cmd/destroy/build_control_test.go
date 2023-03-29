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

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

type fakeBuilderV2 struct {
	getSvcs fakeGetSvcs
	build   error
}

type fakeGetSvcs struct {
	svcs []string
	err  error
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
		name     string
		input    input
		expected error
	}{
		{
			name: "no images to build -> skip build",
			input: input{
				manifest: &model.Manifest{
					Build: model.ManifestBuild{
						"service1": {},
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
			name: "get services failed",
			input: input{
				manifest: &model.Manifest{
					Build: model.ManifestBuild{
						"service1": {},
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{},
						err:  assert.AnError,
					},
				},
			},
			expected: assert.AnError,
		},
		{
			name: "build failed",
			input: input{
				manifest: &model.Manifest{
					Build: model.ManifestBuild{
						"service1": {},
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{"service1"},
						err:  nil,
					},
					build: assert.AnError,
				},
			},
			expected: assert.AnError,
		},
		{
			name: "build all the services correctly",
			input: input{
				manifest: &model.Manifest{
					Build: model.ManifestBuild{
						"service1": {},
						"service2": {},
					},
				},
				builder: fakeBuilderV2{
					getSvcs: fakeGetSvcs{
						svcs: []string{"service1", "service2"},
						err:  nil,
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
