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
	"os"
	"runtime"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	posthog "github.com/posthog/posthog-go"
)

const (
	// skipcq GSC-G101
	posthogToken = "phc_hABE2tLx6OC3RdADvVeFYfwtQQhYSE5swoqkQQscU6K"

	// posthogEndpoint is the Okteto-owned reverse proxy for PostHog.
	posthogEndpoint = "https://ph.okteto.com"

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
	client, err := posthog.NewWithConfig(posthogToken, posthog.Config{
		Endpoint: posthogEndpoint,
	})
	if err != nil {
		oktetoLog.Infof("failed to create posthog client: %s", err)
		return &posthogBackend{}
	}
	return &posthogBackend{client: client}
}

// commonPostHogProperties returns properties sent on every PostHog event from the CLI.
// Must only be called after analyticsEnabled() returns true — GetContext() Fatalf's on
// an uninitialized context.
//
// user_id is included as an explicit property (per product spec) in addition to being
// set as DistinctId on posthog.Capture. agent_type is omitted when is_agent is false.
func commonPostHogProperties() posthog.Properties {
	ctx := okteto.GetContext()
	agent := getAgent()
	props := posthog.Properties{
		// Common (all PostHog sources)
		"customer_name":   ctx.CompanyName,
		"cluster_id":      ctx.ClusterID,
		"cluster_version": ctx.ClusterVersion,
		"user_id":         ctx.UserID,

		// CLI common
		"cli_version":        config.VersionString,
		"os":                 runtime.GOOS,
		"arch":               runtime.GOARCH,
		"machine_id":         get().MachineID,
		"measurement_source": "cli",
		"is_agent":           agent != "",
	}
	if agent != "" {
		props["agent_type"] = agent
	}
	return props
}

func getAgent() string {
	if env.LoadBoolean("CODEX_CI") {
		return "codex"
	}
	if env.LoadBoolean("GEMINI_CLI") {
		return "gemini"
	}
	if env.LoadBoolean("CLAUDECODE") {
		return "claude"
	}
	if os.Getenv("CURSOR_SANDBOX") != "" {
		return "cursor"
	}
	return ""
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
		DistinctId: okteto.GetContext().UserID,
		Event:      posthogImageBuildEvent,
		Properties: props,
	}); err != nil {
		oktetoLog.Infof("failed to send posthog analytics: %s", err)
	}
}

// Close flushes pending events and shuts down the PostHog client.
func (b *posthogBackend) Close() {
	if b.client == nil {
		return
	}
	if err := b.client.Close(); err != nil {
		oktetoLog.Infof("failed to close posthog client: %s", err)
	}
}
