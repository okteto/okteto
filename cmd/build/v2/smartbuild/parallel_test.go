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
	"sync"
	"testing"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Helper function to create a test ParallelCheckStrategy
func createTestParallelCheckStrategy() (*ParallelCheckStrategy, *MockImageTagger, *MockHasherController, *MockCacheProbe, *MockServiceEnvVarsSetter, *MockRegistryController) {
	tagger := &MockImageTagger{}
	hasher := &MockHasherController{}
	imageCacheChecker := &MockCacheProbe{}
	serviceEnvVarsSetter := &MockServiceEnvVarsSetter{}
	ioCtrl := io.NewIOController()

	// Create a real cloner with a mock registry controller
	mockRegistry := &MockRegistryController{}
	cloner := NewCloner(mockRegistry, ioCtrl)

	strategy := &ParallelCheckStrategy{
		tagger:               tagger,
		hasher:               hasher,
		imageCacheChecker:    imageCacheChecker,
		ioCtrl:               ioCtrl,
		serviceEnvVarsSetter: serviceEnvVarsSetter,
		cloner:               cloner,
	}

	return strategy, tagger, hasher, imageCacheChecker, serviceEnvVarsSetter, mockRegistry
}

func TestNewParallelCheckStrategy(t *testing.T) {
	tagger := &MockImageTagger{}
	hasher := &MockHasherController{}
	imageCacheChecker := &MockCacheProbe{}
	ioCtrl := &io.Controller{}
	serviceEnvVarsSetter := &MockServiceEnvVarsSetter{}
	mockRegistry := &MockRegistryController{}
	cloner := NewCloner(mockRegistry, ioCtrl)

	strategy := NewParallelCheckStrategy(tagger, hasher, imageCacheChecker, ioCtrl, serviceEnvVarsSetter, cloner)

	assert.NotNil(t, strategy)
	assert.Equal(t, tagger, strategy.tagger)
	assert.Equal(t, hasher, strategy.hasher)
	assert.Equal(t, imageCacheChecker, strategy.imageCacheChecker)
	assert.Equal(t, ioCtrl, strategy.ioCtrl)
	assert.Equal(t, serviceEnvVarsSetter, strategy.serviceEnvVarsSetter)
	assert.Equal(t, cloner, strategy.cloner)
}

func TestParallelCheckStrategy_CheckServicesCache(t *testing.T) {
	tests := []struct {
		name              string
		manifestName      string
		buildManifest     build.ManifestBuild
		svcsToBuild       []string
		setupMocks        func(*MockHasherController, *MockCacheProbe, *MockServiceEnvVarsSetter, *MockRegistryController)
		expectedCached    []string
		expectedNotCached []string
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

				mockRegistry.On("IsGlobalRegistry", "digest1").Return(false)
				mockRegistry.On("IsGlobalRegistry", "digest2").Return(false)

				serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "digest1").Return()
				serviceEnvVarsSetter.On("SetServiceEnvVars", "service2", "digest2").Return()
			},
			expectedCached:    []string{"service1", "service2"},
			expectedNotCached: nil,
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

				mockRegistry.On("IsGlobalRegistry", "digest1").Return(false)

				serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "digest1").Return()
			},
			expectedCached:    []string{"service1"},
			expectedNotCached: []string{"service2"},
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
			expectedNotCached: []string{"service1"},
		},
		{
			name:         "services with dependencies - all cached",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
				"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
			},
			svcsToBuild: []string{"service1", "service2"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
				cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(true, "digest2", nil)

				mockRegistry.On("IsGlobalRegistry", "digest1").Return(false)
				mockRegistry.On("IsGlobalRegistry", "digest2").Return(false)

				serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "digest1").Return()
				serviceEnvVarsSetter.On("SetServiceEnvVars", "service2", "digest2").Return()
			},
			expectedCached:    []string{"service1", "service2"},
			expectedNotCached: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, _, hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry := createTestParallelCheckStrategy()
			tt.setupMocks(hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry)

			svcInfos := buildTypes.NewBuildInfos(tt.manifestName, "test-namespace", "", tt.svcsToBuild)
			cached, notCached, err := strategy.CheckServicesCache(context.Background(), tt.manifestName, tt.buildManifest, svcInfos)

			toNames := func(bis []*buildTypes.BuildInfo) []string {
				out := make([]string, 0, len(bis))
				for _, bi := range bis {
					out = append(out, bi.Name())
				}
				return out
			}

			assert.ElementsMatch(t, tt.expectedCached, toNames(cached), "cached services")
			assert.ElementsMatch(t, tt.expectedNotCached, toNames(notCached), "not cached services")
			assert.NoError(t, err)

			hasher.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
			serviceEnvVarsSetter.AssertExpectations(t)
			mockRegistry.AssertExpectations(t)
		})
	}
}

func TestParallelCheckStrategy_CheckServicesCache_Error(t *testing.T) {
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
		// Note: The clone error test is skipped because mocking IsGlobalRegistry in parallel execution
		// has proven difficult. The cloner behavior depends on IsGlobalRegistry, and when it returns false,
		// the cloner returns the image directly without calling Clone, making it hard to test the error path.
		// This functionality is covered by integration tests and the sequential strategy tests.
		{
			name:         "clone error when image is global registry",
			manifestName: "test-manifest",
			buildManifest: build.ManifestBuild{
				"service1": &build.Info{Image: "image1"},
			},
			svcsToBuild: []string{"service1"},
			setupMocks: func(hasher *MockHasherController, cacheProbe *MockCacheProbe, serviceEnvVarsSetter *MockServiceEnvVarsSetter, mockRegistry *MockRegistryController) {
				hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")

				cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)

				// When IsGlobalRegistry returns true, the cloner tries to clone
				// Since buildInfo.Image is "image1", that's what gets passed as svcImage (not empty)
				mockRegistry.On("IsGlobalRegistry", "digest1").Return(true)
				mockRegistry.On("Clone", "digest1", "image1").Return("", fmt.Errorf("clone error"))
			},
			expectedCached:    nil,
			expectedNotCached: nil,
			expectedError:     fmt.Errorf("error cloning svc service1 global image to dev: error cloning image 'digest1' to 'image1': clone error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, _, hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry := createTestParallelCheckStrategy()
			tt.setupMocks(hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry)

			svcInfos := buildTypes.NewBuildInfos(tt.manifestName, "test-namespace", "", tt.svcsToBuild)
			cached, notCached, err := strategy.CheckServicesCache(context.Background(), tt.manifestName, tt.buildManifest, svcInfos)

			toNames := func(bis []*buildTypes.BuildInfo) []string {
				out := make([]string, 0, len(bis))
				for _, bi := range bis {
					out = append(out, bi.Name())
				}
				return out
			}

			assert.ElementsMatch(t, tt.expectedCached, toNames(cached), "cached services")
			assert.ElementsMatch(t, tt.expectedNotCached, toNames(notCached), "not cached services")
			assert.Error(t, err)
			assert.Equal(t, tt.expectedError.Error(), err.Error())

			hasher.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
			serviceEnvVarsSetter.AssertExpectations(t)
			mockRegistry.AssertExpectations(t)
		})
	}
}

func TestParallelCheckStrategy_GetImageDigestReferenceForServiceDeploy(t *testing.T) {
	tests := []struct {
		name              string
		manifestName      string
		service           string
		buildInfo         *build.Info
		setupMocks        func(*MockImageTagger, *MockCacheProbe)
		expectedReference string
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, tagger, _, cacheProbe, _, _ := createTestParallelCheckStrategy()
			tt.setupMocks(tagger, cacheProbe)

			reference, err := strategy.GetImageDigestReferenceForServiceDeploy(tt.manifestName, tt.service, tt.buildInfo)

			assert.Equal(t, tt.expectedReference, reference)
			assert.NoError(t, err)

			tagger.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
		})
	}
}

func TestParallelCheckStrategy_GetImageDigestReferenceForServiceDeploy_Error(t *testing.T) {
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
			strategy, tagger, _, cacheProbe, _, _ := createTestParallelCheckStrategy()
			tt.setupMocks(tagger, cacheProbe)

			reference, err := strategy.GetImageDigestReferenceForServiceDeploy(tt.manifestName, tt.service, tt.buildInfo)

			assert.Equal(t, tt.expectedReference, reference)
			assert.Error(t, err)
			assert.Equal(t, tt.expectedError.Error(), err.Error())

			tagger.AssertExpectations(t)
			cacheProbe.AssertExpectations(t)
		})
	}
}

// TestParallelCheckStrategy_CheckServicesCache_OrderOfExecution verifies that
// services are processed in the correct order based on their dependencies.
// A service should only start processing after all its dependencies have completed.
func TestParallelCheckStrategy_CheckServicesCache_OrderOfExecution(t *testing.T) {
	// Create a chain of dependencies: service1 -> service2 -> service3
	// service2 depends on service1, service3 depends on service2
	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{
		"service1": &build.Info{Image: "image1"},
		"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
		"service3": &build.Info{Image: "image3", DependsOn: []string{"service2"}},
	}
	svcsToBuild := []string{"service1", "service2", "service3"}

	// Use a slice protected by mutex to record execution order
	var executionOrder []string
	var mu sync.Mutex

	strategy, _, hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry := createTestParallelCheckStrategy()

	// Mock hasher to record execution order
	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Run(func(args mock.Arguments) {
		mu.Lock()
		executionOrder = append(executionOrder, "hash-service1")
		mu.Unlock()
	}).Return("hash1")

	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service2").Run(func(args mock.Arguments) {
		mu.Lock()
		executionOrder = append(executionOrder, "hash-service2")
		mu.Unlock()
	}).Return("hash2")

	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service3").Run(func(args mock.Arguments) {
		mu.Lock()
		executionOrder = append(executionOrder, "hash-service3")
		mu.Unlock()
	}).Return("hash3")

	// Mock cache checks - all services are cached
	// The second return value from IsCached is the imageWithDigest, which is passed to cloneGlobalImageToDev
	cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
	cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(true, "digest2", nil)
	cacheProbe.On("IsCached", "test-manifest", "image3", "hash3", "service3").Return(true, "digest3", nil)

	// The cloner calls IsGlobalRegistry with the cachedImage (digest) value
	mockRegistry.On("IsGlobalRegistry", "digest1").Return(false)
	mockRegistry.On("IsGlobalRegistry", "digest2").Return(false)
	mockRegistry.On("IsGlobalRegistry", "digest3").Return(false)

	// When IsGlobalRegistry returns false, the cloner returns the image directly
	// So the reference passed to SetServiceEnvVars is the digest
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "digest1").Return()
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service2", "digest2").Return()
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service3", "digest3").Return()

	svcInfos := buildTypes.NewBuildInfos(manifestName, "test-namespace", "", svcsToBuild)
	cached, notCached, err := strategy.CheckServicesCache(context.Background(), manifestName, buildManifest, svcInfos)

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, cached, 3, "all services should be cached")
	assert.Len(t, notCached, 0, "no services should be not cached")

	// Verify execution order: service1 should be processed before service2, service2 before service3
	mu.Lock()
	order := make([]string, len(executionOrder))
	copy(order, executionOrder)
	mu.Unlock()

	// Find the positions of each service in the execution order
	positions := make(map[string]int)
	for i, svc := range order {
		if _, found := positions[svc]; !found {
			positions[svc] = i
		}
	}

	// Verify that each service appears exactly once
	pos1, ok1 := positions["hash-service1"]
	pos2, ok2 := positions["hash-service2"]
	pos3, ok3 := positions["hash-service3"]

	assert.True(t, ok1, "service1 should be processed")
	assert.True(t, ok2, "service2 should be processed")
	assert.True(t, ok3, "service3 should be processed")

	// Verify the order: service1 must come before service2, service2 must come before service3
	assert.True(t, pos1 < pos2, "service1 should be processed before service2 (got positions: service1=%d, service2=%d)", pos1, pos2)
	assert.True(t, pos2 < pos3, "service2 should be processed before service3 (got positions: service2=%d, service3=%d)", pos2, pos3)

	hasher.AssertExpectations(t)
	cacheProbe.AssertExpectations(t)
	serviceEnvVarsSetter.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}

// TestParallelCheckStrategy_CheckServicesCache_OrderOfExecution_ComplexDependencies
// verifies order of execution with a more complex dependency graph.
func TestParallelCheckStrategy_CheckServicesCache_OrderOfExecution_ComplexDependencies(t *testing.T) {
	// Create a dependency graph:
	//   service1 (no deps)
	//   service2 (depends on service1)
	//   service3 (depends on service1)
	//   service4 (depends on service2 and service3)
	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{
		"service1": &build.Info{Image: "image1"},
		"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
		"service3": &build.Info{Image: "image3", DependsOn: []string{"service1"}},
		"service4": &build.Info{Image: "image4", DependsOn: []string{"service2", "service3"}},
	}
	svcsToBuild := []string{"service1", "service2", "service3", "service4"}

	// Use a slice protected by mutex to record execution order
	var executionOrder []string
	var mu sync.Mutex

	strategy, _, hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry := createTestParallelCheckStrategy()

	// Mock hasher to record execution order
	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Run(func(args mock.Arguments) {
		mu.Lock()
		executionOrder = append(executionOrder, "hash-service1")
		mu.Unlock()
	}).Return("hash1")

	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service2").Run(func(args mock.Arguments) {
		mu.Lock()
		executionOrder = append(executionOrder, "hash-service2")
		mu.Unlock()
	}).Return("hash2")

	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service3").Run(func(args mock.Arguments) {
		mu.Lock()
		executionOrder = append(executionOrder, "hash-service3")
		mu.Unlock()
	}).Return("hash3")

	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service4").Run(func(args mock.Arguments) {
		mu.Lock()
		executionOrder = append(executionOrder, "hash-service4")
		mu.Unlock()
	}).Return("hash4")

	// Mock cache checks - all services are cached
	// The second return value from IsCached is the imageWithDigest, which is passed to cloneGlobalImageToDev
	cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
	cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(true, "digest2", nil)
	cacheProbe.On("IsCached", "test-manifest", "image3", "hash3", "service3").Return(true, "digest3", nil)
	cacheProbe.On("IsCached", "test-manifest", "image4", "hash4", "service4").Return(true, "digest4", nil)

	// The cloner calls IsGlobalRegistry with the cachedImage (digest) value
	mockRegistry.On("IsGlobalRegistry", "digest1").Return(false)
	mockRegistry.On("IsGlobalRegistry", "digest2").Return(false)
	mockRegistry.On("IsGlobalRegistry", "digest3").Return(false)
	mockRegistry.On("IsGlobalRegistry", "digest4").Return(false)

	// When IsGlobalRegistry returns false, the cloner returns the image directly
	// So the reference passed to SetServiceEnvVars is the digest
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "digest1").Return()
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service2", "digest2").Return()
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service3", "digest3").Return()
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service4", "digest4").Return()

	svcInfos := buildTypes.NewBuildInfos(manifestName, "test-namespace", "", svcsToBuild)
	cached, notCached, err := strategy.CheckServicesCache(context.Background(), manifestName, buildManifest, svcInfos)

	// Verify results
	assert.NoError(t, err)
	assert.Len(t, cached, 4, "all services should be cached")
	assert.Len(t, notCached, 0, "no services should be not cached")

	// Verify execution order
	mu.Lock()
	order := make([]string, len(executionOrder))
	copy(order, executionOrder)
	mu.Unlock()

	// Find the positions of each service in the execution order
	positions := make(map[string]int)
	for i, svc := range order {
		if _, found := positions[svc]; !found {
			positions[svc] = i
		}
	}

	// Verify that each service appears exactly once
	pos1, ok1 := positions["hash-service1"]
	pos2, ok2 := positions["hash-service2"]
	pos3, ok3 := positions["hash-service3"]
	pos4, ok4 := positions["hash-service4"]

	assert.True(t, ok1, "service1 should be processed")
	assert.True(t, ok2, "service2 should be processed")
	assert.True(t, ok3, "service3 should be processed")
	assert.True(t, ok4, "service4 should be processed")

	// Verify the order constraints:
	// - service1 must come before service2 and service3
	// - service2 and service3 must come before service4
	assert.True(t, pos1 < pos2, "service1 should be processed before service2")
	assert.True(t, pos1 < pos3, "service1 should be processed before service3")
	assert.True(t, pos2 < pos4, "service2 should be processed before service4")
	assert.True(t, pos3 < pos4, "service3 should be processed before service4")

	hasher.AssertExpectations(t)
	cacheProbe.AssertExpectations(t)
	serviceEnvVarsSetter.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}

// TestParallelCheckStrategy_CheckServicesCache_OrderOfResults verifies that
// cachedSvcs and notCachedSvcs are returned in the same order as svcsToBuild,
// even when services have dependencies. This test specifically covers the change
// that iterates over svcsToBuild instead of the nodes map to preserve order.
func TestParallelCheckStrategy_CheckServicesCache_OrderOfResults(t *testing.T) {
	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{
		"service1": &build.Info{Image: "image1"},
		"service2": &build.Info{Image: "image2", DependsOn: []string{"service1"}},
		"service3": &build.Info{Image: "image3", DependsOn: []string{"service1"}},
		"service4": &build.Info{Image: "image4", DependsOn: []string{"service2", "service3"}},
		"service5": &build.Info{Image: "image5", DependsOn: []string{"service4"}},
	}
	// Define services in a specific order (already ordered by dependencies from DAG.Ordered())
	svcsToBuild := []string{"service1", "service2", "service3", "service4", "service5"}

	strategy, _, hasher, cacheProbe, serviceEnvVarsSetter, mockRegistry := createTestParallelCheckStrategy()

	// Setup mocks: service1 and service3 are cached, service2 is not cached
	// service4 depends on service2 (not cached), so it will be marked as not cached without checking
	// service5 depends on service4 (not cached), so it will be marked as not cached without checking
	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service1").Return("hash1")
	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service2").Return("hash2")
	hasher.On("hashWithBuildContext", mock.AnythingOfType("*build.Info"), "service3").Return("hash3")
	// service4 and service5 won't be checked because their dependencies are not cached

	// service1 and service3 are cached
	cacheProbe.On("IsCached", "test-manifest", "image1", "hash1", "service1").Return(true, "digest1", nil)
	cacheProbe.On("IsCached", "test-manifest", "image3", "hash3", "service3").Return(true, "digest3", nil)

	// service2 is not cached (service4 and service5 will be marked as not cached without checking)
	cacheProbe.On("IsCached", "test-manifest", "image2", "hash2", "service2").Return(false, "", nil)

	// Mock registry for cached services
	mockRegistry.On("IsGlobalRegistry", "digest1").Return(false)
	mockRegistry.On("IsGlobalRegistry", "digest3").Return(false)

	// Mock SetServiceEnvVars for cached services
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service1", "digest1").Return()
	serviceEnvVarsSetter.On("SetServiceEnvVars", "service3", "digest3").Return()

	svcInfos := buildTypes.NewBuildInfos(manifestName, "test-namespace", "", svcsToBuild)
	cached, notCached, err := strategy.CheckServicesCache(context.Background(), manifestName, buildManifest, svcInfos)

	// Verify no error
	assert.NoError(t, err)

	// Verify the counts
	assert.Len(t, cached, 2, "should have 2 cached services")
	assert.Len(t, notCached, 3, "should have 3 not cached services")

	// Convert to names for easier comparison
	toNames := func(bis []*buildTypes.BuildInfo) []string {
		out := make([]string, 0, len(bis))
		for _, bi := range bis {
			out = append(out, bi.Name())
		}
		return out
	}

	cachedNames := toNames(cached)
	notCachedNames := toNames(notCached)

	// Verify cached services are in the correct order
	// service1 comes before service3 in svcsToBuild, so cached should be [service1, service3]
	assert.Equal(t, []string{"service1", "service3"}, cachedNames, "cached services should maintain order from svcsToBuild")

	// Verify not cached services are in the correct order
	// service2, service4, service5 appear in that order in svcsToBuild
	assert.Equal(t, []string{"service2", "service4", "service5"}, notCachedNames, "not cached services should maintain order from svcsToBuild")

	hasher.AssertExpectations(t)
	cacheProbe.AssertExpectations(t)
	serviceEnvVarsSetter.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}
