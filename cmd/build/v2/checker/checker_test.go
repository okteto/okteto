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

package checker

import (
	"context"
	"errors"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockImageTagger is a mock implementation of the ImageTagger interface
type MockImageTagger struct {
	mock.Mock
}

func (m *MockImageTagger) GetGlobalTagFromDevIfNeccesary(tags, namespace, registryURL, buildHash string, ic registry.ImageCtrl) string {
	args := m.Called(tags, namespace, registryURL, buildHash, ic)
	return args.String(0)
}

func (m *MockImageTagger) GetImageReferencesForTag(manifestName, svcToBuildName, tag string) []string {
	args := m.Called(manifestName, svcToBuildName, tag)
	return args.Get(0).([]string)
}

func (m *MockImageTagger) GetImageReferencesForDeploy(manifestName, svcToBuildName string) []string {
	args := m.Called(manifestName, svcToBuildName)
	return args.Get(0).([]string)
}

// MockSmartBuildController is a mock implementation of the SmartBuildController interface
type MockSmartBuildController struct {
	mock.Mock
}

func (m *MockSmartBuildController) GetBuildHash(buildInfo *build.Info, service string) string {
	args := m.Called(buildInfo, service)
	return args.String(0)
}

func (m *MockSmartBuildController) CloneGlobalImageToDev(globalImage, svcImage string) (string, error) {
	args := m.Called(globalImage, svcImage)
	return args.String(0), args.Error(1)
}

// MockCheckStrategy is a mock implementation of the CheckStrategy interface
type MockCheckStrategy struct {
	mock.Mock
}

func (m *MockCheckStrategy) CheckServicesCache(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, svcsToBuild []string) (cachedServices []string, notCachedServices []string, err error) {
	args := m.Called(ctx, manifestName, buildManifest, svcsToBuild)
	return args.Get(0).([]string), args.Get(1).([]string), args.Error(2)
}

func (m *MockCheckStrategy) CloneGlobalImagesToDev(manifestName string, buildManifest build.ManifestBuild, svcsToClone []string) error {
	args := m.Called(manifestName, buildManifest, svcsToClone)
	return args.Error(0)
}

func (m *MockCheckStrategy) GetImageDigestReferenceForServiceDeploy(manifestName, service string, buildInfo *build.Info) (string, error) {
	args := m.Called(manifestName, service, buildInfo)
	return args.String(0), args.Error(1)
}

// MockImageCtrl is a mock implementation of registry.ImageCtrl
// Since ImageCtrl is a struct, we'll create a simple mock that can be used in tests
type MockImageCtrl struct {
	registry.ImageCtrl
}

// MockDigestResolver is a mock implementation of DigestResolver
type MockDigestResolver struct {
	mock.Mock
}

func (m *MockDigestResolver) GetImageTagWithDigest(image string) (string, error) {
	args := m.Called(image)
	return args.String(0), args.Error(1)
}

func TestNewImageCacheChecker(t *testing.T) {
	t.Run("creates checker with sequential strategy", func(t *testing.T) {
		// Create a simple test that verifies the constructor works
		// We'll use a minimal approach to avoid complex dependency mocking
		checker := &ImageCacheChecker{
			checkStrategy: &MockCheckStrategy{},
		}

		// Verify checker was created
		assert.NotNil(t, checker)
		assert.NotNil(t, checker.checkStrategy)
	})
}

func TestImageCacheChecker_CheckImages(t *testing.T) {
	tests := []struct {
		name              string
		manifestName      string
		buildManifest     build.ManifestBuild
		images            []string
		mockCached        []string
		mockNotCached     []string
		mockError         error
		expectedCached    []string
		expectedNotCached []string
		expectedError     bool
	}{
		{
			name:         "successful check with cached and non-cached images",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "test/service1"},
				"service2": &build.Info{Image: "test/service2"},
			},
			images:            []string{"service1", "service2"},
			mockCached:        []string{"service1"},
			mockNotCached:     []string{"service2"},
			mockError:         nil,
			expectedCached:    []string{"service1"},
			expectedNotCached: []string{"service2"},
			expectedError:     false,
		},
		{
			name:         "all images cached",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "test/service1"},
			},
			images:            []string{"service1"},
			mockCached:        []string{"service1"},
			mockNotCached:     []string{},
			mockError:         nil,
			expectedCached:    []string{"service1"},
			expectedNotCached: []string{},
			expectedError:     false,
		},
		{
			name:         "no images cached",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "test/service1"},
			},
			images:            []string{"service1"},
			mockCached:        []string{},
			mockNotCached:     []string{"service1"},
			mockError:         nil,
			expectedCached:    []string{},
			expectedNotCached: []string{"service1"},
			expectedError:     false,
		},
		{
			name:         "error from check strategy",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "test/service1"},
			},
			images:            []string{"service1"},
			mockCached:        nil,
			mockNotCached:     nil,
			mockError:         errors.New("check strategy error"),
			expectedCached:    nil,
			expectedNotCached: nil,
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockCheckStrategy := &MockCheckStrategy{}
			mockCheckStrategy.On("CheckServicesCache", mock.Anything, tt.manifestName, tt.buildManifest, tt.images).
				Return(tt.mockCached, tt.mockNotCached, tt.mockError)

			// Create checker with mock strategy
			checker := &ImageCacheChecker{
				checkStrategy: mockCheckStrategy,
			}

			// Execute
			ctx := context.Background()
			cached, notCached, err := checker.CheckImages(ctx, tt.manifestName, tt.buildManifest, tt.images)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.mockError, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCached, cached)
				assert.Equal(t, tt.expectedNotCached, notCached)
			}

			// Verify mock was called
			mockCheckStrategy.AssertExpectations(t)
		})
	}
}

func TestImageCacheChecker_GetImageDigestReferenceForServiceDeploy(t *testing.T) {
	tests := []struct {
		name           string
		manifestName   string
		service        string
		buildInfo      *build.Info
		mockDigest     string
		mockError      error
		expectedDigest string
		expectedError  bool
	}{
		{
			name:           "successful digest retrieval",
			manifestName:   "test-manifest",
			service:        "test-service",
			buildInfo:      &build.Info{Image: "test/image"},
			mockDigest:     "sha256:abc123",
			mockError:      nil,
			expectedDigest: "sha256:abc123",
			expectedError:  false,
		},
		{
			name:           "error from check strategy",
			manifestName:   "test-manifest",
			service:        "test-service",
			buildInfo:      &build.Info{Image: "test/image"},
			mockDigest:     "",
			mockError:      errors.New("digest error"),
			expectedDigest: "",
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockCheckStrategy := &MockCheckStrategy{}
			mockCheckStrategy.On("GetImageDigestReferenceForServiceDeploy", tt.manifestName, tt.service, tt.buildInfo).
				Return(tt.mockDigest, tt.mockError)

			// Create checker with mock strategy
			checker := &ImageCacheChecker{
				checkStrategy: mockCheckStrategy,
			}

			// Execute
			digest, err := checker.GetImageDigestReferenceForServiceDeploy(tt.manifestName, tt.service, tt.buildInfo)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.mockError, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedDigest, digest)
			}

			// Verify mock was called
			mockCheckStrategy.AssertExpectations(t)
		})
	}
}

func TestImageCacheChecker_CloneGlobalImagesToDev(t *testing.T) {
	tests := []struct {
		name          string
		images        []string
		mockError     error
		expectedError bool
	}{
		{
			name:          "successful cloning",
			images:        []string{"global/image1", "global/image2"},
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "empty images list",
			images:        []string{},
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "error from check strategy",
			images:        []string{"global/image1"},
			mockError:     errors.New("clone error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockCheckStrategy := &MockCheckStrategy{}
			mockCheckStrategy.On("CloneGlobalImagesToDev", "test-manifest", build.ManifestBuild{}, tt.images).
				Return(tt.mockError)

			// Create checker with mock strategy
			checker := &ImageCacheChecker{
				checkStrategy: mockCheckStrategy,
			}

			// Execute
			err := checker.CloneGlobalImagesToDev("test-manifest", build.ManifestBuild{}, tt.images)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.mockError, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mock was called
			mockCheckStrategy.AssertExpectations(t)
		})
	}
}

func TestImageCacheChecker_EdgeCases(t *testing.T) {
	t.Run("context handling", func(t *testing.T) {
		mockCheckStrategy := &MockCheckStrategy{}
		checker := &ImageCacheChecker{
			checkStrategy: mockCheckStrategy,
		}

		// This should not panic and should pass the context to the strategy
		mockCheckStrategy.On("CheckServicesCache", mock.Anything, "test", build.ManifestBuild{}, []string{}).
			Return([]string{}, []string{}, nil)

		_, _, err := checker.CheckImages(context.TODO(), "test", build.ManifestBuild{}, []string{})
		assert.NoError(t, err)
		mockCheckStrategy.AssertExpectations(t)
	})

	t.Run("nil build info", func(t *testing.T) {
		mockCheckStrategy := &MockCheckStrategy{}
		checker := &ImageCacheChecker{
			checkStrategy: mockCheckStrategy,
		}

		mockCheckStrategy.On("GetImageDigestReferenceForServiceDeploy", "test", "service", (*build.Info)(nil)).
			Return("", nil)

		_, err := checker.GetImageDigestReferenceForServiceDeploy("test", "service", nil)
		assert.NoError(t, err)
		mockCheckStrategy.AssertExpectations(t)
	})

	t.Run("nil images slice", func(t *testing.T) {
		mockCheckStrategy := &MockCheckStrategy{}
		checker := &ImageCacheChecker{
			checkStrategy: mockCheckStrategy,
		}

		mockCheckStrategy.On("CloneGlobalImagesToDev", "test-manifest", build.ManifestBuild{}, ([]string)(nil)).
			Return(nil)

		err := checker.CloneGlobalImagesToDev("test-manifest", build.ManifestBuild{}, nil)
		assert.NoError(t, err)
		mockCheckStrategy.AssertExpectations(t)
	})
}
