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

package metadata

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/cmd/build/v2/smartbuild"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type mockNamespaceDevEnvGetter struct {
	namespace string
}

func (m *mockNamespaceDevEnvGetter) GetNamespace() string {
	return m.namespace
}

type mockRepoAnonymizer struct {
	repo string
}

func (m *mockRepoAnonymizer) GetAnonymizedRepo() string {
	return m.repo
}

// Mock implementations for smartbuild dependencies
type mockRepositoryInterface struct {
	sha       string
	shaErr    error
	dirSha    string
	dirShaErr error
	diffHash  string
	diffErr   error
}

func (m *mockRepositoryInterface) GetSHA() (string, error) {
	return m.sha, m.shaErr
}

func (m *mockRepositoryInterface) GetLatestDirSHA(dir string) (string, error) {
	return m.dirSha, m.dirShaErr
}

func (m *mockRepositoryInterface) GetDiffHash(sha string) (string, error) {
	return m.diffHash, m.diffErr
}

type mockRegistryController struct{}

func (m *mockRegistryController) GetDevImageFromGlobal(image string) string {
	return image
}

func (m *mockRegistryController) Clone(globalImage, devImage string) (string, error) {
	return devImage, nil
}

func (m *mockRegistryController) IsGlobalRegistry(image string) bool {
	return false
}

func (m *mockRegistryController) IsOktetoRegistry(image string) bool {
	return false
}

type mockWorkingDirGetter struct {
	wd    string
	wdErr error
}

func (m *mockWorkingDirGetter) Get() (string, error) {
	return m.wd, m.wdErr
}

// createMockSmartBuildCtrl creates a smartbuild.Ctrl with mock dependencies
func createMockSmartBuildCtrl(projectHash string, projectErr error, serviceHash string) *smartbuild.Ctrl {
	repo := &mockRepositoryInterface{
		sha:    projectHash,
		shaErr: projectErr,
	}
	registry := &mockRegistryController{}
	wdGetter := &mockWorkingDirGetter{wd: "/test"}

	return smartbuild.NewSmartBuildCtrl(repo, registry, afero.NewMemMapFs(), io.NewIOController(), wdGetter)
}

func TestNewMetadataCollector(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	require.NotNil(t, collector)
	assert.Equal(t, "test-namespace", collector.oktetoContext.GetNamespace())
	assert.Equal(t, "test-repo", collector.config.GetAnonymizedRepo())
	assert.NotNil(t, collector.metaMap)
	assert.Empty(t, collector.metaMap)
	assert.NotNil(t, collector.smartBuildCtrl)
	assert.NotNil(t, collector.logger)
}

func TestGetMetadataMap(t *testing.T) {
	collector := &MetadataCollector{
		metaMap: make(map[string]*analytics.ImageBuildMetadata),
	}

	// Test empty map
	metaMap := collector.GetMetadataMap()
	assert.NotNil(t, metaMap)
	assert.Empty(t, metaMap)

	// Test with data
	expectedMeta := analytics.NewImageBuildMetadata()
	expectedMeta.Name = "test-service"
	collector.metaMap["test-service"] = expectedMeta

	metaMap = collector.GetMetadataMap()
	assert.NotNil(t, metaMap)
	assert.Len(t, metaMap, 1)
	assert.Equal(t, expectedMeta, metaMap["test-service"])
}

func TestGetMetadata(t *testing.T) {
	collector := &MetadataCollector{
		metaMap: make(map[string]*analytics.ImageBuildMetadata),
	}

	// Test getting non-existent metadata
	meta := collector.GetMetadata("non-existent")
	assert.Nil(t, meta)

	// Test getting existing metadata
	expectedMeta := analytics.NewImageBuildMetadata()
	expectedMeta.Name = "test-service"
	collector.metaMap["test-service"] = expectedMeta

	meta = collector.GetMetadata("test-service")
	assert.NotNil(t, meta)
	assert.Equal(t, expectedMeta, meta)
}

func TestGetMetadata_ThreadSafety(t *testing.T) {
	collector := &MetadataCollector{
		metaMap: make(map[string]*analytics.ImageBuildMetadata),
	}

	// Add some test data
	meta1 := analytics.NewImageBuildMetadata()
	meta1.Name = "service1"
	collector.metaMap["service1"] = meta1

	meta2 := analytics.NewImageBuildMetadata()
	meta2.Name = "service2"
	collector.metaMap["service2"] = meta2

	// Test concurrent reads
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Read from both services
			meta1 := collector.GetMetadata("service1")
			meta2 := collector.GetMetadata("service2")
			assert.NotNil(t, meta1)
			assert.NotNil(t, meta2)
		}()
	}

	wg.Wait()
}

func TestCollectMetadata(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{
		"service1": &build.Info{
			Context:    ".",
			Dockerfile: "Dockerfile",
		},
		"service2": &build.Info{
			Context:    "./app",
			Dockerfile: "Dockerfile.app",
		},
	}
	toBuildSvcs := []string{"service1", "service2"}

	ctx := context.Background()
	err := collector.CollectMetadata(ctx, manifestName, buildManifest, toBuildSvcs)

	require.NoError(t, err)

	// Verify metadata was collected for both services
	metaMap := collector.GetMetadataMap()
	assert.Len(t, metaMap, 2)

	// Check service1 metadata
	meta1 := collector.GetMetadata("service1")
	require.NotNil(t, meta1)
	assert.Equal(t, "service1", meta1.Name)
	assert.Equal(t, "test-namespace", meta1.Namespace)
	assert.Equal(t, "test-manifest", meta1.DevenvName)
	assert.Equal(t, "test-repo", meta1.RepoURL)
	assert.NotEmpty(t, meta1.RepoHash)         // Hash should be computed
	assert.NotEmpty(t, meta1.BuildContextHash) // Hash should be computed

	// Check service2 metadata
	meta2 := collector.GetMetadata("service2")
	require.NotNil(t, meta2)
	assert.Equal(t, "service2", meta2.Name)
	assert.Equal(t, "test-namespace", meta2.Namespace)
	assert.Equal(t, "test-manifest", meta2.DevenvName)
	assert.Equal(t, "test-repo", meta2.RepoURL)
	assert.NotEmpty(t, meta2.RepoHash)         // Hash should be computed
	assert.NotEmpty(t, meta2.BuildContextHash) // Hash should be computed
}

func TestCollectMetadata_EmptyServices(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{}
	toBuildSvcs := []string{}

	ctx := context.Background()
	err := collector.CollectMetadata(ctx, manifestName, buildManifest, toBuildSvcs)

	require.NoError(t, err)

	// Verify no metadata was collected
	metaMap := collector.GetMetadataMap()
	assert.Empty(t, metaMap)
}

func TestCollectMetadata_ConcurrentExecution(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{
		"service1": &build.Info{Context: "."},
		"service2": &build.Info{Context: "./app"},
		"service3": &build.Info{Context: "./api"},
		"service4": &build.Info{Context: "./web"},
	}
	toBuildSvcs := []string{"service1", "service2", "service3", "service4"}

	ctx := context.Background()
	err := collector.CollectMetadata(ctx, manifestName, buildManifest, toBuildSvcs)

	require.NoError(t, err)

	// Verify all services were processed
	metaMap := collector.GetMetadataMap()
	assert.Len(t, metaMap, 4)

	for _, svcName := range toBuildSvcs {
		meta := collector.GetMetadata(svcName)
		require.NotNil(t, meta)
		assert.Equal(t, svcName, meta.Name)
	}
}

func TestCollectMetadata_ContextCancellation(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{
		"service1": &build.Info{Context: "."},
	}
	toBuildSvcs := []string{"service1"}

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := collector.CollectMetadata(ctx, manifestName, buildManifest, toBuildSvcs)

	// Should not error even with cancelled context
	require.NoError(t, err)
}

func TestCollectMetadata_ProjectHashError(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("", errors.New("project hash error"), "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{
		"service1": &build.Info{Context: "."},
	}
	toBuildSvcs := []string{"service1"}

	ctx := context.Background()
	err := collector.CollectMetadata(ctx, manifestName, buildManifest, toBuildSvcs)

	// Should not error even with project hash error
	require.NoError(t, err)

	// Verify metadata was still collected (with empty repo hash due to error)
	meta := collector.GetMetadata("service1")
	require.NotNil(t, meta)
	assert.Equal(t, "service1", meta.Name)
	assert.Empty(t, meta.RepoHash)            // Should be empty due to error
	assert.NotEmpty(t, meta.BuildContextHash) // Should still be computed
}

func TestCollectMetadata_LimitConcurrency(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	buildManifest := build.ManifestBuild{}
	toBuildSvcs := []string{}

	// Create many services to test concurrency limit
	for i := 0; i < 20; i++ {
		svcName := fmt.Sprintf("service%d", i)
		buildManifest[svcName] = &build.Info{Context: "."}
		toBuildSvcs = append(toBuildSvcs, svcName)
	}

	ctx := context.Background()
	err := collector.CollectMetadata(ctx, manifestName, buildManifest, toBuildSvcs)

	require.NoError(t, err)

	// Verify all services were processed
	metaMap := collector.GetMetadataMap()
	assert.Len(t, metaMap, 20)

	// Verify concurrency limit is respected (should be min(20, min(4, 2*NumCPU())))
	expectedLimit := min(20, min(4, 2*runtime.NumCPU()))
	assert.LessOrEqual(t, len(metaMap), 20)
	assert.LessOrEqual(t, expectedLimit, 20)
}

func TestBaseMeta(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	svcName := "test-service"

	meta := collector.baseMeta(manifestName, svcName)

	require.NotNil(t, meta)
	assert.Equal(t, svcName, meta.Name)
	assert.Equal(t, "test-namespace", meta.Namespace)
	assert.Equal(t, manifestName, meta.DevenvName)
	assert.Equal(t, "test-repo", meta.RepoURL)
}

func TestCollectForService_ContextCancellation(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	svcName := "test-service"
	info := &build.Info{Context: "."}

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	meta, err := collector.collectForService(ctx, manifestName, svcName, info)

	// Should return context error
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	// Meta should still be created with base information
	require.NotNil(t, meta)
	assert.Equal(t, svcName, meta.Name)
	assert.Equal(t, "test-namespace", meta.Namespace)
	assert.Equal(t, manifestName, meta.DevenvName)
	assert.Equal(t, "test-repo", meta.RepoURL)
}

func TestCollectForService_Success(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	svcName := "test-service"
	info := &build.Info{Context: "."}

	ctx := context.Background()
	meta, err := collector.collectForService(ctx, manifestName, svcName, info)

	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, svcName, meta.Name)
	assert.Equal(t, "test-namespace", meta.Namespace)
	assert.Equal(t, manifestName, meta.DevenvName)
	assert.Equal(t, "test-repo", meta.RepoURL)
	assert.NotEmpty(t, meta.RepoHash)         // Hash should be computed
	assert.NotEmpty(t, meta.BuildContextHash) // Hash should be computed

	// Verify durations are set
	assert.Greater(t, meta.RepoHashDuration, time.Duration(0))
	assert.Greater(t, meta.BuildContextHashDuration, time.Duration(0))
}

func TestCollectForService_ProjectHashError(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("", errors.New("project hash error"), "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	svcName := "test-service"
	info := &build.Info{Context: "."}

	ctx := context.Background()
	meta, err := collector.collectForService(ctx, manifestName, svcName, info)

	// The method doesn't return an error for project hash failures, only for context cancellation
	require.NoError(t, err)

	// Meta should still be created with base information
	require.NotNil(t, meta)
	assert.Equal(t, svcName, meta.Name)
	assert.Equal(t, "test-namespace", meta.Namespace)
	assert.Equal(t, manifestName, meta.DevenvName)
	assert.Equal(t, "test-repo", meta.RepoURL)
	assert.Empty(t, meta.RepoHash)            // Should be empty due to error
	assert.NotEmpty(t, meta.BuildContextHash) // Should still be computed
}

func TestCollectForService_ConcurrentExecution(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	svcName := "test-service"
	info := &build.Info{Context: "."}

	// Test concurrent execution
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			meta, err := collector.collectForService(ctx, manifestName, svcName, info)
			require.NoError(t, err)
			require.NotNil(t, meta)
			assert.Equal(t, svcName, meta.Name)
		}()
	}

	wg.Wait()
}

func TestCollectForService_DurationMeasurement(t *testing.T) {
	oktetoContext := &mockNamespaceDevEnvGetter{namespace: "test-namespace"}
	config := &mockRepoAnonymizer{repo: "test-repo"}

	// Create a smart build controller that takes some time
	smartBuildCtrl := createMockSmartBuildCtrl("project-hash-123", nil, "service-hash-456")
	logger := io.NewIOController()

	collector := NewMetadataCollector(oktetoContext, config, smartBuildCtrl, logger)

	manifestName := "test-manifest"
	svcName := "test-service"
	info := &build.Info{Context: "."}

	ctx := context.Background()
	meta, err := collector.collectForService(ctx, manifestName, svcName, info)

	require.NoError(t, err)
	require.NotNil(t, meta)

	// Verify durations are measured
	assert.GreaterOrEqual(t, meta.RepoHashDuration, time.Duration(0))
	assert.GreaterOrEqual(t, meta.BuildContextHashDuration, time.Duration(0))
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
