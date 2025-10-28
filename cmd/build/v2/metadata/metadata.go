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

package metadata

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/okteto/okteto/cmd/build/v2/smartbuild"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"golang.org/x/sync/errgroup"
)

type namespaceDevEnvGetter interface {
	GetNamespace() string
}

type repoAnonymizer interface {
	GetAnonymizedRepo() string
}

type MetadataCollector struct {
	metaMu  sync.RWMutex                             // mutex to protect the metaMap
	metaMap map[string]*analytics.ImageBuildMetadata // map[svcName]meta

	oktetoContext  namespaceDevEnvGetter
	config         repoAnonymizer
	smartBuildCtrl *smartbuild.Ctrl
	logger         *io.Controller
}

func NewMetadataCollector(oktetoContext namespaceDevEnvGetter, config repoAnonymizer, smartBuildCtrl *smartbuild.Ctrl, logger *io.Controller) *MetadataCollector {
	return &MetadataCollector{
		metaMap:        make(map[string]*analytics.ImageBuildMetadata),
		oktetoContext:  oktetoContext,
		config:         config,
		smartBuildCtrl: smartBuildCtrl,
		logger:         logger,
	}
}

func (m *MetadataCollector) GetMetadataMap() map[string]*analytics.ImageBuildMetadata {
	m.metaMu.RLock()
	defer m.metaMu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*analytics.ImageBuildMetadata)
	for k, v := range m.metaMap {
		result[k] = v
	}
	return result
}

// GetMetadata returns the metadata for the given service name
func (m *MetadataCollector) GetMetadata(svcName string) *analytics.ImageBuildMetadata {
	m.metaMu.RLock()
	defer m.metaMu.RUnlock()
	return m.metaMap[svcName]
}

func (m *MetadataCollector) CollectMetadata(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, toBuildSvcs []string) error {
	g, ctx := errgroup.WithContext(ctx)
	limit := min(len(toBuildSvcs), min(4, 2*runtime.NumCPU()))
	g.SetLimit(limit)

	for _, name := range toBuildSvcs {
		svcName := name
		info := buildManifest[svcName]

		g.Go(func() error {
			meta, _ := m.collectForService(ctx, manifestName, svcName, info)

			m.metaMu.Lock()
			m.metaMap[svcName] = meta
			m.metaMu.Unlock()
			return nil
		})
	}
	return g.Wait()
}

// collectForService collects the metadata for the given service name
func (m *MetadataCollector) collectForService(ctx context.Context, manifestName, svcName string, info *build.Info) (*analytics.ImageBuildMetadata, error) {
	meta := m.baseMeta(manifestName, svcName)

	var wg sync.WaitGroup
	wg.Add(2)

	// Repo hash
	var repoHash string
	var repoHashDuration time.Duration
	var repoErr error
	go func() {
		defer wg.Done()
		start := time.Now()
		select {
		case <-ctx.Done():
			repoErr = ctx.Err()
			repoHashDuration = time.Since(start)
			return
		default:
		}
		m.logger.Logger().Debugf("getting project hash for analytics (%s)", svcName)
		h, err := m.smartBuildCtrl.GetProjectHash(info)
		repoHashDuration = time.Since(start)
		if err != nil {
			m.logger.Logger().Infof("error getting project commit hash for %s: %v", svcName, err)
		} else {
			repoHash = h
		}
	}()

	// Context hash
	var ctxHash string
	var ctxHashDuration time.Duration
	go func() {
		defer wg.Done()
		start := time.Now()
		select {
		case <-ctx.Done():
			ctxHashDuration = time.Since(start)
			return
		default:
		}
		h := m.smartBuildCtrl.GetBuildHash(info, svcName)
		ctxHash = h
		ctxHashDuration = time.Since(start)
	}()

	wg.Wait()

	meta.RepoHash = repoHash
	meta.RepoHashDuration = repoHashDuration
	meta.BuildContextHash = ctxHash
	meta.BuildContextHashDuration = ctxHashDuration

	return meta, repoErr
}

func (m *MetadataCollector) baseMeta(manifestName, svcName string) *analytics.ImageBuildMetadata {
	meta := analytics.NewImageBuildMetadata()
	meta.Name = svcName
	meta.Namespace = m.oktetoContext.GetNamespace()
	meta.DevenvName = manifestName
	meta.RepoURL = m.config.GetAnonymizedRepo()
	return meta
}
