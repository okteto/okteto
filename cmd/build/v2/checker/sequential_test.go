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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Additional mocks needed for SequentialCheckStrategy

type MockCacheProbe struct {
	mock.Mock
}

func (m *MockCacheProbe) IsCached(ctx context.Context, manifestName, image, buildHash, svcToBuild string) (bool, string, error) {
	args := m.Called(ctx, manifestName, image, buildHash, svcToBuild)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockCacheProbe) LookupReferenceWithDigest(reference string) (string, error) {
	args := m.Called(reference)
	return args.String(0), args.Error(1)
}

type MockServiceEnvVarsSetter struct {
	mock.Mock
}

func (m *MockServiceEnvVarsSetter) SetServiceEnvVars(service, reference string) {
	m.Called(service, reference)
}

type MockOutInformer struct {
	mock.Mock
}

func (m *MockOutInformer) Infof(format string, args ...interface{}) {
	m.Called(format, args)
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
func createTestSequentialCheckStrategy() (*SequentialCheckStrategy, *MockSmartBuildController, *MockImageTagger, *MockCacheProbe, *MockMetadataCollector, *MockServiceEnvVarsSetter, *MockOutInformer) {
	smartBuildCtrl := &MockSmartBuildController{}
	tagger := &MockImageTagger{}
	imageCacheChecker := &MockCacheProbe{}
	metadataCollector := NewMockMetadataCollector()
	serviceEnvVarsSetter := &MockServiceEnvVarsSetter{}
	ioCtrl := &MockOutInformer{}

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
		setupMocks        func(*MockSmartBuildController, *MockCacheProbe, *MockMetadataCollector)
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", mock.Anything, "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
				cacheProbe.On("IsCached", mock.Anything, "test-manifest", "image2", "hash2", "service2").Return(true, "digest2", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", mock.Anything, "test-manifest", "image1", "hash1", "service1").Return(false, "", nil)
				cacheProbe.On("IsCached", mock.Anything, "test-manifest", "image2", "hash2", "service2").Return(false, "", nil)

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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", mock.Anything, "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
				cacheProbe.On("IsCached", mock.Anything, "test-manifest", "image2", "hash2", "service2").Return(false, "", nil)

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "service2").Return(&analytics.ImageBuildMetadata{})
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
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, cacheProbe *MockCacheProbe, metadataCollector *MockMetadataCollector) {
				smartBuildCtrl.On("GetBuildHash", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				cacheProbe.On("IsCached", mock.Anything, "test-manifest", "image1", "hash1", "service1").Return(false, "", errors.New("cache check failed"))

				metadataCollector.On("GetMetadata", "service1").Return(&analytics.ImageBuildMetadata{})
			},
			expectedCached:    nil,
			expectedNotCached: nil,
			expectedError:     errors.New("cache check failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, smartBuildCtrl, _, cacheProbe, metadataCollector, _, _ := createTestSequentialCheckStrategy()
			tt.setupMocks(smartBuildCtrl, cacheProbe, metadataCollector)

			cached, notCached, err := strategy.CheckServicesCache(context.Background(), tt.manifestName, tt.buildManifest, tt.svcsToBuild)

			assert.Equal(t, tt.expectedCached, cached)
			assert.Equal(t, tt.expectedNotCached, notCached)
			assert.Equal(t, tt.expectedError, err)

			smartBuildCtrl.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
			metadataCollector.AssertExpectations(t)
		})
	}
}

func TestSequentialCheckStrategy_CloneGlobalImagesToDev(t *testing.T) {
	tests := []struct {
		name          string
		images        []string
		setupMocks    func(*MockSmartBuildController, *MockServiceEnvVarsSetter, *MockMetadataCollector, *MockOutInformer)
		expectedError error
	}{
		{
			name:   "single image success",
			images: []string{"image1"},
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, serviceEnvVarsSetter *MockServiceEnvVarsSetter, metadataCollector *MockMetadataCollector, ioCtrl *MockOutInformer) {
				smartBuildCtrl.On("CloneGlobalImageToDev", "image1", "image1").Return("dev-image1", nil)
				serviceEnvVarsSetter.On("SetServiceEnvVars", "image1", "dev-image1").Return()
				metadataCollector.On("GetMetadata", "image1").Return(&analytics.ImageBuildMetadata{})
				ioCtrl.On("Infof", "Okteto Smart Builds is skipping build of %q because it's already built from cache.", mock.Anything).Return()
			},
			expectedError: nil,
		},
		{
			name:   "multiple images success",
			images: []string{"image1", "image2", "image3"},
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, serviceEnvVarsSetter *MockServiceEnvVarsSetter, metadataCollector *MockMetadataCollector, ioCtrl *MockOutInformer) {
				smartBuildCtrl.On("CloneGlobalImageToDev", "image1", "image1").Return("dev-image1", nil)
				smartBuildCtrl.On("CloneGlobalImageToDev", "image2", "image2").Return("dev-image2", nil)
				smartBuildCtrl.On("CloneGlobalImageToDev", "image3", "image3").Return("dev-image3", nil)

				serviceEnvVarsSetter.On("SetServiceEnvVars", "image1", "dev-image1").Return()
				serviceEnvVarsSetter.On("SetServiceEnvVars", "image2", "dev-image2").Return()
				serviceEnvVarsSetter.On("SetServiceEnvVars", "image3", "dev-image3").Return()

				metadataCollector.On("GetMetadata", "image1").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "image2").Return(&analytics.ImageBuildMetadata{})
				metadataCollector.On("GetMetadata", "image3").Return(&analytics.ImageBuildMetadata{})
				ioCtrl.On("Infof", "Okteto Smart Builds is skipping build of %d services [%s] because they're already built from cache.", mock.Anything, mock.Anything).Return()
			},
			expectedError: nil,
		},
		{
			name:   "clone error",
			images: []string{"image1"},
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, serviceEnvVarsSetter *MockServiceEnvVarsSetter, metadataCollector *MockMetadataCollector, ioCtrl *MockOutInformer) {
				smartBuildCtrl.On("CloneGlobalImageToDev", "image1", "image1").Return("", errors.New("clone failed"))
				metadataCollector.On("GetMetadata", "image1").Return(&analytics.ImageBuildMetadata{})
			},
			expectedError: errors.New("clone failed"),
		},
		{
			name:   "empty images list",
			images: []string{},
			setupMocks: func(smartBuildCtrl *MockSmartBuildController, serviceEnvVarsSetter *MockServiceEnvVarsSetter, metadataCollector *MockMetadataCollector, ioCtrl *MockOutInformer) {
				// Mock the Infof call that happens even with empty list
				ioCtrl.On("Infof", "Okteto Smart Builds is skipping build of %d services [%s] because they're already built from cache.", mock.Anything, mock.Anything).Return()
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, smartBuildCtrl, _, _, metadataCollector, serviceEnvVarsSetter, ioCtrl := createTestSequentialCheckStrategy()
			tt.setupMocks(smartBuildCtrl, serviceEnvVarsSetter, metadataCollector, ioCtrl)

			err := strategy.CloneGlobalImagesToDev(tt.images)

			assert.Equal(t, tt.expectedError, err)

			smartBuildCtrl.AssertExpectations(t)
			serviceEnvVarsSetter.AssertExpectations(t)
			metadataCollector.AssertExpectations(t)
			ioCtrl.AssertExpectations(t)
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

	strategy := newSequentialCheckStrategy(smartBuildCtrl, tagger, imageCacheChecker, metadataCollector, ioCtrl, serviceEnvVarsSetter)

	assert.NotNil(t, strategy)
	assert.Equal(t, smartBuildCtrl, strategy.smartBuildCtrl)
	assert.Equal(t, tagger, strategy.tagger)
	assert.Equal(t, imageCacheChecker, strategy.imageCacheChecker)
	assert.Equal(t, metadataCollector, strategy.metadataCollector)
	assert.Equal(t, ioCtrl.Out(), strategy.ioCtrl)
	assert.Equal(t, serviceEnvVarsSetter, strategy.serviceEnvVarsSetter)
}
