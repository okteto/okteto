// Copyright 2024 The Okteto Authors
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

package test

import (
	"context"

	"github.com/okteto/okteto/pkg/analytics"
)

// ProxyTracker is a dedicated wrapper for okteto test that intercepts deploy
// operation to edit metadata prior to be sent to out analytics backend
type ProxyTracker struct {
	*analytics.Tracker
}

func (a *ProxyTracker) TrackDeploy(metadata analytics.DeployMetadata) {
	metadata.DeployType = "test"
	a.Tracker.TrackDeploy(metadata)
}

func (a *ProxyTracker) TrackImageBuild(ctx context.Context, m *analytics.ImageBuildMetadata) {
	m.Initiator = "test"
	a.Tracker.TrackImageBuild(ctx, m)
}
