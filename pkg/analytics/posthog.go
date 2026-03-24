// Copyright 2026 The Okteto Authors
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

package analytics

import (
	"context"
	"maps"
	"runtime"

	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	posthog "github.com/posthog/posthog-go"
)

const (
	// skipcq GSC-G101
	posthogToken = "phc_hABE2tLx6OC3RdADvVeFYfwtQQhYSE5swoqkQQscU6K"

	posthogImageBuildEvent = "image_build"
)

// posthogEnqueuer is a narrow interface over posthog.Client that only exposes
// the methods we use. This keeps the mock simple in tests.
type posthogEnqueuer interface {
	Enqueue(posthog.Message) error
	Close() error
}

// posthogBackend sends analytics events to PostHog.
// posthog.New() initializes an HTTP client and background flush goroutine at
// construction time, consistent with how mixpanelClient is initialized via init().
type posthogBackend struct {
	client posthogEnqueuer
}

// newPostHogBackend creates the PostHog backend.
// Returns a no-op backend (nil client) when the token is empty — safe for tests.
func newPostHogBackend() *posthogBackend {
	if posthogToken == "" {
		return &posthogBackend{}
	}
	return &posthogBackend{client: posthog.New(posthogToken)}
}

// commonPostHogProperties returns properties sent on every PostHog event from the CLI.
// PRECONDITION: must only be called after analyticsEnabled() returns true.
// Calling it with an uninitialized okteto context will call GetContext() which Fatalf's.
//
// Note: user_id is included as an explicit property (per product spec) in addition to
// being set as DistinctId on posthog.Capture. Both use getTrackID() so they are identical.
func commonPostHogProperties() posthog.Properties {
	return posthog.Properties{
		// Common (all PostHog sources)
		"customer_name": okteto.GetContext().CompanyName,
		"cluster_id":    okteto.GetContext().Name,
		"user_id":       getTrackID(),
		// TBD: "customer_id", "repository_hash"
		// CLI common
		"cli_version": config.VersionString,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"machine_id":  get().MachineID,
	}
}

// TrackImageBuild sends an image_build event to PostHog.
func (b *posthogBackend) TrackImageBuild(_ context.Context, m *ImageBuildMetadata) {
	if b.client == nil {
		return
	}
	if !analyticsEnabled() {
		return
	}

	props := commonPostHogProperties()
	maps.Copy(props, m.toPostHogProps())

	if err := b.client.Enqueue(posthog.Capture{
		DistinctId: getTrackID(),
		Event:      posthogImageBuildEvent,
		Properties: props,
	}); err != nil {
		oktetoLog.Infof("failed to send posthog analytics: %s", err)
	}
}
