// Copyright 2024 The Okteto Authors
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

package test

import (
	"context"
	"fmt"
	"testing"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

type fakeSvcFromRegex struct {
	err error
	svc string
}

type fakeSvcToBuildDuringDeploy struct {
	err  error
	svcs []string
}

type fakeBuilderV2 struct {
	build                  error
	svcFromRegex           fakeSvcFromRegex
	svcToBuildDuringDeploy fakeSvcToBuildDuringDeploy
}

func (fb *fakeBuilderV2) GetSvcToBuildFromRegex(manifest *model.Manifest, imgFinder model.ImageFromManifest) (string, error) {
	return fb.svcFromRegex.svc, fb.svcFromRegex.err
}
func (fb *fakeBuilderV2) GetServicesToBuildDuringExecution(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error) {
	return fb.svcToBuildDuringDeploy.svcs, fb.svcToBuildDuringDeploy.err
}
func (fb *fakeBuilderV2) Build(ctx context.Context, options *types.BuildOptions) error {
	return fb.build
}

func TestDoBuild(t *testing.T) {
	ctx := context.TODO()
	manifest := &model.Manifest{
		Build: build.ManifestBuild{
			"svc1": {
				Image: "image1",
			},
			"svc2": {
				Image: "image2",
			},
		},
		Test: map[string]*model.Test{
			"svc1": {
				Image: "image1",
			},
			"svc2": {
				Image: "image2",
			},
			"svc3": {
				Image: "$OKTETO_BUILD_NOIMAGE_IMAGE",
			},
		},
	}

	ioCtrl := io.NewIOController()

	type input struct {
		builder *fakeBuilderV2
		svcs    []string
	}
	type expected struct {
		err      error
		wasBuilt bool
	}

	tt := []struct {
		expected expected
		name     string
		input    input
	}{
		{
			name: "image on okteto variable is not in build section",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: buildv2.ErrOktetBuildSyntaxImageIsNotInBuildSection,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      fmt.Errorf("test 'svc3' needs image '$OKTETO_BUILD_NOIMAGE_IMAGE' but it's not defined in the build section of the Okteto Manifest. See: https://www.okteto.com/docs/core/okteto-variables/#built-in-environment-variables-for-images-in-okteto-registry"),
				wasBuilt: false,
			},
		},
		{
			name: "no images to build",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: buildv2.ErrImageIsNotAOktetoBuildSyntax,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      nil,
				wasBuilt: false,
			},
		},
		{
			name: "another error on get svc to build from regex",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: assert.AnError,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: false,
			},
		},
		{
			name: "error checking services to build during deploy",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  assert.AnError,
						svcs: []string{},
					},
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: false,
			},
		},
		{
			name: "error on build",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  nil,
						svcs: []string{"svc1"},
					},
					build: assert.AnError,
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: true,
			},
		},
		{
			name: "buildSuccess",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  nil,
						svcs: []string{"svc1"},
					},
					build: nil,
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      nil,
				wasBuilt: true,
			},
		},
	}

	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			wasBuilt, err := doBuild(ctx, manifest, tt.input.svcs, tt.input.builder, ioCtrl)
			if tt.expected.err != nil {
				assert.ErrorContains(t, err, tt.expected.err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected.wasBuilt, wasBuilt)
		})
	}
}
