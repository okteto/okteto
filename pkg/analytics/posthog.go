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
	"time"

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
	client, _ := posthog.NewWithConfig(posthogToken, posthog.Config{
		Endpoint:        posthogEndpoint,
		ShutdownTimeout: 500 * time.Millisecond,
	})
	return &posthogBackend{client: client}
}

// commonPostHogProperties returns properties sent on every PostHog event from the CLI.
// PRECONDITION: must only be called after analyticsEnabled() returns true.
// Calling it with an uninitialized okteto context will call GetContext() which Fatalf's.
//
// Note: user_id is included as an explicit property (per product spec) in addition to
// being set as DistinctId on posthog.Capture.
func commonPostHogProperties() posthog.Properties {
	agent := getAgent()
	return posthog.Properties{
		// Common (all PostHog sources)
		"customer_name": okteto.GetContext().CompanyName,
		"cluster_id":    okteto.GetContext().ClusterID,
		"user_id":       okteto.GetContext().UserID,

		// CLI common
		"cli_version":        config.VersionString,
		"os":                 runtime.GOOS,
		"arch":               runtime.GOARCH,
		"machine_id":         get().MachineID,
		"measurement_source": "cli",
		"trigger_source":     getTriggerSource(),
		"is_agent":           agent != "",
		"agent_type":         agent,
	}
}

func getTriggerSource() string {
	if env.LoadBoolean("GITHUB_ACTIONS") {
		return "github_actions"
	}
	if env.LoadBoolean("GITLAB_CI") {
		return "gitlab_ci"
	}
	if env.LoadBoolean("CIRCLECI") {
		return "circleci"
	}
	if os.Getenv("JENKINS_URL") != "" {
		return "jenkins"
	}
	return "cli"
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

// IdentifyGroups sends $groupidentify calls for the customer and cluster groups.
// Safe to call immediately after context is populated — skips silently if
// analytics is disabled or either key is empty.
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
				Set("cluster_id", ctx.ClusterID).
				Set("cluster_version", ctx.ClusterVersion),
		}); err != nil {
			oktetoLog.Infof("failed to send posthog group identify (cluster): %s", err)
		}
	}
	if ctx.CompanyName != "" {
		if err := b.client.Enqueue(posthog.GroupIdentify{
			Type: "customer",
			Key:  ctx.CompanyName,
			Properties: posthog.NewProperties().
				Set("customer_name", ctx.CompanyName),
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
