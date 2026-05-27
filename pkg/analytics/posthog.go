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

// commonPostHogGroups returns the PostHog group memberships for an event.
// PRECONDITION: must only be called after analyticsEnabled() returns true.
func commonPostHogGroups() posthog.Groups {
	ctx := okteto.GetContext()
	g := posthog.NewGroups()
	if ctx.CompanyName != "" {
		g = g.Set("customer", ctx.CompanyName)
	}
	if ctx.ClusterID != "" {
		g = g.Set("cluster", ctx.ClusterID)
	}
	return g
}

// IdentifyGroups sends $groupidentify calls for the two canonical group types:
//   - "cluster"  keyed on ClusterID (stable UUID from the K8s telemetry secret)
//   - "customer" keyed on CompanyName (normalized at ingestion via Hog transformation)
//
// Safe to call immediately after context is populated — skips silently if
// analytics is disabled or both keys are empty.
func (b *posthogBackend) IdentifyGroups() {
	if b.client == nil || !analyticsEnabled() {
		return
	}
	ctx := okteto.GetContext()

	if ctx.ClusterID != "" {
		if err := b.client.Enqueue(posthog.GroupIdentify{
			Type: "cluster",
			Key:  ctx.ClusterID,
			Properties: posthog.NewProperties().
				Set("cluster_id", ctx.ClusterID),
		}); err != nil {
			oktetoLog.Infof("failed to send posthog group identify (cluster): %s", err)
		}
	}

	if ctx.CompanyName != "" {
		if err := b.client.Enqueue(posthog.GroupIdentify{
			Type: "customer",
			Key:  ctx.CompanyName,
			Properties: posthog.NewProperties().
				Set("customer_id", ctx.CompanyName),
		}); err != nil {
			oktetoLog.Infof("failed to send posthog group identify (customer): %s", err)
		}
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
		DistinctId: okteto.GetContext().UserID,
		Event:      posthogImageBuildEvent,
		Properties: props,
		Groups:     commonPostHogGroups(),
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
