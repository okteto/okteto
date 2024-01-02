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
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/stretchr/testify/assert"
)

type fakeSmartBuildCtrl struct {
	isEnabled bool
}

func (fsc *fakeSmartBuildCtrl) IsEnabled() bool {
	return fsc.isEnabled
}

func TestInitTaggers(t *testing.T) {
	cfg := fakeConfig{}
	assert.Implements(t, (*imageTaggerInterface)(nil), newImageTagger(cfg, &fakeSmartBuildCtrl{}))
	assert.Implements(t, (*imageTaggerInterface)(nil), newImageWithVolumesTagger(cfg, &fakeSmartBuildCtrl{}))
}

func Test_ImageTaggerWithoutVolumes_GetServiceImageReference(t *testing.T) {
	tt := []struct {
		b             *build.Info
		name          string
		expectedImage string
		cfg           fakeConfig
	}{
		{
			name: "image is set without access to global",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: false,
				sha:       "sha",
			},
			b: &build.Info{
				Image: "nginx",
			},
			expectedImage: "nginx",
		},
		{
			name: "image is set with no clean project",
			cfg: fakeConfig{
				isClean:   false,
				hasAccess: true,
				sha:       "sha",
			},
			b: &build.Info{
				Image: "nginx",
			},
			expectedImage: "nginx",
		},
		{
			name: "image is set with access to global",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: true,
				sha:       "sha",
			},
			b: &build.Info{
				Image: "nginx",
			},
			expectedImage: "nginx",
		},
		{
			name: "image is set but in okteto dev registry",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: true,
				sha:       "sha",
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Image:      "okteto.dev/test-test:test",
			},
			expectedImage: "okteto.dev/test-test:test",
		},
		{
			name: "image is set but in okteto global registry",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: true,
				sha:       "sha",
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
				Image:      "okteto.global/test-test:test",
			},
			expectedImage: "okteto.global/test-test:test",
		},
		{
			name: "image inferred without access to global",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: false,
				sha:       "sha",
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto",
		},
		{
			name: "image inferred without clean project",
			cfg: fakeConfig{
				isClean:   false,
				hasAccess: true,
				sha:       "sha",
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto",
		},
		{
			name: "image inferred with clean project, has access to global registry but no feature flag enabled",
			cfg: fakeConfig{
				isClean:             true,
				hasAccess:           true,
				sha:                 "sha",
				isSmartBuildsEnable: false,
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto",
		},
		{
			name: "image inferred with clean project, has access to global registry and feature flag enabled",
			cfg: fakeConfig{
				isClean:             true,
				hasAccess:           true,
				sha:                 "sha",
				isSmartBuildsEnable: true,
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.global/test-test:sha",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageTagger(tc.cfg, &fakeSmartBuildCtrl{
				isEnabled: tc.cfg.isSmartBuildsEnable,
			})
			buildHash := tc.cfg.sha
			assert.Equal(t, tc.expectedImage, tagger.getServiceImageReference("test", "test", tc.b, buildHash))
		})
	}
}

func TestImageTaggerWithVolumesTag(t *testing.T) {
	tt := []struct {
		b             *build.Info
		name          string
		expectedImage string
		cfg           fakeConfig
	}{
		{
			name: "image inferred without access to global",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: false,
				sha:       "sha",
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto-with-volume-mounts",
		},
		{
			name: "image inferred without clean project",
			cfg: fakeConfig{
				isClean:   false,
				hasAccess: true,
				sha:       "sha",
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto-with-volume-mounts",
		},
		{
			name: "image inferred with clean project, has access to global registry but no feature flag enabled",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: true,
				sha:       "sha",
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto-with-volume-mounts",
		},
		{
			name: "image inferred with clean project, has access to global registry and feature flag enabled",
			cfg: fakeConfig{
				isClean:             true,
				hasAccess:           true,
				sha:                 "sha",
				isSmartBuildsEnable: true,
			},
			b: &build.Info{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.global/test-test:okteto-with-volume-mounts-sha",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageWithVolumesTagger(tc.cfg, &fakeSmartBuildCtrl{
				isEnabled: tc.cfg.isSmartBuildsEnable,
			})
			buildHash := tc.cfg.sha
			assert.Equal(t, tc.expectedImage, tagger.getServiceImageReference("test", "test", tc.b, buildHash))
		})
	}
}

func TestImageTaggerGetPossibleHashImages(t *testing.T) {
	tt := []struct {
		name           string
		sha            string
		expectedImages []string
	}{
		{
			name:           "no sha",
			sha:            "",
			expectedImages: []string{},
		},
		{
			name: "sha",
			sha:  "sha",
			expectedImages: []string{
				"okteto.dev/test-test:sha",
				"okteto.global/test-test:sha",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageTagger(
				fakeConfig{
					isOkteto: true,
				},
				&fakeSmartBuildCtrl{})
			assert.Equal(t, tc.expectedImages, tagger.getImageReferencesForTag("test", "test", tc.sha))
		})
	}
}

func TestImageTaggerWithVolumesGetPossibleHashImages(t *testing.T) {
	tt := []struct {
		name           string
		sha            string
		expectedImages []string
	}{
		{
			name:           "no sha",
			sha:            "",
			expectedImages: []string{},
		},
		{
			name: "sha",
			sha:  "sha",
			expectedImages: []string{
				"okteto.dev/test-test:okteto-with-volume-mounts-sha",
				"okteto.global/test-test:okteto-with-volume-mounts-sha",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageWithVolumesTagger(fakeConfig{
				isOkteto: true,
			}, &fakeSmartBuildCtrl{})
			assert.Equal(t, tc.expectedImages, tagger.getImageReferencesForTag("test", "test", tc.sha))
		})
	}
}

func TestImageTaggerGetPossibleTags(t *testing.T) {
	tt := []struct {
		name                 string
		sha                  string
		expectedImages       []string
		isSmartBuildsEnabled bool
	}{
		{
			name: "no sha",
			sha:  "",
			expectedImages: []string{
				"okteto.dev/test-test:okteto",
				"okteto.global/test-test:okteto",
			},
			isSmartBuildsEnabled: true,
		},
		{
			name: "sha",
			sha:  "sha",
			expectedImages: []string{
				"okteto.dev/test-test:sha",
				"okteto.global/test-test:sha",
				"okteto.dev/test-test:okteto",
				"okteto.global/test-test:okteto",
			},
			isSmartBuildsEnabled: true,
		},
		{
			name: "sha but smart builds not enabled",
			sha:  "sha",
			expectedImages: []string{
				"okteto.dev/test-test:okteto",
				"okteto.global/test-test:okteto",
			},
			isSmartBuildsEnabled: false,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageTagger(fakeConfig{isOkteto: true}, &fakeSmartBuildCtrl{
				isEnabled: tc.isSmartBuildsEnabled,
			})
			assert.Equal(t, tc.expectedImages, tagger.getImageReferencesForTagWithDefaults("test", "test", tc.sha))
		})
	}
}

func TestImageTaggerWithVolumesGetPossibleTags(t *testing.T) {
	tt := []struct {
		name           string
		sha            string
		expectedImages []string
	}{
		{
			name: "no sha",
			sha:  "",
			expectedImages: []string{
				"okteto.dev/test-test:okteto-with-volume-mounts",
				"okteto.global/test-test:okteto-with-volume-mounts",
			},
		},
		{
			name: "sha",
			sha:  "sha",
			expectedImages: []string{
				"okteto.dev/test-test:okteto-with-volume-mounts-sha",
				"okteto.global/test-test:okteto-with-volume-mounts-sha",
				"okteto.dev/test-test:okteto-with-volume-mounts",
				"okteto.global/test-test:okteto-with-volume-mounts",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageWithVolumesTagger(fakeConfig{isOkteto: true}, &fakeSmartBuildCtrl{})
			assert.Equal(t, tc.expectedImages, tagger.getImageReferencesForTagWithDefaults("test", "test", tc.sha))
		})
	}
}

func Test_getTargetRegistries(t *testing.T) {
	tt := []struct {
		name     string
		expected []string
		isOkteto bool
	}{
		{
			name:     "vanilla-cluster",
			isOkteto: false,
			expected: []string{},
		},
		{
			name:     "okteto-cluster",
			isOkteto: true,
			expected: []string{
				"okteto.dev",
				"okteto.global",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, getTargetRegistries(tc.isOkteto))
		})
	}
}
