// Copyright 2025 The Okteto Authors
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

package up

import (
	"context"
	"strings"
	"testing"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRegistry struct{}

func (fb fakeRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	return "", nil
}
func (fb fakeRegistry) GetImageTag(image, service, namespace string) string {
	return ""
}
func (fb fakeRegistry) ExpandImage(image string) string {
	return strings.Replace(image, "okteto.dev", "okteto.registry/ns", 1)
}

func TestUpBuilder_Build(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		expectError error
		manifest    *model.Manifest
		builder     *fakeBuilder
		name        string
		devName     string
	}{
		{
			name: "DevImageIsEmpty_returnsNil",
			manifest: &model.Manifest{
				Dev: map[string]*model.Dev{
					"my-dev": {Image: ""},
				},
			},
			devName:     "my-dev",
			builder:     &fakeBuilder{},
			expectError: nil,
		},
		{
			name: "GetSvcToBuildFromRegexError_returnsError",
			manifest: &model.Manifest{
				Dev: map[string]*model.Dev{
					"my-dev": {Image: "$OKTETO_BUILD_NONEXISTENT_IMAGE"},
				},
			},
			devName: "my-dev",
			builder: &fakeBuilder{
				getSvcFromRegexErr: assert.AnError,
			},
			expectError: assert.AnError,
		},
		{
			name:    "GetSvcToBuildFromRegexErrorIsNotOktetoBuildSyntaxAndNotFound_returnsNil",
			devName: "my-dev",
			manifest: &model.Manifest{
				Dev: map[string]*model.Dev{
					"my-dev": {Image: "my-image"},
				},
			},
			builder: &fakeBuilder{
				getSvcFromRegexErr: buildv2.ErrImageIsNotAOktetoBuildSyntax,
			},
			expectError: nil,
		},
		{
			name:    "Error while checking if images are built",
			devName: "my-dev",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"my-dev": {Image: "my-image"},
				},
				Dev: map[string]*model.Dev{
					"my-dev": {Image: "my-image"},
				},
			},
			builder: &fakeBuilder{
				getSvcFromRegexErr: buildv2.ErrImageIsNotAOktetoBuildSyntax,
				getServicesErr:     assert.AnError,
			},
			expectError: assert.AnError,
		},
		{
			name:    "Nothing to build",
			devName: "my-dev",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"my-dev": {Image: "my-image"},
				},
				Dev: map[string]*model.Dev{
					"my-dev": {Image: "my-image"},
				},
			},
			builder: &fakeBuilder{
				getSvcFromRegexErr: buildv2.ErrImageIsNotAOktetoBuildSyntax,
			},
			expectError: nil,
		},
		{
			name:    "Build",
			devName: "my-dev",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"my-dev": {Image: "my-image"},
				},
				Dev: map[string]*model.Dev{
					"my-dev": {Image: "my-image"},
				},
			},
			builder: &fakeBuilder{
				getSvcFromRegexErr: buildv2.ErrImageIsNotAOktetoBuildSyntax,
				services:           []string{"my-dev"},
			},
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := &upBuilder{
				manifest: tt.manifest,
				devName:  tt.devName,
				builder:  tt.builder,
				registry: &fakeRegistry{},
			}
			err := ub.build(ctx)
			require.ErrorIs(t, err, tt.expectError)
		})
	}
}

func TestGetBuildServiceFromImage(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		manifest    *model.Manifest
		wantService string
	}{
		{
			name:  "Exact match found",
			image: "okteto.dev/imageA:tag",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"svcA": {Image: "okteto.registry/ns/imageA:tag"},
					"svcB": {Image: "registry.com/repo/imageB:tag"},
				},
			},
			wantService: "svcA",
		},
		{
			name:  "Normalized image match found (okteto.dev in dev)",
			image: "okteto.registry/ns/imageA:latest",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"svcA": {Image: "okteto.dev/imageA:latest"},
					"svcB": {Image: "registry.com/repo/imageB:latest"},
				},
			},
			wantService: "svcA",
		},
		{
			name:  "Normalized image match found (okteto.dev in build)",
			image: "okteto.registry/ns/imageA:latest",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"svcA": {Image: "okteto.dev/imageA:latest"},
					"svcB": {Image: "registry.com/repo/imageB:latest"},
				},
			},
			wantService: "svcA",
		},
		{
			name:  "No match found",
			image: "registry.com/repo/imageC:tag",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"svcA": {Image: "registry.com/repo/imageA:tag"},
					"svcB": {Image: "registry.com/repo/imageB:tag"},
				},
			},
			wantService: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ub := &upBuilder{
				manifest: tt.manifest,
				registry: &fakeRegistry{},
			}

			gotService := ub.getBuildServiceFromImage(tt.image)
			assert.Equal(t, tt.wantService, gotService)
		})
	}
}

func TestGetBuildSvcFromDev(t *testing.T) {
	tests := []struct {
		name       string
		manifest   *model.Manifest
		devName    string
		ubManifest *model.Manifest
		wantImage  string
	}{
		{
			name: "Dev service exists with image",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"serviceA": {Image: "registry.com/repo/imageA:tag"},
				},
			},
			devName:   "serviceA",
			wantImage: "registry.com/repo/imageA:tag",
		},
		{
			name: "Dev service does not exist",
			manifest: &model.Manifest{
				Dev: model.ManifestDevs{
					"serviceA": {Image: "registry.com/repo/imageA:tag"},
				},
			},
			devName:   "serviceB",
			wantImage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := &upBuilder{
				manifest: tt.manifest,
				devName:  tt.devName,
			}

			gotImage := ub.getBuildSvcFromDev(tt.manifest)
			assert.Equal(t, tt.wantImage, gotImage)
		})
	}
}

func TestGetDependentServices(t *testing.T) {
	tests := []struct {
		name       string
		buildSvc   string
		bm         build.ManifestBuild
		visited    map[string]bool
		wantResult []string
	}{
		{
			name:     "No dependencies",
			buildSvc: "svcA",
			bm: build.ManifestBuild{
				"svcA": {DependsOn: []string{}},
			},
			visited:    map[string]bool{},
			wantResult: nil,
		},
		{
			name:     "Single level dependencies",
			buildSvc: "svcA",
			bm: build.ManifestBuild{
				"svcA": {DependsOn: []string{"svcB", "svcC"}},
				"svcB": {DependsOn: []string{}},
				"svcC": {DependsOn: []string{}},
			},
			visited:    map[string]bool{},
			wantResult: []string{"svcB", "svcC"},
		},
		{
			name:     "Nested dependencies",
			buildSvc: "svcA",
			bm: build.ManifestBuild{
				"svcA": {DependsOn: []string{"svcB"}},
				"svcB": {DependsOn: []string{"svcC"}},
				"svcC": {DependsOn: []string{}},
			},
			visited:    map[string]bool{},
			wantResult: []string{"svcB", "svcC"},
		},
		{
			name:     "Already visited service",
			buildSvc: "svcA",
			bm: build.ManifestBuild{
				"svcA": {DependsOn: []string{"svcB"}},
				"svcB": {DependsOn: []string{"svcC"}},
				"svcC": {DependsOn: []string{}},
			},
			visited:    map[string]bool{"svcA": true},
			wantResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ub := &upBuilder{}
			gotResult := ub.getDependentServices(tt.buildSvc, tt.bm, tt.visited)
			assert.Equal(t, tt.wantResult, gotResult)
		})
	}
}
