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

	"github.com/okteto/okteto/pkg/okteto"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestInitTaggers(t *testing.T) {
	cfg := fakeConfig{}
	assert.Implements(t, (*imageTaggerInterface)(nil), newImageTagger(cfg))
	assert.Implements(t, (*imageTaggerInterface)(nil), newImageWithVolumesTagger(cfg))
}

func Test_ImageTaggerWithoutVolumes_GetServiceImageReference(t *testing.T) {
	tt := []struct {
		name          string
		cfg           fakeConfig
		b             *model.BuildInfo
		expectedImage string
	}{
		{
			name: "image is set without access to global",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: false,
				sha:       "sha",
			},
			b: &model.BuildInfo{
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
			b: &model.BuildInfo{
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
			b: &model.BuildInfo{
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
			b: &model.BuildInfo{
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
			b: &model.BuildInfo{
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
			b: &model.BuildInfo{
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
			b: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto",
		},
		{
			name: "image inferred with clean project and has access to global registry",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: true,
				sha:       "sha",
			},
			b: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.global/test-test:b794839154e982d4df54fe7141aee87029f4099599a939b8ebd4a64d368b5c29",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageTagger(tc.cfg)
			buildHash := getBuildHashFromCommit(tc.b, tc.cfg.GetGitCommit())
			assert.Equal(t, tc.expectedImage, tagger.getServiceImageReference("test", "test", tc.b, buildHash))
		})
	}
}

func TestImageTaggerWithVolumesTag(t *testing.T) {
	tt := []struct {
		name          string
		cfg           fakeConfig
		b             *model.BuildInfo
		expectedImage string
	}{
		{
			name: "image inferred without access to global",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: false,
				sha:       "sha",
			},
			b: &model.BuildInfo{
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
			b: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.dev/test-test:okteto-with-volume-mounts",
		},
		{
			name: "image inferred with clean project and has access to global registry",
			cfg: fakeConfig{
				isClean:   true,
				hasAccess: true,
				sha:       "sha",
			},
			b: &model.BuildInfo{
				Dockerfile: "Dockerfile",
				Context:    ".",
			},
			expectedImage: "okteto.global/test-test:okteto-with-volume-mounts-b794839154e982d4df54fe7141aee87029f4099599a939b8ebd4a64d368b5c29",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageWithVolumesTagger(tc.cfg)
			buildHash := getBuildHashFromCommit(tc.b, tc.cfg.GetGitCommit())
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
			tagger := newImageTagger(fakeConfig{})
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
			tagger := newImageWithVolumesTagger(fakeConfig{})
			assert.Equal(t, tc.expectedImages, tagger.getImageReferencesForTag("test", "test", tc.sha))
		})
	}
}

func TestImageTaggerGetPossibleTags(t *testing.T) {
	tt := []struct {
		name           string
		sha            string
		expectedImages []string
	}{
		{
			name: "no sha",
			sha:  "",
			expectedImages: []string{
				"okteto.dev/test-test:okteto",
				"okteto.global/test-test:okteto",
			},
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
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tagger := newImageTagger(fakeConfig{})
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
			tagger := newImageWithVolumesTagger(fakeConfig{})
			assert.Equal(t, tc.expectedImages, tagger.getImageReferencesForTagWithDefaults("test", "test", tc.sha))
		})
	}
}

func Test_getTargetRegistries(t *testing.T) {
	tt := []struct {
		name     string
		isOkteto bool
		expected []string
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
			okteto.CurrentStore = &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Namespace: "test",
						IsOkteto:  tc.isOkteto,
					},
				},
				CurrentContext: "test",
			}
			assert.Equal(t, tc.expected, getTargetRegistries())
		})
	}
}
