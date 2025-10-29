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
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Additional mocks needed for SequentialCheckStrategy

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

type MockMetadataCollector struct {
	mock.Mock
	metadata map[string]*analytics.ImageBuildMetadata
}

func NewMockMetadataCollector() *MockMetadataCollector {
	return &MockMetadataCollector{
		metadata: make(map[string]*analytics.ImageBuildMetadata),
	}
}

func (m *MockMetadataCollector) GetMetadata(svcName string) *analytics.ImageBuildMetadata {
	args := m.Called(svcName)
	if args.Get(0) == nil {
		// Return a default metadata object if none is provided
		if m.metadata[svcName] == nil {
			m.metadata[svcName] = &analytics.ImageBuildMetadata{}
		}
		return m.metadata[svcName]
	}
	return args.Get(0).(*analytics.ImageBuildMetadata)
}

// Helper function to create a test SequentialCheckStrategy
func createTestSequentialCheckStrategy() (*SequentialCheckStrategy, *MockSmartBuildController, *MockImageTagger, *MockCacheProbe, *MockMetadataCollector, *MockServiceEnvVarsSetter, *io.Controller) {
	smartBuildCtrl := &MockSmartBuildController{}
	tagger := &MockImageTagger{}
	imageCacheChecker := &MockCacheProbe{}
	metadataCollector := NewMockMetadataCollector()
	serviceEnvVarsSetter := &MockServiceEnvVarsSetter{}
	ioCtrl := io.NewIOController()

	strategy := &SequentialCheckStrategy{
		smartBuildCtrl:       smartBuildCtrl,
		tagger:               tagger,
		imageCacheChecker:    imageCacheChecker,
		metadataCollector:    metadataCollector,
		ioCtrl:               ioCtrl,
		serviceEnvVarsSetter: serviceEnvVarsSetter,
	}

	return strategy, smartBuildCtrl, tagger, imageCacheChecker, metadataCollector, serviceEnvVarsSetter, ioCtrl
}

func TestSequentialCheckStrategy_CheckServicesCache(t *testing.T) {
	tests := []struct {
		name              string
		manifestName      string
		buildManifest     build.ManifestBuild
		svcsToBuild       []string
		setupMocks        func(*MockSmartBuildController, *MockCacheProbe, *MockMetadataCollector, *MockServiceEnvVarsSetter)
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
				cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(true, "digest2", nil)
				cacheProbe.On("GetFromCache", "service1").Return(true, "global-image1")
				cacheProbe.On("GetFromCache", "service2").Return(true, "global-image2")
				smartBuildCtrl.On("CloneGlobalImageToDev", "global-image1", "image1").Return("dev-image1", nil)
				smartBuildCtrl.On("CloneGlobalImageToDev", "global-image2", "image2").Return("dev-image2", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
				serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "dev-image1").Return()
				serviceEnvVarsSetter.On("SetServiceEnvVars", "service2", "dev-image2").Return()
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)
				cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(false, "", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
				cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(false, "", nil)
				cacheProbe.On("GetFromCache", "service1").Return(true, "global-image1")
				smartBuildCtrl.On("CloneGlobalImageToDev", "global-image1", "image1").Return("dev-image1", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
				serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "dev-image1").Return()
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", errors.New("cache check failed"))

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				// Only service1 should be checked for cache, service2 and service3 should be added directly to notCached
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service3").Return(&analytics.ImageBuildMetadata{})
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service3").Return("hash3")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service4").Return("hash4")

				// service1 is not cached, so service2 should be added without checking
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)
				// service3 is cached, so service4 should be checked normally
				cacheProbe.On("IsCached", "test-manifest", "image3", "hash3", "service3").Return(true, "digest3", nil)
				cacheProbe.On("IsCached", "test-manifest", "image4", "hash4", "service4").Return(false, "", nil)
				cacheProbe.On("GetFromCache", "service3").Return(true, "global-image3")
				smartBuildCtrl.On("CloneGlobalImageToDev", "global-image3", "image3").Return("dev-image3", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service3").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service4").Return(&analytics.ImageBuildMetadata{})
				serviceEnvVarsSetter.On("SetServiceEnvVars", "service3", "dev-image3").Return()
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				// Only service1 should be checked for cache, service2 and service3 should be added directly to notCached
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service3").Return(&analytics.ImageBuildMetadata{})
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector, serviceEnvVarsSetter *MockServiceEnvVarsSetter) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				// Only service1 should be checked for cache, all others should be added directly to notCached
				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service3").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service4").Return(&analytics.ImageBuildMetadata{})
			},
			expectedCached:    nil,
			expectedNotCached: []string{"service1", "service2", "service3", "service4"},
			expectedError:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, smartBuildCtrl, _, cacheProbe, metadataCollector, serviceEnvVarsSetter, _ := createTestSequentialCheckStrategy()
			tt.setupMocks(smartBuildCtrl, cacheProbe, metadataCollector, serviceEnvVarsSetter)

			cached, notCached, err := strategy.CheckServicesCache(context.Background(), tt.manifestName, tt.buildManifest, tt.svcsToBuild)

			assert.ElementsMatch(t, tt.expectedCached, cached, "cached services")
			assert.ElementsMatch(t, tt.expectedNotCached, notCached, "not cached services")
			assert.Equal(t, tt.expectedError, err)

			smartBuildCtrl.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
			metadataCollector.AssertExpectations(t)
			serviceEnvVarsSetter.AssertExpectations(t)
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
			strategy, _, tagger, cacheProbe, _, _, _ := createTestSequentialCheckStrategy()
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
	smartBuildCtrl := &MockSmartBuildController{}
	tagger := &MockImageTagger{}
	imageCacheChecker := &MockCacheProbe{}
	metadataCollector := NewMockMetadataCollector()
	ioCtrl := &io.Controller{}
	serviceEnvVarsSetter := &MockServiceEnvVarsSetter{}

	strategy := NewSequentialCheckStrategy(smartBuildCtrl, tagger, imageCacheChecker, metadataCollector, ioCtrl, serviceEnvVarsSetter)

	assert.NotNil(t, strategy)
	assert.Equal(t, smartBuildCtrl, strategy.smartBuildCtrl)
	assert.Equal(t, tagger, strategy.tagger)
	assert.Equal(t, imageCacheChecker, strategy.imageCacheChecker)
	assert.Equal(t, metadataCollector, strategy.metadataCollector)
	assert.Equal(t, ioCtrl, strategy.ioCtrl)
	assert.Equal(t, serviceEnvVarsSetter, strategy.serviceEnvVarsSetter)
}
