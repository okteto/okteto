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

package types

import (
	"time"

	"github.com/okteto/okteto/pkg/analytics"
)

type BuildInfo struct {
	name       string
	devenvName string
	buildHash  string
	metadata   *analytics.ImageBuildMetadata
}

func NewBuildInfos(devenvName, namespace, repoURL string, svcNames []string) []*BuildInfo {
	buildInfos := make([]*BuildInfo, 0, len(svcNames))
	for _, svcName := range svcNames {
		buildInfos = append(buildInfos, newBuildInfo(devenvName, namespace, repoURL, svcName))
	}
	return buildInfos
}

func newBuildInfo(devenvName, namespace, repoURL, svcName string) *BuildInfo {
	metadata := analytics.NewImageBuildMetadata()
	metadata.Name = svcName
	metadata.Namespace = namespace
	metadata.RepoURL = repoURL
	metadata.DevenvName = devenvName
	metadata.Initiator = "build"
	return &BuildInfo{
		name:       svcName,
		devenvName: devenvName,
		metadata:   metadata,
	}
}

func (b *BuildInfo) Name() string {
	return b.name
}

func (b *BuildInfo) Metadata() *analytics.ImageBuildMetadata {
	return b.metadata
}

// String returns the service name for pretty printing in logs
func (b *BuildInfo) String() string {
	return b.name
}

func (b *BuildInfo) SetBuildHash(buildHash string, duration time.Duration) {
	b.buildHash = buildHash

	b.metadata.BuildContextHashDuration = duration
	b.metadata.BuildContextHash = buildHash
}

func (b *BuildInfo) SetCacheHit(isCached bool, duration time.Duration) {
	b.metadata.CacheHit = isCached
	b.metadata.CacheHitDuration = duration
}

func (b *BuildInfo) SetCloneDuration(duration time.Duration, success bool) {
	b.metadata.CloneDuration = duration
	b.metadata.Success = success
}

func (b *BuildInfo) SetBuildDuration(buildDuration, waitForBuildkitAvailable time.Duration, success bool) {
	b.metadata.BuildDuration = buildDuration
	b.metadata.WaitForBuildkitAvailable = waitForBuildkitAvailable
	b.metadata.Success = success
}

func (b *BuildInfo) GetBuildHash() string {
	return b.buildHash
}
