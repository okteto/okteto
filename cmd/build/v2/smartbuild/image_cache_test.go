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

package smartbuild

import (
	"errors"
	"testing"

	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockImageTaggerForCache is a mock implementation of the ImageTagger interface for image cache tests
type MockImageTaggerForCache struct {
	mock.Mock
}

func (m *MockImageTaggerForCache) GetGlobalTagFromDevIfNeccesary(tags, namespace, registryURL, buildHash string, ic registry.ImageCtrl) string {
	args := m.Called(tags, namespace, registryURL, buildHash, ic)
	return args.String(0)
}

func (m *MockImageTaggerForCache) GetImageReferencesForTag(manifestName, svcToBuildName, tag string) []string {
	args := m.Called(manifestName, svcToBuildName, tag)
	return args.Get(0).([]string)
}

func (m *MockImageTaggerForCache) GetImageReferencesForDeploy(manifestName, svcToBuildName string) []string {
	args := m.Called(manifestName, svcToBuildName)
	return args.Get(0).([]string)
}

// MockDigestResolverForCache is a mock implementation of DigestResolver for image cache tests
type MockDigestResolverForCache struct {
	mock.Mock
}

func (m *MockDigestResolverForCache) GetImageTagWithDigest(image string) (string, error) {
	args := m.Called(image)
	return args.String(0), args.Error(1)
}

// MockLoggerForCache is a mock implementation of Logger for image cache tests
type MockLoggerForCache struct {
	mock.Mock
}

func (m *MockLoggerForCache) Infof(format string, args ...interface{}) {
	// Convert to slice for mock.Called
	callArgs := make([]interface{}, 0, len(args)+1)
	callArgs = append(callArgs, format)
	callArgs = append(callArgs, args...)
	m.Called(callArgs...)
}

// MockImageConfig is a mock implementation of imageConfig interface
type MockImageConfig struct {
	mock.Mock
}

func (m *MockImageConfig) IsOktetoCluster() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockImageConfig) GetGlobalNamespace() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockImageConfig) GetNamespace() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockImageConfig) GetRegistryURL() string {
	args := m.Called()
	return args.String(0)
}

// createTestImageCtrl creates a real ImageCtrl for testing
func createTestImageCtrl() registry.ImageCtrl {
	mockConfig := &MockImageConfig{}
	mockConfig.On("IsOktetoCluster").Return(false)
	mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
	mockConfig.On("GetNamespace").Return("test-namespace")
	mockConfig.On("GetRegistryURL").Return("test-registry.com")

	return registry.NewImageCtrl(mockConfig)
}

func TestNewRegistryCacheProbe(t *testing.T) {
	t.Run("creates registry cache probe with all dependencies", func(t *testing.T) {
		// Create mocks
		mockTagger := &MockImageTaggerForCache{}
		mockDigestResolver := &MockDigestResolverForCache{}
		mockLogger := &MockLoggerForCache{}
		imageCtrl := createTestImageCtrl()

		// Create registry cache probe
		probe := NewRegistryCacheProbe(
			mockTagger,
			"test-namespace",
			"test-registry.com",
			imageCtrl,
			mockDigestResolver,
			mockLogger,
		)

		// Verify probe was created with correct properties
		assert.NotNil(t, probe)
		assert.Equal(t, mockTagger, probe.tagger)
		assert.Equal(t, "test-namespace", probe.namespace)
		assert.Equal(t, "test-registry.com", probe.registryURL)
		assert.Equal(t, imageCtrl, probe.imageCtrl)
		assert.Equal(t, mockDigestResolver, probe.registry)
		assert.Equal(t, mockLogger, probe.logger)

	})
}

func TestRegistryCacheProbe_IsCached(t *testing.T) {
	tests := []struct {
		name             string
		manifestName     string
		image            string
		buildHash        string
		svcToBuild       string
		mockGlobalTag    string
		mockImageRefs    []string
		mockDigestResult string
		mockDigestError  error
		expectedCached   bool
		expectedDigest   string
		expectedError    bool
	}{
		{
			name:           "empty build hash returns not cached",
			manifestName:   "test-manifest",
			image:          "test/image",
			buildHash:      "",
			svcToBuild:     "test-service",
			expectedCached: false,
			expectedDigest: "",
			expectedError:  false,
		},
		{
			name:             "image with global tag found in cache",
			manifestName:     "test-manifest",
			image:            "test/image",
			buildHash:        "hash123",
			svcToBuild:       "test-service",
			mockGlobalTag:    "global/test/image:hash123",
			mockDigestResult: "global/test/image:hash123@sha256:abc123",
			mockDigestError:  nil,
			expectedCached:   true,
			expectedDigest:   "global/test/image:hash123@sha256:abc123",
			expectedError:    false,
		},
		{
			name:             "image with global tag not found in registry",
			manifestName:     "test-manifest",
			image:            "test/image",
			buildHash:        "hash123",
			svcToBuild:       "test-service",
			mockGlobalTag:    "global/test/image:hash123",
			mockDigestResult: "",
			mockDigestError:  errors.New("not found"),
			expectedCached:   false,
			expectedDigest:   "",
			expectedError:    false,
		},
		{
			name:             "no image provided, uses tagger references",
			manifestName:     "test-manifest",
			image:            "",
			buildHash:        "hash123",
			svcToBuild:       "test-service",
			mockImageRefs:    []string{"test-registry.com/test-service:hash123"},
			mockDigestResult: "test-registry.com/test-service:hash123@sha256:def456",
			mockDigestError:  nil,
			expectedCached:   true,
			expectedDigest:   "test-registry.com/test-service:hash123@sha256:def456",
			expectedError:    false,
		},
		{
			name:             "multiple references, first not found, second found",
			manifestName:     "test-manifest",
			image:            "",
			buildHash:        "hash123",
			svcToBuild:       "test-service",
			mockImageRefs:    []string{"ref1:hash123", "ref2:hash123"},
			mockDigestResult: "ref2:hash123@sha256:ghi789",
			mockDigestError:  nil,
			expectedCached:   true,
			expectedDigest:   "ref2:hash123@sha256:ghi789",
			expectedError:    false,
		},
		{
			name:             "registry error that is not 'not found'",
			manifestName:     "test-manifest",
			image:            "test/image",
			buildHash:        "hash123",
			svcToBuild:       "test-service",
			mockGlobalTag:    "global/test/image:hash123",
			mockDigestResult: "",
			mockDigestError:  errors.New("registry error"),
			expectedCached:   false,
			expectedDigest:   "",
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockTagger := &MockImageTaggerForCache{}
			mockDigestResolver := &MockDigestResolverForCache{}
			mockLogger := &MockLoggerForCache{}
			imageCtrl := createTestImageCtrl()

			// Set up mock expectations
			if tt.buildHash != "" {
				if tt.image != "" {
					// When image is provided, expect GetGlobalTagFromDevIfNeccesary to be called
					mockTagger.On("GetGlobalTagFromDevIfNeccesary", tt.image, "test-namespace", "test-registry.com", tt.buildHash, imageCtrl).
						Return(tt.mockGlobalTag)

					if tt.mockGlobalTag != "" {
						mockDigestResolver.On("GetImageTagWithDigest", tt.mockGlobalTag).
							Return(tt.mockDigestResult, tt.mockDigestError)

						// Use flexible mock expectations for logger
						mockLogger.On("Infof", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
						mockLogger.On("Infof", mock.Anything, mock.Anything).Maybe().Return()
					}
				} else {
					// When no image is provided, expect GetImageReferencesForTag to be called
					mockTagger.On("GetImageReferencesForTag", tt.manifestName, tt.svcToBuild, tt.buildHash).
						Return(tt.mockImageRefs)

					// Set up expectations for each reference
					for i, ref := range tt.mockImageRefs {
						if i == len(tt.mockImageRefs)-1 {
							// Last reference should return the result
							mockDigestResolver.On("GetImageTagWithDigest", ref).
								Return(tt.mockDigestResult, tt.mockDigestError)

							// Use flexible mock expectations for logger
							mockLogger.On("Infof", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
							mockLogger.On("Infof", mock.Anything, mock.Anything).Maybe().Return()
						} else {
							// Previous references should fail
							mockDigestResolver.On("GetImageTagWithDigest", ref).
								Return("", errors.New("not found"))
						}
					}
				}
			}

			// Create registry cache probe
			probe := NewRegistryCacheProbe(
				mockTagger,
				"test-namespace",
				"test-registry.com",
				imageCtrl,
				mockDigestResolver,
				mockLogger,
			)

			// Execute
			cached, digest, err := probe.IsCached(tt.manifestName, tt.image, tt.buildHash, tt.svcToBuild)

			// Verify results
			assert.Equal(t, tt.expectedCached, cached)
			assert.Equal(t, tt.expectedDigest, digest)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mock expectations
			mockTagger.AssertExpectations(t)
			mockDigestResolver.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRegistryCacheProbe_IsCached_CacheBehavior(t *testing.T) {
	t.Run("cached result is returned from cache", func(t *testing.T) {
		// Create mocks
		mockTagger := &MockImageTaggerForCache{}
		mockDigestResolver := &MockDigestResolverForCache{}
		mockLogger := &MockLoggerForCache{}
		imageCtrl := createTestImageCtrl()

		// Create registry cache probe
		probe := NewRegistryCacheProbe(
			mockTagger,
			"test-namespace",
			"test-registry.com",
			imageCtrl,
			mockDigestResolver,
			mockLogger,
		)

		// Set up mock expectations
		mockTagger.On("GetGlobalTagFromDevIfNeccesary", "test-image", "test-namespace", "test-registry.com", "hash123", imageCtrl).
			Return("test-image:tag")

		// GetImageTagWithDigest is always called to check if the image exists in the registry
		mockDigestResolver.On("GetImageTagWithDigest", "test-image:tag").
			Return("test-image:tag@sha256:cached", nil)

		mockLogger.On("Infof", "image %s found", "test-image:tag").Return()

		// Execute
		cached, digest, err := probe.IsCached("test-manifest", "test-image", "hash123", "test-service")

		// Verify results
		assert.True(t, cached)
		assert.Equal(t, "test-image:tag@sha256:cached", digest)
		assert.NoError(t, err)

		// Verify that digest resolver was called
		mockDigestResolver.AssertExpectations(t)
		mockTagger.AssertExpectations(t)
		mockLogger.AssertExpectations(t)
	})
}

func TestRegistryCacheProbe_LookupReferenceWithDigest(t *testing.T) {
	tests := []struct {
		name             string
		reference        string
		mockDigestResult string
		mockDigestError  error
		expectedDigest   string
		expectedError    bool
	}{
		{
			name:             "successful digest lookup",
			reference:        "test/image:tag",
			mockDigestResult: "test/image:tag@sha256:abc123",
			mockDigestError:  nil,
			expectedDigest:   "test/image:tag@sha256:abc123",
			expectedError:    false,
		},
		{
			name:             "digest lookup error",
			reference:        "test/image:tag",
			mockDigestResult: "",
			mockDigestError:  errors.New("digest error"),
			expectedDigest:   "",
			expectedError:    true,
		},
		{
			name:             "empty reference",
			reference:        "",
			mockDigestResult: "",
			mockDigestError:  errors.New("empty reference"),
			expectedDigest:   "",
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockTagger := &MockImageTaggerForCache{}
			mockDigestResolver := &MockDigestResolverForCache{}
			mockLogger := &MockLoggerForCache{}
			imageCtrl := createTestImageCtrl()

			// Set up mock expectations
			mockDigestResolver.On("GetImageTagWithDigest", tt.reference).
				Return(tt.mockDigestResult, tt.mockDigestError)

			// Create registry cache probe
			probe := NewRegistryCacheProbe(
				mockTagger,
				"test-namespace",
				"test-registry.com",
				imageCtrl,
				mockDigestResolver,
				mockLogger,
			)

			// Execute
			digest, err := probe.LookupReferenceWithDigest(tt.reference)

			// Verify results
			assert.Equal(t, tt.expectedDigest, digest)
			if tt.expectedError {
				assert.Error(t, err)
				assert.Equal(t, tt.mockDigestError, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mock expectations
			mockDigestResolver.AssertExpectations(t)
		})
	}
}

func TestRegistryCacheProbe_EdgeCases(t *testing.T) {
	t.Run("nil context handling", func(t *testing.T) {
		mockTagger := &MockImageTaggerForCache{}
		mockDigestResolver := &MockDigestResolverForCache{}
		mockLogger := &MockLoggerForCache{}
		imageCtrl := createTestImageCtrl()

		// Set up mock expectations
		mockTagger.On("GetGlobalTagFromDevIfNeccesary", "test-image", "test-namespace", "test-registry.com", "hash123", imageCtrl).
			Return("test-image:tag")
		mockDigestResolver.On("GetImageTagWithDigest", "test-image:tag").
			Return("test-image:tag@sha256:abc123", nil)
		mockLogger.On("Infof", mock.Anything, mock.Anything).Return()

		probe := NewRegistryCacheProbe(
			mockTagger,
			"test-namespace",
			"test-registry.com",
			imageCtrl,
			mockDigestResolver,
			mockLogger,
		)

		// Should not panic with context
		_, _, err := probe.IsCached("test-manifest", "test-image", "hash123", "test-service")
		assert.NoError(t, err)

		mockTagger.AssertExpectations(t)
		mockDigestResolver.AssertExpectations(t)
		mockLogger.AssertExpectations(t)
	})

	t.Run("empty strings handling", func(t *testing.T) {
		mockTagger := &MockImageTaggerForCache{}
		mockDigestResolver := &MockDigestResolverForCache{}
		mockLogger := &MockLoggerForCache{}
		imageCtrl := createTestImageCtrl()

		probe := NewRegistryCacheProbe(
			mockTagger,
			"test-namespace",
			"test-registry.com",
			imageCtrl,
			mockDigestResolver,
			mockLogger,
		)

		// Test with empty strings
		cached, digest, err := probe.IsCached("", "", "", "")
		assert.False(t, cached)
		assert.Empty(t, digest)
		assert.NoError(t, err)
	})

	t.Run("concurrent cache access", func(t *testing.T) {
		mockTagger := &MockImageTaggerForCache{}
		mockDigestResolver := &MockDigestResolverForCache{}
		mockLogger := &MockLoggerForCache{}
		imageCtrl := createTestImageCtrl()

		probe := NewRegistryCacheProbe(
			mockTagger,
			"test-namespace",
			"test-registry.com",
			imageCtrl,
			mockDigestResolver,
			mockLogger,
		)

		// Set up mock expectations - each goroutine will call these methods
		mockTagger.On("GetGlobalTagFromDevIfNeccesary", "test-image", "test-namespace", "test-registry.com", "hash123", imageCtrl).
			Return("test-image:tag").Maybe()

		// GetImageTagWithDigest will be called by each goroutine (10 times)
		mockDigestResolver.On("GetImageTagWithDigest", "test-image:tag").
			Return("test-image:tag@sha256:cached", nil).Maybe()

		mockLogger.On("Infof", "image %s found", "test-image:tag").Return().Maybe()

		// Test concurrent access
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				cached, _, err := probe.IsCached("test-manifest", "test-image", "hash123", "test-service")
				assert.True(t, cached)
				assert.NoError(t, err)
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		mockTagger.AssertExpectations(t)
	})
}
