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
	"sync"
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

func createTestImageCtrl() registry.ImageCtrl {
	mockConfig := &MockImageConfig{}
	mockConfig.On("IsOktetoCluster").Return(false)
	mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
	mockConfig.On("GetNamespace").Return("test-namespace")
	mockConfig.On("GetRegistryURL").Return("test-registry.com")
	return registry.NewImageCtrl(mockConfig)
}

func createTestRegistryCacheProbe(tagger *MockImageTaggerForCache, digestResolver *MockDigestResolverForCache, logger *MockLoggerForCache) *RegistryCacheProbe {
	imageCtrl := createTestImageCtrl()
	return NewRegistryCacheProbe(
		tagger,
		"test-namespace",
		"test-registry.com",
		imageCtrl,
		digestResolver,
		logger,
	)
}

func setupLoggerMocks(logger *MockLoggerForCache) {
	logger.On("Infof", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	logger.On("Infof", mock.Anything, mock.Anything).Maybe().Return()
}

func TestRegistryCacheProbe_IsCached_EmptyBuildHash(t *testing.T) {
	mockTagger := &MockImageTaggerForCache{}
	mockDigestResolver := &MockDigestResolverForCache{}
	mockLogger := &MockLoggerForCache{}

	probe := createTestRegistryCacheProbe(mockTagger, mockDigestResolver, mockLogger)

	cached, digest, err := probe.IsCached("test-manifest", "test/image", "", "test-service")

	assert.False(t, cached)
	assert.Empty(t, digest)
	assert.NoError(t, err)
}

func TestRegistryCacheProbe_IsCached_WithImage_GlobalTag(t *testing.T) {
	tests := []struct {
		name           string
		image          string
		buildHash      string
		globalTag      string
		digestResult   string
		digestError    error
		expectedCached bool
		expectedDigest string
	}{
		{
			name:           "global tag found in cache",
			image:          "test/image",
			buildHash:      "hash123",
			globalTag:      "global/test/image:hash123",
			digestResult:   "global/test/image:hash123@sha256:abc123",
			digestError:    nil,
			expectedCached: true,
			expectedDigest: "global/test/image:hash123@sha256:abc123",
		},
		{
			name:           "global tag not found in registry",
			image:          "test/image",
			buildHash:      "hash123",
			globalTag:      "global/test/image:hash123",
			digestResult:   "",
			digestError:    errors.New("not found"),
			expectedCached: false,
			expectedDigest: "",
		},
		{
			name:           "registry error that is not 'not found'",
			image:          "test/image",
			buildHash:      "hash123",
			globalTag:      "global/test/image:hash123",
			digestResult:   "",
			digestError:    errors.New("registry error"),
			expectedCached: false,
			expectedDigest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTagger := &MockImageTaggerForCache{}
			mockDigestResolver := &MockDigestResolverForCache{}
			mockLogger := &MockLoggerForCache{}

			probe := createTestRegistryCacheProbe(mockTagger, mockDigestResolver, mockLogger)

			mockTagger.On("GetGlobalTagFromDevIfNeccesary", tt.image, "test-namespace", "test-registry.com", tt.buildHash, mock.AnythingOfType("registry.ImageCtrl")).
				Return(tt.globalTag)

			mockDigestResolver.On("GetImageTagWithDigest", tt.globalTag).
				Return(tt.digestResult, tt.digestError)
			setupLoggerMocks(mockLogger)

			cached, digest, err := probe.IsCached("test-manifest", tt.image, tt.buildHash, "test-service")

			assert.Equal(t, tt.expectedCached, cached)
			assert.Equal(t, tt.expectedDigest, digest)
			assert.NoError(t, err)

			mockTagger.AssertExpectations(t)
			mockDigestResolver.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRegistryCacheProbe_IsCached_NoImage_TaggerReferences(t *testing.T) {
	tests := []struct {
		name           string
		buildHash      string
		imageRefs      []string
		digestResult   string
		digestError    error
		expectedCached bool
		expectedDigest string
	}{
		{
			name:           "single reference found",
			buildHash:      "hash123",
			imageRefs:      []string{"test-registry.com/test-service:hash123"},
			digestResult:   "test-registry.com/test-service:hash123@sha256:def456",
			digestError:    nil,
			expectedCached: true,
			expectedDigest: "test-registry.com/test-service:hash123@sha256:def456",
		},
		{
			name:           "multiple references, first not found, second found",
			buildHash:      "hash123",
			imageRefs:      []string{"ref1:hash123", "ref2:hash123"},
			digestResult:   "ref2:hash123@sha256:ghi789",
			digestError:    nil,
			expectedCached: true,
			expectedDigest: "ref2:hash123@sha256:ghi789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTagger := &MockImageTaggerForCache{}
			mockDigestResolver := &MockDigestResolverForCache{}
			mockLogger := &MockLoggerForCache{}

			mockTagger.On("GetImageReferencesForTag", "test-manifest", "test-service", tt.buildHash).
				Return(tt.imageRefs)

			lastIndex := len(tt.imageRefs) - 1
			for i, ref := range tt.imageRefs {
				if i == lastIndex {
					mockDigestResolver.On("GetImageTagWithDigest", ref).
						Return(tt.digestResult, tt.digestError)
					setupLoggerMocks(mockLogger)
				} else {
					mockDigestResolver.On("GetImageTagWithDigest", ref).
						Return("", errors.New("not found"))
				}
			}

			probe := createTestRegistryCacheProbe(mockTagger, mockDigestResolver, mockLogger)

			cached, digest, err := probe.IsCached("test-manifest", "", tt.buildHash, "test-service")

			assert.Equal(t, tt.expectedCached, cached)
			assert.Equal(t, tt.expectedDigest, digest)
			assert.NoError(t, err)

			mockTagger.AssertExpectations(t)
			mockDigestResolver.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRegistryCacheProbe_IsCached_CacheBehavior(t *testing.T) {
	// Create mocks
	mockTagger := &MockImageTaggerForCache{}
	mockDigestResolver := &MockDigestResolverForCache{}
	mockLogger := &MockLoggerForCache{}
	mockConfig := &MockImageConfig{}
	mockConfig.On("IsOktetoCluster").Return(false)
	mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
	mockConfig.On("GetNamespace").Return("test-namespace")
	mockConfig.On("GetRegistryURL").Return("test-registry.com")
	imageCtrl := registry.NewImageCtrl(mockConfig)

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
}

func TestRegistryCacheProbe_LookupReferenceWithDigest_Success(t *testing.T) {
	// Create mocks
	mockTagger := &MockImageTaggerForCache{}
	mockDigestResolver := &MockDigestResolverForCache{}
	mockLogger := &MockLoggerForCache{}
	mockConfig := &MockImageConfig{}
	mockConfig.On("IsOktetoCluster").Return(false)
	mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
	mockConfig.On("GetNamespace").Return("test-namespace")
	mockConfig.On("GetRegistryURL").Return("test-registry.com")
	imageCtrl := registry.NewImageCtrl(mockConfig)

	reference := "test/image:tag"
	mockDigestResult := "test/image:tag@sha256:abc123"
	expectedDigest := "test/image:tag@sha256:abc123"

	// Set up mock expectations
	mockDigestResolver.On("GetImageTagWithDigest", reference).
		Return(mockDigestResult, nil)

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
	digest, err := probe.LookupReferenceWithDigest(reference)

	// Verify results
	assert.Equal(t, expectedDigest, digest)
	assert.NoError(t, err)

	// Verify mock expectations
	mockDigestResolver.AssertExpectations(t)
}

func TestRegistryCacheProbe_LookupReferenceWithDigest_Error(t *testing.T) {
	tests := []struct {
		name             string
		reference        string
		mockDigestResult string
		mockDigestError  error
		expectedDigest   string
		expectedError    error
	}{
		{
			name:             "digest lookup error",
			reference:        "test/image:tag",
			mockDigestResult: "",
			mockDigestError:  errors.New("digest error"),
			expectedDigest:   "",
			expectedError:    errors.New("digest error"),
		},
		{
			name:             "empty reference",
			reference:        "",
			mockDigestResult: "",
			mockDigestError:  errors.New("empty reference"),
			expectedDigest:   "",
			expectedError:    errors.New("empty reference"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockTagger := &MockImageTaggerForCache{}
			mockDigestResolver := &MockDigestResolverForCache{}
			mockLogger := &MockLoggerForCache{}
			mockConfig := &MockImageConfig{}
			mockConfig.On("IsOktetoCluster").Return(false)
			mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
			mockConfig.On("GetNamespace").Return("test-namespace")
			mockConfig.On("GetRegistryURL").Return("test-registry.com")
			imageCtrl := registry.NewImageCtrl(mockConfig)

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
			assert.Error(t, err)
			assert.Equal(t, tt.expectedError.Error(), err.Error())

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
		mockConfig := &MockImageConfig{}
		mockConfig.On("IsOktetoCluster").Return(false)
		mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
		mockConfig.On("GetNamespace").Return("test-namespace")
		mockConfig.On("GetRegistryURL").Return("test-registry.com")
		imageCtrl := registry.NewImageCtrl(mockConfig)

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
		mockConfig := &MockImageConfig{}
		mockConfig.On("IsOktetoCluster").Return(false)
		mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
		mockConfig.On("GetNamespace").Return("test-namespace")
		mockConfig.On("GetRegistryURL").Return("test-registry.com")
		imageCtrl := registry.NewImageCtrl(mockConfig)

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
		mockConfig := &MockImageConfig{}
		mockConfig.On("IsOktetoCluster").Return(false)
		mockConfig.On("GetGlobalNamespace").Return("test-global-namespace")
		mockConfig.On("GetNamespace").Return("test-namespace")
		mockConfig.On("GetRegistryURL").Return("test-registry.com")
		imageCtrl := registry.NewImageCtrl(mockConfig)

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
		var wg sync.WaitGroup
		const numGoroutines = 10
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				cached, _, err := probe.IsCached("test-manifest", "test-image", "hash123", "test-service")
				assert.True(t, cached)
				assert.NoError(t, err)
			}()
		}

		// Wait for all goroutines to complete
		wg.Wait()

		mockTagger.AssertExpectations(t)
	})
}
