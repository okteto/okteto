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
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Additional mocks needed for SequentialCheckStrategy

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

type MockHasherController struct {
	mock.Mock
}

func (m *MockHasherController) hashProjectCommit(buildInfo *build.Info) (string, error) {
	args := m.Called(buildInfo)
	return args.String(0), args.Error(1)
}

func (m *MockHasherController) hashWithBuildContext(buildInfo *build.Info, service string) string {
	args := m.Called(buildInfo, service)
	return args.String(0)
}

type MockCacheProbe struct {
	mock.Mock
}

func (m *MockCacheProbe) IsCached(manifestName, image, buildHash, svcToBuild string) (bool, string, error) {
	args := m.Called(manifestName, image, buildHash, svcToBuild)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockCacheProbe) LookupReferenceWithDigest(reference string) (string, error) {
	args := m.Called(reference)
	return args.String(0), args.Error(1)
}

func (m *MockCacheProbe) GetFromCache(svc string) (hit bool, reference string) {
	args := m.Called(svc)
	return args.Bool(0), args.String(1)
}

type MockServiceEnvVarsSetter struct {
	mock.Mock
}

func (m *MockServiceEnvVarsSetter) SetServiceEnvVars(service, reference string) {
	m.Called(service, reference)
}

type MockRegistryController struct {
	mock.Mock
}

func (m *MockRegistryController) GetDevImageFromGlobal(image string) string {
	args := m.Called(image)
	return args.String(0)
}

func (m *MockRegistryController) GetImageTagWithDigest(image string) (string, error) {
	args := m.Called(image)
	return args.String(0), args.Error(1)
}

func (m *MockRegistryController) Clone(from, to string) (string, error) {
	args := m.Called(from, to)
	return args.String(0), args.Error(1)
}

func (m *MockRegistryController) IsGlobalRegistry(image string) bool {
	args := m.Called(image)
	return args.Bool(0)
}

func (m *MockRegistryController) IsOktetoRegistry(image string) bool {
	args := m.Called(image)
	return args.Bool(0)
}

// Helper function to create a test SequentialCheckStrategy
func createTestSequentialCheckStrategy() (*SequentialCheckStrategy, *MockImageTagger, *MockHasherController, *MockCacheProbe, *MockServiceEnvVarsSetter, *MockRegistryController) {
	tagger := &MockImageTagger{}
	hasher := &MockHasherController{}
	imageCacheChecker := &MockCacheProbe{}
	serviceEnvVarsSetter := &MockServiceEnvVarsSetter{}
	ioCtrl := io.NewIOController()

	// Create a real cloner with a mock registry controller
	mockRegistry := &MockRegistryController{}
	cloner := NewCloner(mockRegistry, ioCtrl)

	strategy := &SequentialCheckStrategy{
		tagger:               tagger,
		hasher:               hasher,
		imageCacheChecker:    imageCacheChecker,
		ioCtrl:               ioCtrl,
		serviceEnvVarsSetter: serviceEnvVarsSetter,
		cloner:               cloner,
	}

	return strategy, tagger, hasher, imageCacheChecker, serviceEnvVarsSetter, mockRegistry
}

func TestSequentialCheckStrategy_CheckServicesCache(t *testing.T) {
	tests := []struct {
		name              string
		manifestName      string
		buildManifest     build.ManifestBuild
		svcsToBuild       []string
		setupMocks        func(*MockHasherController, *MockCacheProbe, *MockServiceEnvVarsSetter, *MockRegistryController)
		expectedCached    []string
		expectedNotCached []string
		expectedError     error
	}{
		{
			name:         "all services cached",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2"},
			},
			svcsToBuild: []string{"service1", "service2"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
				cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(true, "digest2", nil)
				cacheProbe.On("GetFromCache", "service1").Return(true, "global-image1")
				cacheProbe.On("GetFromCache", "service2").Return(true, "global-image2")

				// Set up mock registry controller expectations for cloning
				mockRegistry.On("IsGlobalRegistry", "global-image1").Return(false)
				mockRegistry.On("IsGlobalRegistry", "global-image2").Return(false)

				serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "global-image1").Return()
				serviceEnvVarsSetter.On("SetServiceEnvVars", "service2", "global-image2").Return()
			},
			expectedCached:    []string{"service1", "service2"},
			expectedNotCached: nil,
			expectedError:     nil,
		},
		{
			name:         "no services cached",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2"},
			},
			svcsToBuild: []string{"service1", "service2"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)
				cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(false, "", nil)
			},
			expectedCached:    nil,
			expectedNotCached: []string{"service1", "service2"},
			expectedError:     nil,
		},
		{
			name:         "mixed cache results",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2"},
			},
			svcsToBuild: []string{"service1", "service2"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
				cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(false, "", nil)
				cacheProbe.On("GetFromCache", "service1").Return(true, "global-image1")

				// Set up mock registry controller expectations for cloning
				mockRegistry.On("IsGlobalRegistry", "global-image1").Return(false)

				serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "global-image1").Return()
			},
			expectedCached:    []string{"service1"},
			expectedNotCached: []string{"service2"},
			expectedError:     nil,
		},
		{
			name:         "cache check error",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
			},
			svcsToBuild: []string{"service1"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", errors.New("cache check failed"))
			},
			expectedCached:    nil,
			expectedNotCached: nil,
			expectedError:     errors.New("cache check failed"),
		},
		{
			name:         "optimization: dependent services added to notCached without cache check",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
				"service3": &build.Info{Image: "image3", DependsOn: []string{"service1"}},
			},
			svcsToBuild: []string{"service1", "service2", "service3"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				// Only service1 should be checked for cache, service2 and service3 should be added directly to notCached
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)

			},
			expectedCached:    nil,
			expectedNotCached: []string{"service1", "service2", "service3"},
			expectedError:     nil,
		},
		{
			name:         "optimization: mixed scenario with cached and not cached services",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
				"service3": &build.Info{Image: "image3"},
				"service4": &build.Info{Image: "image4", DependsOn: []string{"service3"}},
			},
			svcsToBuild: []string{"service1", "service2", "service3", "service4"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service3").Return("hash3")
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service4").Return("hash4")

				// service1 is not cached, so service2 should be added without checking
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)
				// service3 is cached, so service4 should be checked normally
				cacheProbe.On("IsCached", "test-manifest", "image3", "hash3", "service3").Return(true, "digest3", nil)
				cacheProbe.On("IsCached", "test-manifest", "image4", "hash4", "service4").Return(false, "", nil)
				cacheProbe.On("GetFromCache", "service3").Return(true, "global-image3")

				// Set up mock registry controller expectations for cloning
				mockRegistry.On("IsGlobalRegistry", "global-image3").Return(false)

				serviceEnvVarsSetter.On("SetServiceEnvVars", "service3", "global-image3").Return()
			},
			expectedCached:    []string{"service3"},
			expectedNotCached: []string{"service1", "service2", "service4"},
			expectedError:     nil,
		},
		{
			name:         "optimization: nested dependencies - service2 depends on service1, service3 depends on service2",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
				"service3": &build.Info{Image: "image3", DependsOn: []string{"service2"}},
			},
			svcsToBuild: []string{"service1", "service2", "service3"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				// Only service1 should be checked for cache, service2 and service3 should be added directly to notCached
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)

			},
			expectedCached:    nil,
			expectedNotCached: []string{"service1", "service2", "service3"},
			expectedError:     nil,
		},
		{
			name:         "optimization: complex nested dependencies - service1 -> service2 -> service3 -> service4",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
				"service3": &build.Info{Image: "image3", DependsOn: []string{"service2"}},
				"service4": &build.Info{Image: "image4", DependsOn: []string{"service3"}},
			},
			svcsToBuild: []string{"service1", "service2", "service3", "service4"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				// Only service1 should be checked for cache, all others should be added directly to notCached
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)

			},
			expectedCached:    nil,
			expectedNotCached: []string{"service1", "service2", "service3", "service4"},
			expectedError:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, _, hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry := createTestSequentialCheckStrategy()
			tt.setupMocks(hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry)

			cached, notCached, err := strategy.CheckServicesCache(context.Background(), tt.manifestName, tt.buildManifest, tt.svcsToBuild)

			assert.ElementsMatch(t, tt.expectedCached, cached, "cached services")
			assert.ElementsMatch(t, tt.expectedNotCached, notCached, "not cached services")
			assert.Equal(t, tt.expectedError, err)

			hasher.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
			serviceEnvVarsSetter.AssertExpectations(t)
			mockRegistry.AssertExpectations(t)
		})
	}
}

func TestSequentialCheckStrategy_GetImageDigestReferenceForServiceDeploy(t *testing.T) {
	tests := []struct {
		name              string
		manifestName      string
		service           string
		buildInfo         *build.Info
		setupMocks        func(*MockImageTagger, *MockCacheProbe)
		expectedReference string
		expectedError     error
	}{
		{
			name:         "dockerfile with image found",
			manifestName: "test-manifest",
			service:      "service1",
			buildInfo:    &build.Info{Dockerfile: "Dockerfile", Image: ""},
			setupMocks: func(tagger *MockImageTagger, cacheProbe *MockCacheProbe) {
				tagger.On("GetImageReferencesForDeploy", "test-manifest", "service1").Return([]string{"ref1", "ref2"})
				cacheProbe.On("LookupReferenceWithDigest", "ref1").Return("ref1@digest1", nil)
			},
			expectedReference: "ref1@digest1",
			expectedError:     nil,
		},
		{
			name:         "dockerfile with image not found in first reference",
			manifestName: "test-manifest",
			service:      "service1",
			buildInfo:    &build.Info{Dockerfile: "Dockerfile", Image: ""},
			setupMocks: func(tagger *MockImageTagger, cacheProbe *MockCacheProbe) {
				tagger.On("GetImageReferencesForDeploy", "test-manifest", "service1").Return([]string{"ref1", "ref2"})
				cacheProbe.On("LookupReferenceWithDigest", "ref1").Return("", oktetoErrors.ErrNotFound)
				cacheProbe.On("LookupReferenceWithDigest", "ref2").Return("ref2@digest2", nil)
			},
			expectedReference: "ref2@digest2",
			expectedError:     nil,
		},
		{
			name:         "predefined image found",
			manifestName: "test-manifest",
			service:      "service1",
			buildInfo:    &build.Info{Image: "predefined-image"},
			setupMocks: func(tagger *MockImageTagger, cacheProbe *MockCacheProbe) {
				cacheProbe.On("LookupReferenceWithDigest", "predefined-image").Return("predefined-image@digest", nil)
			},
			expectedReference: "predefined-image@digest",
			expectedError:     nil,
		},
		{
			name:         "predefined image not found",
			manifestName: "test-manifest",
			service:      "service1",
			buildInfo:    &build.Info{Image: "predefined-image"},
			setupMocks: func(tagger *MockImageTagger, cacheProbe *MockCacheProbe) {
				cacheProbe.On("LookupReferenceWithDigest", "predefined-image").Return("", oktetoErrors.ErrNotFound)
			},
			expectedReference: "",
			expectedError:     errors.New("images [predefined-image] not found"),
		},
		{
			name:         "dockerfile with all references not found",
			manifestName: "test-manifest",
			service:      "service1",
			buildInfo:    &build.Info{Dockerfile: "Dockerfile", Image: ""},
			setupMocks: func(tagger *MockImageTagger, cacheProbe *MockCacheProbe) {
				tagger.On("GetImageReferencesForDeploy", "test-manifest", "service1").Return([]string{"ref1", "ref2"})
				cacheProbe.On("LookupReferenceWithDigest", "ref1").Return("", oktetoErrors.ErrNotFound)
				cacheProbe.On("LookupReferenceWithDigest", "ref2").Return("", oktetoErrors.ErrNotFound)
			},
			expectedReference: "",
			expectedError:     errors.New("images [ref1, ref2] not found"),
		},
		{
			name:         "registry error",
			manifestName: "test-manifest",
			service:      "service1",
			buildInfo:    &build.Info{Image: "predefined-image"},
			setupMocks: func(tagger *MockImageTagger, cacheProbe *MockCacheProbe) {
				cacheProbe.On("LookupReferenceWithDigest", "predefined-image").Return("", errors.New("registry error"))
			},
			expectedReference: "",
			expectedError:     fmt.Errorf("error checking image at registry predefined-image: registry error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, tagger, _, cacheProbe, _, _ := createTestSequentialCheckStrategy()
			tt.setupMocks(tagger, cacheProbe)

			reference, err := strategy.GetImageDigestReferenceForServiceDeploy(tt.manifestName, tt.service, tt.buildInfo)

			assert.Equal(t, tt.expectedReference, reference)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			tagger.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
		})
	}
}

func TestNewSequentialCheckStrategy(t *testing.T) {
	tagger := &MockImageTagger{}
	hasher := &MockHasherController{}
	imageCacheChecker := &MockCacheProbe{}
	ioCtrl := &io.Controller{}
	serviceEnvVarsSetter := &MockServiceEnvVarsSetter{}
	mockRegistry := &MockRegistryController{}
	cloner := NewCloner(mockRegistry, ioCtrl)

	strategy := NewSequentialCheckStrategy(tagger, hasher, imageCacheChecker, ioCtrl, serviceEnvVarsSetter, cloner)

	assert.NotNil(t, strategy)
	assert.Equal(t, tagger, strategy.tagger)
	assert.Equal(t, hasher, strategy.hasher)
	assert.Equal(t, imageCacheChecker, strategy.imageCacheChecker)
	assert.Equal(t, ioCtrl, strategy.ioCtrl)
	assert.Equal(t, serviceEnvVarsSetter, strategy.serviceEnvVarsSetter)
	assert.Equal(t, cloner, strategy.cloner)
}
